//go:build darwin && amd64

#include "textflag.h"
#include "go_asm.h"

// System V C ABI counterpart of capture_darwin_arm64.s. Preserve the foreign
// thread's callee-saved registers, including R14, without ever reading g.
TEXT ·captureInput(SB), NOSPLIT|NOFRAME, $0-0
	SUBQ $56, SP
	MOVQ BP, 48(SP)
	LEAQ 48(SP), BP
	MOVQ BX, 0(SP)
	MOVQ R12, 8(SP)
	MOVQ R13, 16(SP)
	MOVQ R14, 24(SP)
	MOVQ R15, 32(SP)
	MOVQ DI, BX
	MOVQ DX, R12
	MOVL R8, R13
	MOVQ captureCtrl_sigmask(BX), AX
	TESTQ AX, AX
	JEQ masked
	MOVQ $1, captureCtrl_masked(BX)
	MOVL $1, DI
	LEAQ captureCtrl_sigset(BX), SI
	XORL DX, DX
	CALL AX
masked:
	TESTQ R13, R13
	JEQ done
	CMPQ captureCtrl_status(BX), $0
	JNE signal
	CMPQ R13, captureCtrl_maxFrames(BX)
	JHI bad
	MOVQ captureCtrl_frameBytes(BX), AX
	IMULQ R13, AX
	MOVL AX, captureCtrl_buffer+audioBufferList_dataByteSize(BX)
	MOVQ captureCtrl_scratch(BX), AX
	MOVQ AX, captureCtrl_buffer+audioBufferList_data(BX)
	MOVQ $0, captureCtrl_actionFlags(BX)
	MOVQ captureCtrl_unit(BX), DI
	LEAQ captureCtrl_actionFlags(BX), SI
	MOVQ R12, DX
	MOVL $1, CX
	MOVQ R13, R8
	LEAQ captureCtrl_buffer(BX), R9
	MOVQ captureCtrl_render(BX), AX
	CALL AX
	TESTL AX, AX
	JNE failed
	MOVQ captureCtrl_buffer+audioBufferList_data(BX), R14
	CMPQ R14, captureCtrl_scratch(BX)
	JNE bad
	MOVQ captureCtrl_frameBytes(BX), AX
	IMULQ R13, AX
	MOVL captureCtrl_buffer+audioBufferList_dataByteSize(BX), CX
	CMPQ CX, AX
	JCS bad
	TESTL $16, captureCtrl_actionFlags(BX)
	JEQ hasdata
	MOVQ R14, DI
	XORL SI, SI
	MOVQ AX, DX
	MOVQ captureCtrl_memset(BX), AX
	CALL AX
hasdata:
	CMPQ captureCtrl_paused(BX), $0
	JNE dropped
	MOVQ captureCtrl_head(BX), AX
	MOVQ AX, DX
	SUBQ captureCtrl_tail(BX), DX
	MOVQ captureCtrl_mask(BX), CX
	INCQ CX
	SUBQ DX, CX
	CMPQ CX, R13
	JCS overflow
	MOVQ captureCtrl_head(BX), DX
	ANDQ captureCtrl_mask(BX), DX
	MOVQ captureCtrl_mask(BX), CX
	INCQ CX
	SUBQ DX, CX
	MOVQ R13, R15
	CMPQ CX, R15
	JCC firstok
	MOVQ CX, R15
firstok:
	MOVQ captureCtrl_frameBytes(BX), AX
	IMULQ AX, DX
	MOVQ captureCtrl_data(BX), DI
	ADDQ DX, DI
	MOVQ R14, SI
	MOVQ R15, DX
	IMULQ AX, DX
	MOVQ captureCtrl_memcpy(BX), AX
	CALL AX
	MOVQ R13, DX
	SUBQ R15, DX
	JEQ publish
	MOVQ captureCtrl_frameBytes(BX), CX
	MOVQ R15, AX
	IMULQ CX, AX
	LEAQ (R14)(AX*1), SI
	IMULQ CX, DX
	MOVQ captureCtrl_data(BX), DI
	MOVQ captureCtrl_memcpy(BX), AX
	CALL AX
publish:
	MOVQ captureCtrl_metaHead(BX), AX
	ANDQ captureCtrl_mask(BX), AX
	IMULQ $captureSpan__size, AX
	ADDQ captureCtrl_meta(BX), AX
	MOVQ captureCtrl_head(BX), CX
	MOVQ CX, captureSpan_start(AX)
	ADDQ R13, CX
	MOVQ CX, captureSpan_end(AX)
	XORL DX, DX
	XORL SI, SI
	XORL DI, DI
	TESTQ R12, R12
	JEQ stamp
	MOVQ (R12), DX
	MOVQ 8(R12), SI
	MOVL 56(R12), DI
stamp:
	MOVQ DX, captureSpan_sampleTime(AX)
	MOVQ SI, captureSpan_hostTime(AX)
	MOVQ DI, captureSpan_flags(AX)
	INCQ captureCtrl_metaHead(BX)
	MOVQ CX, captureCtrl_head(BX)
	JMP signal
overflow:
	INCQ captureCtrl_overflows(BX)
	MOVQ $1, captureCtrl_paused(BX)
dropped:
	ADDQ R13, captureCtrl_droppedFrames(BX)
	JMP signal
bad:
	MOVL $-10851, AX
failed:
	MOVLQSX AX, AX
	MOVQ AX, captureCtrl_status(BX)
signal:
	MOVL captureCtrl_sem(BX), DI
	MOVQ captureCtrl_signal(BX), AX
	CALL AX
done:
	XORL AX, AX
	MOVQ 0(SP), BX
	MOVQ 8(SP), R12
	MOVQ 16(SP), R13
	MOVQ 24(SP), R14
	MOVQ 32(SP), R15
	MOVQ 48(SP), BP
	ADDQ $56, SP
	RET

GLOBL ·captureABI0(SB), NOPTR|RODATA, $8
DATA ·captureABI0(SB)/8, $·captureInput(SB)
