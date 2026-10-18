//go:build darwin && arm64

#include "textflag.h"
#include "go_asm.h"

// A C-ABI input callback. No Go stack, Go pointers or Go calls are involved.
// AUHAL invokes input separately from output, with its own capture timestamp.
TEXT ·captureInput(SB), NOSPLIT|NOFRAME, $0-0
	STP.W (R29, R30), -64(RSP)
	MOVD RSP, R29
	STP (R19, R20), 16(RSP)
	STP (R21, R22), 32(RSP)
	STP (R23, R24), 48(RSP)
	MOVD R0, R19
	MOVD R2, R20
	MOVWU R4, R21
	MOVD captureCtrl_sigmask(R19), R9
	CBZ R9, masked
	MOVD $1, R10
	MOVD R10, captureCtrl_masked(R19)
	MOVD $1, R0
	ADD $captureCtrl_sigset, R19, R1
	MOVD ZR, R2
	BL (R9)
masked:
	CBZ R21, done
	MOVD captureCtrl_status(R19), R9
	CBNZ R9, signal
	MOVD captureCtrl_maxFrames(R19), R9
	CMP R9, R21
	BHI bad
	MOVD captureCtrl_frameBytes(R19), R9
	MUL R9, R21, R10
	MOVW R10, captureCtrl_buffer+audioBufferList_dataByteSize(R19)
	MOVD captureCtrl_scratch(R19), R10
	MOVD R10, captureCtrl_buffer+audioBufferList_data(R19)
	MOVD ZR, captureCtrl_actionFlags(R19)
	MOVD captureCtrl_unit(R19), R0
	ADD $captureCtrl_actionFlags, R19, R1
	MOVD R20, R2
	MOVD $1, R3
	MOVD R21, R4
	ADD $captureCtrl_buffer, R19, R5
	MOVD captureCtrl_render(R19), R9
	BL (R9)
	CBNZW R0, failed
	MOVD captureCtrl_buffer+audioBufferList_data(R19), R24
	MOVD captureCtrl_scratch(R19), R9
	CMP R9, R24
	BNE bad
	MOVD captureCtrl_frameBytes(R19), R9
	MUL R9, R21, R10
	MOVWU captureCtrl_buffer+audioBufferList_dataByteSize(R19), R11
	CMP R10, R11
	BLO bad
	MOVWU captureCtrl_actionFlags(R19), R9
	TBZ $4, R9, hasdata
	MOVD R24, R0
	MOVD ZR, R1
	MOVD R10, R2
	MOVD captureCtrl_memset(R19), R9
	BL (R9)
hasdata:
	ADD $captureCtrl_paused, R19, R9
	LDAR (R9), R9
	CBNZ R9, dropped
	MOVD captureCtrl_head(R19), R22
	ADD $captureCtrl_tail, R19, R9
	LDAR (R9), R10
	SUB R10, R22, R10
	MOVD captureCtrl_mask(R19), R9
	ADD $1, R9, R9
	SUB R10, R9, R10
	CMP R21, R10
	BLO overflow
	MOVD captureCtrl_mask(R19), R9
	AND R9, R22, R10
	ADD $1, R9, R9
	SUB R10, R9, R9
	CMP R9, R21
	CSEL LO, R21, R9, R23
	MOVD captureCtrl_frameBytes(R19), R11
	MUL R11, R10, R10
	MOVD captureCtrl_data(R19), R0
	ADD R10, R0
	MOVD R24, R1
	MUL R11, R23, R2
	MOVD captureCtrl_memcpy(R19), R9
	BL (R9)
	SUB R23, R21, R10
	CBZ R10, publish
	MOVD captureCtrl_frameBytes(R19), R11
	MUL R11, R23, R12
	ADD R12, R24, R1
	MOVD captureCtrl_data(R19), R0
	MUL R11, R10, R2
	MOVD captureCtrl_memcpy(R19), R9
	BL (R9)
publish:
	MOVD captureCtrl_metaHead(R19), R9
	MOVD captureCtrl_mask(R19), R10
	AND R9, R10, R10
	MOVD $captureSpan__size, R11
	MUL R11, R10, R10
	MOVD captureCtrl_meta(R19), R11
	ADD R10, R11
	MOVD R22, captureSpan_start(R11)
	ADD R21, R22, R22
	MOVD R22, captureSpan_end(R11)
	MOVD ZR, R12
	MOVD ZR, R13
	MOVD ZR, R14
	CBZ R20, stamp
	MOVD (R20), R12
	MOVD 8(R20), R13
	MOVWU 56(R20), R14
stamp:
	MOVD R12, captureSpan_sampleTime(R11)
	MOVD R13, captureSpan_hostTime(R11)
	MOVD R14, captureSpan_flags(R11)
	ADD $1, R9
	MOVD R9, captureCtrl_metaHead(R19)
	STLR R22, (R19)
	B signal
overflow:
	MOVD captureCtrl_overflows(R19), R9
	ADD $1, R9
	MOVD R9, captureCtrl_overflows(R19)
	MOVD $1, R9
	ADD $captureCtrl_paused, R19, R10
	STLR R9, (R10)
dropped:
	MOVD captureCtrl_droppedFrames(R19), R9
	ADD R21, R9
	MOVD R9, captureCtrl_droppedFrames(R19)
	B signal
bad:
	MOVD $-10851, R0
failed:
	SXTW R0, R0
	ADD $captureCtrl_status, R19, R9
	STLR R0, (R9)
signal:
	MOVWU captureCtrl_sem(R19), R0
	MOVD captureCtrl_signal(R19), R9
	BL (R9)
done:
	MOVD ZR, R0
	LDP 16(RSP), (R19, R20)
	LDP 32(RSP), (R21, R22)
	LDP 48(RSP), (R23, R24)
	LDP.P 64(RSP), (R29, R30)
	RET

GLOBL ·captureABI0(SB), NOPTR|RODATA, $8
DATA ·captureABI0(SB)/8, $·captureInput(SB)
