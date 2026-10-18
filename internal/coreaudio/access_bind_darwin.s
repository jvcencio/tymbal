//go:build darwin && (amd64 || arm64)

#include "textflag.h"

TEXT tymbal_auth_dlopen_trampoline<>(SB),NOSPLIT,$0-0
	JMP tymbal_auth_dlopen(SB)
GLOBL ·fnAuthdlopen(SB),RODATA,$8
DATA ·fnAuthdlopen(SB)/8,$tymbal_auth_dlopen_trampoline<>(SB)

TEXT tymbal_auth_dlsym_trampoline<>(SB),NOSPLIT,$0-0
	JMP tymbal_auth_dlsym(SB)
GLOBL ·fnAuthdlsym(SB),RODATA,$8
DATA ·fnAuthdlsym(SB)/8,$tymbal_auth_dlsym_trampoline<>(SB)

TEXT tymbal_auth_objc_getClass_trampoline<>(SB),NOSPLIT,$0-0
	JMP tymbal_auth_objc_getClass(SB)
GLOBL ·fnAuthobjc_getClass(SB),RODATA,$8
DATA ·fnAuthobjc_getClass(SB)/8,$tymbal_auth_objc_getClass_trampoline<>(SB)

TEXT tymbal_auth_sel_registerName_trampoline<>(SB),NOSPLIT,$0-0
	JMP tymbal_auth_sel_registerName(SB)
GLOBL ·fnAuthsel_registerName(SB),RODATA,$8
DATA ·fnAuthsel_registerName(SB)/8,$tymbal_auth_sel_registerName_trampoline<>(SB)

TEXT tymbal_auth_objc_msgSend_trampoline<>(SB),NOSPLIT,$0-0
	JMP tymbal_auth_objc_msgSend(SB)
GLOBL ·fnAuthobjc_msgSend(SB),RODATA,$8
DATA ·fnAuthobjc_msgSend(SB)/8,$tymbal_auth_objc_msgSend_trampoline<>(SB)

TEXT tymbal_auth_CFStringCreateWithCString_trampoline<>(SB),NOSPLIT,$0-0
	JMP tymbal_auth_CFStringCreateWithCString(SB)
GLOBL ·fnAuthCFStringCreateWithCString(SB),RODATA,$8
DATA ·fnAuthCFStringCreateWithCString(SB)/8,$tymbal_auth_CFStringCreateWithCString_trampoline<>(SB)
