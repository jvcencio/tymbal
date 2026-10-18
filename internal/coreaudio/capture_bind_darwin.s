//go:build darwin && (amd64 || arm64)

#include "textflag.h"

TEXT tymbal_capture_AudioHardwareCreateAggregateDevice_trampoline<>(SB), NOSPLIT, $0-0
	JMP tymbal_capture_AudioHardwareCreateAggregateDevice(SB)
GLOBL ·fnCaptureAudioHardwareCreateAggregateDevice(SB), RODATA, $8
DATA ·fnCaptureAudioHardwareCreateAggregateDevice(SB)/8, $tymbal_capture_AudioHardwareCreateAggregateDevice_trampoline<>(SB)

TEXT tymbal_capture_AudioHardwareDestroyAggregateDevice_trampoline<>(SB), NOSPLIT, $0-0
	JMP tymbal_capture_AudioHardwareDestroyAggregateDevice(SB)
GLOBL ·fnCaptureAudioHardwareDestroyAggregateDevice(SB), RODATA, $8
DATA ·fnCaptureAudioHardwareDestroyAggregateDevice(SB)/8, $tymbal_capture_AudioHardwareDestroyAggregateDevice_trampoline<>(SB)

TEXT tymbal_capture_AudioUnitGetProperty_trampoline<>(SB), NOSPLIT, $0-0
	JMP tymbal_capture_AudioUnitGetProperty(SB)
GLOBL ·fnCaptureAudioUnitGetProperty(SB), RODATA, $8
DATA ·fnCaptureAudioUnitGetProperty(SB)/8, $tymbal_capture_AudioUnitGetProperty_trampoline<>(SB)

TEXT tymbal_capture_AudioUnitRender_trampoline<>(SB), NOSPLIT, $0-0
	JMP tymbal_capture_AudioUnitRender(SB)
GLOBL ·fnCaptureAudioUnitRender(SB), RODATA, $8
DATA ·fnCaptureAudioUnitRender(SB)/8, $tymbal_capture_AudioUnitRender_trampoline<>(SB)

TEXT tymbal_capture_CFDataCreate_trampoline<>(SB), NOSPLIT, $0-0
	JMP tymbal_capture_CFDataCreate(SB)
GLOBL ·fnCaptureCFDataCreate(SB), RODATA, $8
DATA ·fnCaptureCFDataCreate(SB)/8, $tymbal_capture_CFDataCreate_trampoline<>(SB)

TEXT tymbal_capture_CFPropertyListCreateWithData_trampoline<>(SB), NOSPLIT, $0-0
	JMP tymbal_capture_CFPropertyListCreateWithData(SB)
GLOBL ·fnCaptureCFPropertyListCreateWithData(SB), RODATA, $8
DATA ·fnCaptureCFPropertyListCreateWithData(SB)/8, $tymbal_capture_CFPropertyListCreateWithData_trampoline<>(SB)

TEXT tymbal_capture_mach_absolute_time_trampoline<>(SB), NOSPLIT, $0-0
	JMP tymbal_capture_mach_absolute_time(SB)
GLOBL ·fnCapturemach_absolute_time(SB), RODATA, $8
DATA ·fnCapturemach_absolute_time(SB)/8, $tymbal_capture_mach_absolute_time_trampoline<>(SB)

TEXT tymbal_capture_mach_timebase_info_trampoline<>(SB), NOSPLIT, $0-0
	JMP tymbal_capture_mach_timebase_info(SB)
GLOBL ·fnCapturemach_timebase_info(SB), RODATA, $8
DATA ·fnCapturemach_timebase_info(SB)/8, $tymbal_capture_mach_timebase_info_trampoline<>(SB)
