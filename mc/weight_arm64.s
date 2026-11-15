//go:build arm64 && !purego

#include "textflag.h"

// Explicit weighted prediction, NEON (ITU-T H.264 8.4.2.3.2, Equations 8-274
// to 8-276). Hand-written; the luma interpolation kernels are generated.
//
// Single list: 16-bit lanes are exact, |p*weight| <= 255*128 and the rounding
// term is at most 64:  V20 = weight, V21 = rounding, V22 = -logWD (SSHL count),
// V23 = offset.
// Two lists: the products need 32 bits:  V20 = w0, V21 = w1 (16-bit lanes),
// V22 = 2^logWD, V23 = -(logWD+1), V24 = (o0+o1+1)>>1 (32-bit lanes).
//
// SSHL by a negative count is an arithmetic shift right, floor like ">>".
// cmd/asm has no SSHL; it is emitted as a WORD (go-asmgen gap candidate).

// func weightUni8(dst *byte, stride, w, h, weight, rnd, shift, off int)
TEXT ·weightUni8(SB), NOSPLIT, $0-64
	MOVD dst+0(FP), R0
	MOVD stride+8(FP), R1
	MOVD w+16(FP), R2
	MOVD h+24(FP), R3
	MOVD weight+32(FP), R4
	MOVD rnd+40(FP), R5
	MOVD shift+48(FP), R6
	MOVD off+56(FP), R7
	VDUP R4, V20.H8
	VDUP R5, V21.H8
	VDUP R6, V22.H8
	VDUP R7, V23.H8
strip:
	MOVD R3, R8
	MOVD R0, R9
rows:
	FMOVD (R9), F0
	VUXTL V0.B8, V0.H8
	VMUL V20.H8, V0.H8, V0.H8
	VADD V21.H8, V0.H8, V0.H8
	WORD $0x4e764400 // SSHL V0.8H, V0.8H, V22.8H
	VADD V23.H8, V0.H8, V0.H8
	VSQXTUN V0.H8, V0.B8
	FMOVD F0, (R9)
	ADD R1, R9
	SUBS $1, R8
	BNE rows
	ADD $8, R0
	SUBS $8, R2
	BNE strip
	RET

// func weightUni4(dst *byte, stride, w, h, weight, rnd, shift, off int)
TEXT ·weightUni4(SB), NOSPLIT, $0-64
	MOVD dst+0(FP), R0
	MOVD stride+8(FP), R1
	MOVD h+24(FP), R3
	MOVD weight+32(FP), R4
	MOVD rnd+40(FP), R5
	MOVD shift+48(FP), R6
	MOVD off+56(FP), R7
	VDUP R4, V20.H8
	VDUP R5, V21.H8
	VDUP R6, V22.H8
	VDUP R7, V23.H8
rows:
	FMOVS (R0), F0
	VUXTL V0.B8, V0.H8
	VMUL V20.H8, V0.H8, V0.H8
	VADD V21.H8, V0.H8, V0.H8
	WORD $0x4e764400 // SSHL V0.8H, V0.8H, V22.8H
	VADD V23.H8, V0.H8, V0.H8
	VSQXTUN V0.H8, V0.B8
	FMOVS F0, (R0)
	ADD R1, R0
	SUBS $1, R3
	BNE rows
	RET

// func weightBi8(dst *byte, dstStride int, src *byte, srcStride, w, h, w0, w1, rnd, shift, off int)
TEXT ·weightBi8(SB), NOSPLIT, $0-88
	MOVD dst+0(FP), R0
	MOVD dstStride+8(FP), R1
	MOVD src+16(FP), R2
	MOVD srcStride+24(FP), R3
	MOVD w+32(FP), R4
	MOVD h+40(FP), R5
	MOVD w0+48(FP), R6
	MOVD w1+56(FP), R7
	MOVD rnd+64(FP), R8
	MOVD shift+72(FP), R9
	MOVD off+80(FP), R10
	VDUP R6, V20.H8
	VDUP R7, V21.H8
	VDUP R8, V22.S4
	VDUP R9, V23.S4
	VDUP R10, V24.S4
strip:
	MOVD R5, R11
	MOVD R0, R12
	MOVD R2, R13
rows:
	FMOVD (R12), F0
	FMOVD (R13), F1
	VUXTL V0.B8, V0.H8
	VUXTL V1.B8, V1.H8
	VMOV V22.B16, V2.B16
	VMOV V22.B16, V3.B16
	VSMLAL V20.H4, V0.H4, V2.S4
	VSMLAL V21.H4, V1.H4, V2.S4
	VSMLAL2 V20.H8, V0.H8, V3.S4
	VSMLAL2 V21.H8, V1.H8, V3.S4
	WORD $0x4eb74442 // SSHL V2.4S, V2.4S, V23.4S
	WORD $0x4eb74463 // SSHL V3.4S, V3.4S, V23.4S
	VADD V24.S4, V2.S4, V2.S4
	VADD V24.S4, V3.S4, V3.S4
	VSQXTUN V2.S4, V4.H4
	VSQXTUN2 V3.S4, V4.H8
	VUQXTN V4.H8, V4.B8
	FMOVD F4, (R12)
	ADD R1, R12
	ADD R3, R13
	SUBS $1, R11
	BNE rows
	ADD $8, R0
	ADD $8, R2
	SUBS $8, R4
	BNE strip
	RET

// func weightBi4(dst *byte, dstStride int, src *byte, srcStride, w, h, w0, w1, rnd, shift, off int)
TEXT ·weightBi4(SB), NOSPLIT, $0-88
	MOVD dst+0(FP), R0
	MOVD dstStride+8(FP), R1
	MOVD src+16(FP), R2
	MOVD srcStride+24(FP), R3
	MOVD h+40(FP), R5
	MOVD w0+48(FP), R6
	MOVD w1+56(FP), R7
	MOVD rnd+64(FP), R8
	MOVD shift+72(FP), R9
	MOVD off+80(FP), R10
	VDUP R6, V20.H8
	VDUP R7, V21.H8
	VDUP R8, V22.S4
	VDUP R9, V23.S4
	VDUP R10, V24.S4
rows:
	FMOVS (R0), F0
	FMOVS (R2), F1
	VUXTL V0.B8, V0.H8
	VUXTL V1.B8, V1.H8
	VMOV V22.B16, V2.B16
	VSMLAL V20.H4, V0.H4, V2.S4
	VSMLAL V21.H4, V1.H4, V2.S4
	WORD $0x4eb74442 // SSHL V2.4S, V2.4S, V23.4S
	VADD V24.S4, V2.S4, V2.S4
	VSQXTUN V2.S4, V4.H4
	VUQXTN V4.H8, V4.B8
	FMOVS F4, (R0)
	ADD R1, R0
	ADD R3, R2
	SUBS $1, R5
	BNE rows
	RET
