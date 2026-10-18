//go:build darwin && amd64
#include "textflag.h"

// The same C fixture layout as fake_input_darwin_arm64.s.
TEXT ·fakeInputRender(SB), NOSPLIT|NOFRAME, $0-0
 SUBQ $24,SP
 MOVQ BP,16(SP)
 LEAQ 16(SP),BP
 MOVQ BX,0(SP)
 MOVQ DI,BX
 MOVQ R8,24(BX)
 MOVQ CX,32(BX)
 MOVL (BX),AX
 TESTL AX,AX
 JNE done
 MOVL 48(BX),AX
 MOVL AX,(SI)
 MOVQ 16(R9),DI
 MOVQ 8(BX),SI
 MOVQ R8,DX
 IMULQ 16(BX),DX
 MOVQ 40(BX),AX
 CALL AX
 XORL AX,AX
 done:
 MOVQ 0(SP),BX
 MOVQ 16(SP),BP
 ADDQ $24,SP
 RET
GLOBL ·InputRenderABI(SB),NOPTR|RODATA,$8
DATA ·InputRenderABI(SB)/8,$·fakeInputRender(SB)
