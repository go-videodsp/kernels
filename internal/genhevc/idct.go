package main

import "fmt"

// The inverse transform of HEVC (ITU-T H.265 (V11) (01/2026), clause 8.6.4, Equations 8-314
// to 8-322) of one n x n block in 32-bit arithmetic, n = 4, 8, 16, 32, for the DCT and, with
// another matrix, the 4-point DST.
//
// Stage 1, the columns of d: g[i][x] = Clip3( -32768, 32767, ( sum over j <= maxY of
// M[j][i] * d[j][x] + 64 ) >> 7 ). Stage 2, the rows of g: out[y][i] = ( sum over k <= maxX of
// M[k][i] * g[y][k] + 2^(shift-1) ) >> shift. M[j][i] is the basis function of frequency j at
// position i: for the n-point DCT the row j * (32 / n) of the 32 x 32 matrix, first n
// entries, so the caller passes the matrix pointer and the stride in entries between
// frequencies. The inputs are within 16 bits, so no sum overflows 32 bits (94 371 840 at
// most), and the result is the one of the 64-bit evaluation.
//
// Both stages are one vector multiply-add loop: stage 1 has the lanes on the columns x (the
// broadcast M[j][i] times the row j of d), stage 2 on the positions i (the broadcast g[y][k]
// times the row k of M). Columns past maxX and frequencies past maxY are zero and skipped, at
// the granularity of a vector.

// neonIDCT is idctNEON(dst, d, g, mat *int32, matStride, n, maxX, maxY, shift int): lanes of
// four, for every n from 4.
func neonIDCT(e *emitter) {
	e.raw("// func idctNEON(dst, d, gb, mat *int32, matStride, n, maxX, maxY, shift int)\n")
	e.raw("TEXT ·idctNEON(SB), NOSPLIT, $0-72\n")
	e.f("MOVD d+8(FP), R1")
	e.f("MOVD gb+16(FP), R2")
	e.f("MOVD mat+24(FP), R3")
	e.f("MOVD matStride+32(FP), R4")
	e.f("LSL $2, R4, R4") // bytes between frequencies
	e.f("MOVD n+40(FP), R5")
	e.f("LSL $2, R5, R9") // bytes of a row
	e.f("MOVD maxX+48(FP), R6")
	e.f("MOVD maxY+56(FP), R7")
	e.f("MOVD shift+64(FP), R8")
	e.f("MOVD $-32768, R11")
	e.f("VDUP R11, V16.S4")
	e.f("MOVD $32767, R11")
	e.f("VDUP R11, V17.S4")
	e.f("MOVD $64, R11")
	e.f("VDUP R11, V18.S4")
	e.f("MOVD $1, R11")
	e.f("SUB $1, R8, R12")
	e.f("LSL R12, R11, R11")
	e.f("VDUP R11, V19.S4") // the rounding offset of stage 2
	e.f("NEG R8, R11")
	e.f("VDUP R11, V20.S4") // minus the shift: a left shift by it is the arithmetic right shift
	// stage 1
	e.f("LSR $2, R6, R10")
	e.f("ADD $1, R10, R10") // the groups of four columns up to maxX
	e.f("MOVD ZR, R11")
	e.label("s1g")
	e.f("MOVD ZR, R12")
	e.f("ADD R2, R11, R19")
	e.label("s1i")
	e.f("VEOR V0.B16, V0.B16, V0.B16")
	e.f("ADD R3, R12, R13")
	e.f("ADD R1, R11, R14")
	e.f("ADD $1, R7, R15")
	e.label("s1j")
	e.f("VLD1R (R13), [V2.S4]")
	e.f("VLD1 (R14), [V1.S4]")
	e.f("VMLA V1.S4, V2.S4, V0.S4")
	e.f("ADD R4, R13, R13")
	e.f("ADD R9, R14, R14")
	e.f("SUBS $1, R15, R15")
	e.f("BNE s1j")
	e.f("VADD V18.S4, V0.S4, V0.S4")
	e.f("VSSHR $7, V0.S4, V0.S4")
	e.f("VSMAX V16.S4, V0.S4, V0.S4")
	e.f("VSMIN V17.S4, V0.S4, V0.S4")
	e.f("VST1 [V0.S4], (R19)")
	e.f("ADD R9, R19, R19")
	e.f("ADD $4, R12, R12")
	e.f("CMP R9, R12")
	e.f("BLT s1i")
	e.f("ADD $16, R11, R11")
	e.f("SUBS $1, R10, R10")
	e.f("BNE s1g")
	// stage 2
	e.f("MOVD dst+0(FP), R0")
	e.f("MOVD R2, R12") // the row of g
	e.f("MOVD R5, R10") // rows left
	e.label("s2y")
	e.f("MOVD ZR, R14") // the offset of the output vector, in bytes
	e.label("s2o")
	e.f("VEOR V0.B16, V0.B16, V0.B16")
	e.f("MOVD R12, R15")
	e.f("ADD R3, R14, R16")
	e.f("ADD $1, R6, R17")
	e.label("s2k")
	e.f("VLD1R (R15), [V2.S4]")
	e.f("VLD1 (R16), [V1.S4]")
	e.f("VMLA V1.S4, V2.S4, V0.S4")
	e.f("ADD $4, R15, R15")
	e.f("ADD R4, R16, R16")
	e.f("SUBS $1, R17, R17")
	e.f("BNE s2k")
	e.f("VADD V19.S4, V0.S4, V0.S4")
	e.f("VSSHL V20.S4, V0.S4, V0.S4")
	e.f("ADD R0, R14, R20")
	e.f("VST1 [V0.S4], (R20)")
	e.f("ADD $16, R14, R14")
	e.f("CMP R9, R14")
	e.f("BLT s2o")
	e.f("ADD R9, R12, R12")
	e.f("ADD R9, R0, R0")
	e.f("SUBS $1, R10, R10")
	e.f("BNE s2y")
	e.f("RET")
	e.raw("\n")
}

// avxIDCT is idctAVX2x8 (lanes of eight, n from 8) or idctAVX2x4 (lanes of four, n = 4).
func avxIDCT(e *emitter, lanes int) {
	reg, log2L := "Y", 3
	if lanes == 4 {
		reg, log2L = "X", 2
	}
	name := fmt.Sprintf("idctAVX2x%d", lanes)
	e.raw(fmt.Sprintf("// func %s(dst, d, gb, mat *int32, matStride, n, maxX, maxY, shift int)\n", name))
	e.raw(fmt.Sprintf("TEXT ·%s(SB), NOSPLIT, $0-72\n", name))
	e.f("MOVQ d+8(FP), SI")
	e.f("MOVQ gb+16(FP), BX")
	e.f("MOVQ mat+24(FP), R8")
	e.f("MOVQ matStride+32(FP), R9")
	e.f("SHLQ $2, R9") // bytes between frequencies
	e.f("MOVQ n+40(FP), R10")
	e.f("SHLQ $2, R10") // bytes of a row
	for _, c := range []struct {
		r string
		v int
	}{{"3", -32768}, {"4", 32767}, {"5", 64}} {
		e.f("MOVQ $%d, AX", c.v)
		e.f("MOVQ AX, X%s", c.r)
		e.f("VPBROADCASTD X%s, %s%s", c.r, reg, c.r)
	}
	e.f("MOVQ maxX+48(FP), DX")
	e.f("SHRQ $%d, DX", log2L)
	e.f("INCQ DX") // the groups of columns up to maxX
	e.f("XORQ R11, R11")
	e.label("s1g")
	e.f("XORQ AX, AX")
	e.f("LEAQ (BX)(R11*1), CX")
	e.label("s1i")
	e.f("VPXOR %s0, %s0, %s0", reg, reg, reg)
	e.f("LEAQ (R8)(AX*1), R14")
	e.f("LEAQ (SI)(R11*1), R15")
	e.f("MOVQ maxY+56(FP), DI")
	e.f("INCQ DI")
	e.label("s1j")
	e.f("VPBROADCASTD (R14), %s2", reg)
	e.f("VPMULLD (R15), %s2, %s1", reg, reg)
	e.f("VPADDD %s1, %s0, %s0", reg, reg, reg)
	e.f("ADDQ R9, R14")
	e.f("ADDQ R10, R15")
	e.f("DECQ DI")
	e.f("JNZ s1j")
	e.f("VPADDD %s5, %s0, %s0", reg, reg, reg)
	e.f("VPSRAD $7, %s0, %s0", reg, reg)
	e.f("VPMAXSD %s3, %s0, %s0", reg, reg, reg)
	e.f("VPMINSD %s4, %s0, %s0", reg, reg, reg)
	e.f("VMOVDQU %s0, (CX)", reg)
	e.f("ADDQ R10, CX")
	e.f("ADDQ $4, AX")
	e.f("CMPQ AX, R10")
	e.f("JLT s1i")
	e.f("ADDQ $%d, R11", 4*lanes)
	e.f("DECQ DX")
	e.f("JNZ s1g")
	// stage 2
	e.f("MOVQ shift+64(FP), CX")
	e.f("MOVQ CX, X14")
	e.f("MOVQ $1, AX")
	e.f("DECQ CX")
	e.f("SHLQ CX, AX")
	e.f("MOVQ AX, X6")
	e.f("VPBROADCASTD X6, %s6", reg)
	e.f("MOVQ dst+0(FP), DI")
	e.f("MOVQ BX, R13")
	e.f("MOVQ n+40(FP), DX")
	e.label("s2y")
	e.f("XORQ AX, AX")
	e.label("s2o")
	e.f("VPXOR %s0, %s0, %s0", reg, reg, reg)
	e.f("MOVQ R13, R15")
	e.f("LEAQ (R8)(AX*1), R14")
	e.f("MOVQ maxX+48(FP), R11")
	e.f("INCQ R11")
	e.label("s2k")
	e.f("VPBROADCASTD (R15), %s2", reg)
	e.f("VPMULLD (R14), %s2, %s1", reg, reg)
	e.f("VPADDD %s1, %s0, %s0", reg, reg, reg)
	e.f("ADDQ $4, R15")
	e.f("ADDQ R9, R14")
	e.f("DECQ R11")
	e.f("JNZ s2k")
	e.f("VPADDD %s6, %s0, %s0", reg, reg, reg)
	e.f("VPSRAD X14, %s0, %s0", reg, reg)
	e.f("VMOVDQU %s0, (DI)(AX*1)", reg)
	e.f("ADDQ $%d, AX", 4*lanes)
	e.f("CMPQ AX, R10")
	e.f("JLT s2o")
	e.f("ADDQ R10, R13")
	e.f("ADDQ R10, DI")
	e.f("DECQ DX")
	e.f("JNZ s2y")
	e.f("VZEROUPPER")
	e.f("RET")
	e.raw("\n")
}
