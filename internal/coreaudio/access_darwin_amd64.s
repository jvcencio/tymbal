//go:build darwin && amd64
#include "textflag.h"
#include "go_asm.h"

TEXT ·permissionCompleted(SB),NOSPLIT|NOFRAME,$0-0
	SUBQ $24,SP
	MOVQ BP,16(SP)
	LEAQ 16(SP),BP
	MOVQ BX,0(SP)
	MOVQ R12,8(SP)
	MOVQ permissionBlock_result(DI),BX
	MOVBLZX SI,R12
	MOVQ permissionResult_sigmask(BX),AX
	TESTQ AX,AX
	JEQ publish
	MOVL $1,DI
	LEAQ permissionResult_sigset(BX),SI
	XORL DX,DX
	CALL AX
publish:
	XORL AX,AX
	TESTL R12,R12
	SETNE AL
	INCQ AX
	MOVQ AX,permissionResult_state(BX)
	MOVQ 0(SP),BX
	MOVQ 8(SP),R12
	MOVQ 16(SP),BP
	ADDQ $24,SP
	RET
GLOBL ·permissionCompletedABI(SB),NOPTR|RODATA,$8
DATA ·permissionCompletedABI(SB)/8,$·permissionCompleted(SB)
