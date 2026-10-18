//go:build darwin && (amd64 || arm64)

package coreaudio

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"m31labs.dev/tymbal/internal/driver"
	"m31labs.dev/tymbal/internal/ring"
)

var errMicrophoneAccess = fmt.Errorf("%w: allow microphone access for this app in System Settings > Privacy & Security > Microphone", driver.ErrPermission)

type authorizationAPI struct {
	once                                                                                sync.Once
	err                                                                                 error
	framework, class, media, status, request, poolClass, poolNew, poolDrain, blockClass uintptr
}

var microphoneAPI authorizationAPI

// Authorization is a control-path feature. Load Apple's AVFoundation lazily;
// the real-time Core Audio bindings remain load-time imports with no loader
// or Objective-C work in a render callback.
func (a *authorizationAPI) initialize() {
	a.once.Do(func() {
		withNativeThread(func() {
			a.framework = authLoadFramework()
			if a.framework == 0 {
				a.err = fmt.Errorf("%w: AVFoundation authorization unavailable", driver.ErrUnsupported)
				return
			}
			a.class, a.poolClass = authClass("AVCaptureDevice"), authClass("NSAutoreleasePool")
			a.status, a.request = authSelector("authorizationStatusForMediaType:"), authSelector("requestAccessForMediaType:completionHandler:")
			a.poolNew, a.poolDrain = authSelector("new"), authSelector("drain")
			a.media, a.blockClass = authMediaType(), authGlobalBlockClass()
			if a.class == 0 || a.poolClass == 0 || a.media == 0 || a.blockClass == 0 {
				a.err = fmt.Errorf("%w: native input authorization unavailable", driver.ErrUnsupported)
			}
		})
	})
}

// Cocoa work stays on one OS thread with an autorelease pool. Native workers
// created by a framework inherit blocked process signals; Go handles those on
// its own threads. Synchronous faults remain unblocked for diagnostics.
func withNativeThread(fn func()) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	set := uint32(blockedSignals)
	var previous uint32
	if pthreadSigmask(1, &set, &previous) == 0 {
		defer pthreadSigmask(3, &previous, nil)
	}
	fn()
}

func (a *authorizationAPI) pooled(fn func()) error {
	a.initialize()
	if a.err != nil {
		return a.err
	}
	withNativeThread(func() {
		pool, _, _ := syscall_syscall(fnAuthobjc_msgSend, a.poolClass, a.poolNew, 0)
		defer syscall_syscall(fnAuthobjc_msgSend, pool, a.poolDrain, 0)
		fn()
	})
	return nil
}

func nativeInputAccess() (driver.InputAccess, error) {
	var status uintptr
	err := microphoneAPI.pooled(func() {
		status, _, _ = syscall_syscall(fnAuthobjc_msgSend, microphoneAPI.class, microphoneAPI.status, microphoneAPI.media)
	})
	if err != nil {
		return driver.InputAccessUnavailable, err
	}
	switch status {
	case 0:
		return driver.InputAccessPrompt, nil
	case 1:
		return driver.InputAccessRestricted, nil
	case 2:
		return driver.InputAccessDenied, nil
	case 3:
		return driver.InputAccessGranted, nil
	default:
		return driver.InputAccessUnavailable, fmt.Errorf("coreaudio: unknown input authorization state %d", status)
	}
}

type permissionResult struct {
	state   uint64
	sigmask uintptr
	sigset  uint64
}
type permissionBlock struct {
	isa                        uintptr
	flags, reserved            uint32
	invoke, descriptor, result uintptr
}
type permissionDescriptor struct{ reserved, size, signature uintptr }
type permissionSignal struct {
	arena  *ring.Arena
	block  *permissionBlock
	result *permissionResult
}

func permissionCompleted()

var permissionCompletedABI uintptr

func newPermissionSignal(class uintptr) (*permissionSignal, error) {
	arena, err := ring.NewArena(512)
	if err != nil {
		return nil, err
	}
	blockData := arena.Take(int(unsafe.Sizeof(permissionBlock{})), 64)
	descriptorData := arena.Take(int(unsafe.Sizeof(permissionDescriptor{})), 64)
	resultData := arena.Take(int(unsafe.Sizeof(permissionResult{})), 64)
	signature := arena.Take(16, 8)
	copy(signature, "v12@?0B8\x00")
	if runtime.GOARCH == "amd64" {
		copy(signature, "v12@?0c8\x00")
	}
	block := (*permissionBlock)(unsafe.Pointer(&blockData[0]))
	descriptor := (*permissionDescriptor)(unsafe.Pointer(&descriptorData[0]))
	result := (*permissionResult)(unsafe.Pointer(&resultData[0]))
	*descriptor = permissionDescriptor{size: unsafe.Sizeof(permissionBlock{}), signature: uintptr(unsafe.Pointer(&signature[0]))}
	*block = permissionBlock{isa: class, flags: 1<<28 | 1<<30, invoke: permissionCompletedABI, descriptor: uintptr(unsafe.Pointer(descriptor)), result: uintptr(unsafe.Pointer(result))}
	result.sigmask, result.sigset = fnpthread_sigmask, uint64(blockedSignals)
	return &permissionSignal{arena: arena, block: block, result: result}, nil
}

type permissionController struct {
	once     sync.Once
	query    func() (driver.InputAccess, error)
	allocate func() (*permissionSignal, error)
	begin    func(*permissionSignal) error
	signal   *permissionSignal
	err      error
}

func (c *permissionController) request(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	access, err := c.query()
	if err != nil {
		return err
	}
	if access == driver.InputAccessGranted {
		return nil
	}
	if access != driver.InputAccessPrompt {
		return errMicrophoneAccess
	}
	c.once.Do(func() {
		c.signal, c.err = c.allocate()
		if c.err == nil {
			c.err = c.begin(c.signal)
		}
	})
	if c.err != nil {
		return c.err
	}
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		switch atomic.LoadUint64(&c.signal.result.state) {
		case 1:
			return errMicrophoneAccess
		case 2:
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

var microphonePermission = permissionController{
	query: nativeInputAccess,
	allocate: func() (*permissionSignal, error) {
		microphoneAPI.initialize()
		if microphoneAPI.err != nil {
			return nil, microphoneAPI.err
		}
		return newPermissionSignal(microphoneAPI.blockClass)
	},
	begin: func(signal *permissionSignal) error {
		// One global block and its mmap live for the process lifetime. A canceled
		// caller must not unmap memory that AVFoundation may invoke later.
		return microphoneAPI.pooled(func() {
			_, _, _ = syscall_syscall6(fnAuthobjc_msgSend, microphoneAPI.class, microphoneAPI.request, microphoneAPI.media, uintptr(unsafe.Pointer(signal.block)), 0, 0)
		})
	},
}

func (*backend) InputAccess() (driver.InputAccess, error) { return nativeInputAccess() }
func (*backend) RequestInputAccess(ctx context.Context) error {
	return microphonePermission.request(ctx)
}
