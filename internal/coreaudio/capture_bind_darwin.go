//go:build darwin && (amd64 || arm64)

package coreaudio

import "unsafe"

//go:cgo_import_dynamic tymbal_capture_AudioHardwareCreateAggregateDevice AudioHardwareCreateAggregateDevice "/System/Library/Frameworks/CoreAudio.framework/Versions/A/CoreAudio"
//go:cgo_import_dynamic tymbal_capture_AudioHardwareDestroyAggregateDevice AudioHardwareDestroyAggregateDevice "/System/Library/Frameworks/CoreAudio.framework/Versions/A/CoreAudio"
//go:cgo_import_dynamic tymbal_capture_AudioUnitGetProperty AudioUnitGetProperty "/System/Library/Frameworks/AudioToolbox.framework/Versions/A/AudioToolbox"
//go:cgo_import_dynamic tymbal_capture_AudioUnitRender AudioUnitRender "/System/Library/Frameworks/AudioToolbox.framework/Versions/A/AudioToolbox"
//go:cgo_import_dynamic tymbal_capture_CFDataCreate CFDataCreate "/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation"
//go:cgo_import_dynamic tymbal_capture_CFPropertyListCreateWithData CFPropertyListCreateWithData "/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation"
//go:cgo_import_dynamic tymbal_capture_mach_absolute_time mach_absolute_time "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic tymbal_capture_mach_timebase_info mach_timebase_info "/usr/lib/libSystem.B.dylib"

var (
	fnCaptureAudioHardwareCreateAggregateDevice  uintptr
	fnCaptureAudioHardwareDestroyAggregateDevice uintptr
	fnCaptureAudioUnitGetProperty                uintptr
	fnCaptureAudioUnitRender                     uintptr
	fnCaptureCFDataCreate                        uintptr
	fnCaptureCFPropertyListCreateWithData        uintptr
	fnCapturemach_absolute_time                  uintptr
	fnCapturemach_timebase_info                  uintptr
)

func createAggregateDevice(description uintptr, device *uint32) int32 {
	r, _, _ := syscall_syscall(fnCaptureAudioHardwareCreateAggregateDevice, description, uintptr(unsafe.Pointer(device)), 0)
	return int32(r)
}

func destroyAggregateDevice(device uint32) int32 {
	r, _, _ := syscall_syscall(fnCaptureAudioHardwareDestroyAggregateDevice, uintptr(device), 0, 0)
	return int32(r)
}

func audioUnitGetProperty(unit uintptr, property, scope, element uint32, data unsafe.Pointer, size *uint32) int32 {
	r, _, _ := syscall_syscall6(fnCaptureAudioUnitGetProperty, unit, uintptr(property), uintptr(scope), uintptr(element), uintptr(data), uintptr(unsafe.Pointer(size)))
	return int32(r)
}

func cfPropertyList(data []byte) uintptr {
	if len(data) == 0 {
		return 0
	}
	ref, _, _ := syscall_syscall(fnCaptureCFDataCreate, 0, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)))
	if ref == 0 {
		return 0
	}
	defer cfRelease(ref)
	plist, _, _ := syscall_syscall6(fnCaptureCFPropertyListCreateWithData, 0, ref, 0, 0, 0, 0)
	return plist
}

func captureMachTimebase(numer, denom *uint32) int32 {
	var ratio [2]uint32
	r, _, _ := syscall_syscall(fnCapturemach_timebase_info, uintptr(unsafe.Pointer(&ratio[0])), 0, 0)
	*numer, *denom = ratio[0], ratio[1]
	return int32(r)
}

func captureMachTime() uint64 {
	r, _, _ := syscall_syscall(fnCapturemach_absolute_time, 0, 0, 0)
	return uint64(r)
}
