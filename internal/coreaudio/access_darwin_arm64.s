//go:build darwin && arm64
#include "textflag.h"
#include "go_asm.h"

// AVFoundation invokes a C block on its own dispatch thread. Publish the
// result into mmap memory, with no entry into the Go runtime.
TEXT ·permissionCompleted(SB),NOSPLIT|NOFRAME,$0-0
	STP.W (R29,R30),-32(RSP)
	MOVD RSP,R29
	STP (R19,R20),16(RSP)
	MOVD permissionBlock_result(R0),R19
	AND $255,R1,R20
	MOVD permissionResult_sigmask(R19),R9
	CBZ R9,publish
	MOVD $1,R0
	ADD $permissionResult_sigset,R19,R1
	MOVD ZR,R2
	BL (R9)
publish:
	CMPW $0,R20
	CSET NE,R20
	ADD $1,R20
	STLR R20,(R19)
	LDP 16(RSP),(R19,R20)
	LDP.P 32(RSP),(R29,R30)
	RET
GLOBL ·permissionCompletedABI(SB),NOPTR|RODATA,$8
DATA ·permissionCompletedABI(SB)/8,$·permissionCompleted(SB)
