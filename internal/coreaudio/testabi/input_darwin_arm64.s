//go:build darwin && arm64
#include "textflag.h"

// Test-only C AudioUnitRender stand-in. unit points to the mmap fixture:
// status@0, source@8, frameBytes@16, frames@24, bus@32, memcpy@40, flags@48.
TEXT ·fakeInputRender(SB), NOSPLIT|NOFRAME, $0-0
 STP.W (R29,R30), -32(RSP)
 MOVD RSP,R29
 STP (R19,R20),16(RSP)
 MOVD R0,R19
 MOVD R4,24(R19)
 MOVD R3,32(R19)
 MOVW (R19),R0
 CBNZW R0,done
 MOVWU 48(R19),R9
 MOVW R9,(R1)
 MOVD 16(R5),R0
 MOVD 8(R19),R1
 MOVD 16(R19),R9
 MUL R9,R4,R2
 MOVD 40(R19),R9
 BL (R9)
 MOVD ZR,R0
 done:
 LDP 16(RSP),(R19,R20)
 LDP.P 32(RSP),(R29,R30)
 RET
GLOBL ·InputRenderABI(SB),NOPTR|RODATA,$8
DATA ·InputRenderABI(SB)/8,$·fakeInputRender(SB)
