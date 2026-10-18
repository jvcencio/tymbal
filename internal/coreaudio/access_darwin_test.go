//go:build darwin && (amd64 || arm64)

package coreaudio

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"m31labs.dev/tymbal/internal/driver"
)

func completePermission(signal *permissionSignal, granted uintptr) {
	_, _, _ = syscall_syscall(permissionCompletedABI, uintptr(unsafe.Pointer(signal.block)), granted, 0)
}

func TestInputAccessQueryDoesNotRequestOrOpenInput(t *testing.T) {
	access, err := nativeInputAccess()
	if err != nil {
		t.Fatal(err)
	}
	switch access {
	case driver.InputAccessGranted, driver.InputAccessPrompt, driver.InputAccessDenied, driver.InputAccessRestricted:
	default:
		t.Fatalf("unknown authorization state: %q", access)
	}
	if microphonePermission.signal != nil {
		t.Fatal("read-only query requested input access")
	}
}

func TestPermissionCompletionUsesNativeBlockABI(t *testing.T) {
	signal, err := newPermissionSignal(0)
	if err != nil {
		t.Fatal(err)
	}
	defer signal.arena.Close()
	signal.result.sigmask = 0 // This stand-in runs on a Go-owned test thread.
	if unsafe.Offsetof(signal.block.result) != 32 || unsafe.Sizeof(permissionBlock{}) != 40 {
		t.Fatal("native block layout changed")
	}
	for _, test := range []struct{ granted, want uintptr }{{0, 1}, {1, 2}, {255, 2}, {256, 1}} {
		completePermission(signal, test.granted)
		if got := atomic.LoadUint64(&signal.result.state); got != uint64(test.want) {
			t.Fatalf("native result %d became %d, want %d", test.granted, got, test.want)
		}
	}
}

func TestPermissionBlockIsAcceptedByObjectiveCAndBlockRuntime(t *testing.T) {
	microphoneAPI.initialize()
	if microphoneAPI.err != nil {
		t.Fatal(microphoneAPI.err)
	}
	signal, err := newPermissionSignal(microphoneAPI.blockClass)
	if err != nil {
		t.Fatal(err)
	}
	defer signal.arena.Close()
	signal.result.sigmask = 0
	lookup := func(name string) uintptr {
		c := append([]byte(name), 0)
		value, _, _ := syscall_syscall(fnAuthdlsym, ^uintptr(1), uintptr(unsafe.Pointer(&c[0])), 0)
		if value == 0 {
			t.Fatalf("missing native symbol %s", name)
		}
		return value
	}
	copyFn, releaseFn := lookup("_Block_copy"), lookup("_Block_release")
	copied, _, _ := syscall_syscall(copyFn, uintptr(unsafe.Pointer(signal.block)), 0, 0)
	if copied != uintptr(unsafe.Pointer(signal.block)) {
		t.Fatal("process-lifetime block was not treated as global")
	}
	name := []byte("class\x00")
	selector, _, _ := syscall_syscall(fnAuthsel_registerName, uintptr(unsafe.Pointer(&name[0])), 0, 0)
	err = microphoneAPI.pooled(func() {
		class, _, _ := syscall_syscall(fnAuthobjc_msgSend, copied, selector, 0)
		if class == 0 {
			t.Fatal("native block is not an Objective-C block object")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, _ = syscall_syscall(releaseFn, copied, 0, 0)
	completePermission(signal, 1)
	if atomic.LoadUint64(&signal.result.state) != 2 {
		t.Fatal("copied native block lost its result storage")
	}
}

func TestPermissionCancelKeepsLateNativeCompletionAlive(t *testing.T) {
	ready := make(chan *permissionSignal, 1)
	controller := permissionController{
		query: func() (driver.InputAccess, error) { return driver.InputAccessPrompt, nil },
		allocate: func() (*permissionSignal, error) {
			signal, err := newPermissionSignal(0)
			if err == nil {
				signal.result.sigmask = 0
			}
			return signal, err
		},
		begin: func(signal *permissionSignal) error { ready <- signal; return nil },
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- controller.request(ctx) }()
	var signal *permissionSignal
	select {
	case signal = <-ready:
	case <-time.After(time.Second):
		t.Fatal("request did not start")
	}
	defer signal.arena.Close()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled request remained blocked")
	}
	// AVFoundation may complete after the HTTP request or initiating panel
	// disappears. The block still owns valid C memory and does not enter Go.
	completePermission(signal, 1)
	if err := controller.request(context.Background()); err != nil {
		t.Fatalf("late granted access was lost: %v", err)
	}
}

func TestDeniedOrCanceledInputAccessNeverStartsPrompt(t *testing.T) {
	for _, access := range []driver.InputAccess{driver.InputAccessGranted, driver.InputAccessDenied, driver.InputAccessRestricted} {
		controller := permissionController{query: func() (driver.InputAccess, error) { return access, nil }, allocate: func() (*permissionSignal, error) { t.Fatal("unexpected authorization prompt"); return nil, nil }}
		err := controller.request(context.Background())
		if access == driver.InputAccessGranted && err != nil || access != driver.InputAccessGranted && !errors.Is(err, driver.ErrPermission) {
			t.Fatalf("access %q: %v", access, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	controller := permissionController{query: func() (driver.InputAccess, error) {
		t.Fatal("canceled request queried native authorization")
		return driver.InputAccessPrompt, nil
	}}
	if err := controller.request(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
