//go:build darwin && (amd64 || arm64)

package coreaudio

import "unsafe"

//go:cgo_import_dynamic tymbal_auth_dlopen dlopen "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic tymbal_auth_dlsym dlsym "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic tymbal_auth_objc_getClass objc_getClass "/usr/lib/libobjc.A.dylib"
//go:cgo_import_dynamic tymbal_auth_sel_registerName sel_registerName "/usr/lib/libobjc.A.dylib"
//go:cgo_import_dynamic tymbal_auth_objc_msgSend objc_msgSend "/usr/lib/libobjc.A.dylib"
//go:cgo_import_dynamic tymbal_auth_CFStringCreateWithCString CFStringCreateWithCString "/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation"

var (
	fnAuthdlopen                    uintptr
	fnAuthdlsym                     uintptr
	fnAuthobjc_getClass             uintptr
	fnAuthsel_registerName          uintptr
	fnAuthobjc_msgSend              uintptr
	fnAuthCFStringCreateWithCString uintptr
)

// These APIs copy the temporary string or return C-owned lookup results.
// No caller string is retained in an OS-visible object.
func authLoadFramework() uintptr {
	path := []byte("/System/Library/Frameworks/AVFoundation.framework/Versions/A/AVFoundation\x00")
	value, _, _ := syscall_syscall(fnAuthdlopen, uintptr(unsafe.Pointer(&path[0])), 5, 0)
	return value
}
func authClass(name string) uintptr {
	c := append([]byte(name), 0)
	value, _, _ := syscall_syscall(fnAuthobjc_getClass, uintptr(unsafe.Pointer(&c[0])), 0, 0)
	return value
}
func authSelector(name string) uintptr {
	c := append([]byte(name), 0)
	value, _, _ := syscall_syscall(fnAuthsel_registerName, uintptr(unsafe.Pointer(&c[0])), 0, 0)
	return value
}
func authMediaType() uintptr {
	c := []byte("soun\x00")
	value, _, _ := syscall_syscall(fnAuthCFStringCreateWithCString, 0, uintptr(unsafe.Pointer(&c[0])), 0x08000100)
	return value
}
func authGlobalBlockClass() uintptr {
	c := []byte("_NSConcreteGlobalBlock\x00")
	value, _, _ := syscall_syscall(fnAuthdlsym, ^uintptr(1), uintptr(unsafe.Pointer(&c[0])), 0)
	return value
}
