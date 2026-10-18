//go:build darwin && (amd64 || arm64)

package rt

import (
	"syscall"
	"time"
	"unsafe"
)

// SleepThread waits in libSystem rather than allocating a Go timer. Signals
// may end the sleep early, as on Linux. This keeps watchdog startup and sampling
// from adding allocations to the callback scope they are meant to observe.
func SleepThread(d time.Duration) {
	if d <= 0 {
		return
	}
	request := syscall.NsecToTimespec(int64(d))
	_, _, _ = syscall_syscall(fnnanosleep, uintptr(unsafe.Pointer(&request)), 0, 0)
}
