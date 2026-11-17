package main

import "fmt"

// The kernels that write the bytes of an 8-bit picture: Average, WeightUni, WeightBi, SAOBand
// and SAOEdge with a []uint8 destination (and a []uint8 source where they have one). The
// arithmetic is that of average.go, weight.go and sao.go, at bit depth 8 (the band shift is 3,
// the largest sample 255); the results are narrowed to bytes in registers before the store, so
// an 8-bit picture no longer goes through a block of 16-bit samples that is narrowed afterwards.
// The strides and offsets are in bytes.

// urhaddB is URHADD Vd.<T>, Vn.<T>, Vm.<T> on bytes: 16B when q is 1, 8B when it is 0. Base
// 0x2e201400: U=1, size=00, opcode 00010 "rounding halving add", with Q at bit 30.
func urhaddB(q, d, n, m int) uint32 {
	return 0x2e201400 | uint32(q)<<30 | uint32(m)<<16 | uint32(n)<<5 | uint32(d)
}

// ---- Average ----

func neonAverageB(e *emitter) {
	e.raw("// func averageBNEON(dst *uint8, dstStride int, a *uint8, aStride int, b *uint8, bStride, w, h int)\n")
	e.raw("TEXT ·averageBNEON(SB), NOSPLIT, $0-64\n")
	e.f("MOVD dst+0(FP), R0")
	e.f("MOVD dstStride+8(FP), R1")
	e.f("MOVD a+16(FP), R2")
	e.f("MOVD aStride+24(FP), R3")
	e.f("MOVD b+32(FP), R4")
	e.f("MOVD bStride+40(FP), R5")
	e.f("MOVD w+48(FP), R6")
	e.f("MOVD h+56(FP), R7")
	e.label("rows")
	e.f("MOVD R2, R8")
	e.f("MOVD R4, R9")
	e.f("MOVD R0, R10")
	e.f("MOVD R6, R11")
	e.label("cols32")
	e.f("CMP $32, R11")
	e.f("BLT tail16")
	e.f("VLD1.P 32(R8), [V0.B16, V1.B16]")
	e.f("VLD1.P 32(R9), [V2.B16, V3.B16]")
	e.f("WORD $0x%08x // URHADD V0.16B, V0.16B, V2.16B", urhaddB(1, 0, 0, 2))
	e.f("WORD $0x%08x // URHADD V1.16B, V1.16B, V3.16B", urhaddB(1, 1, 1, 3))
	e.f("VST1.P [V0.B16, V1.B16], 32(R10)")
	e.f("SUB $32, R11, R11")
	e.f("B cols32")
	e.label("tail16")
	e.f("CMP $16, R11")
	e.f("BLT tail8")
	e.f("VLD1.P 16(R8), [V0.B16]")
	e.f("VLD1.P 16(R9), [V2.B16]")
	e.f("WORD $0x%08x // URHADD V0.16B, V0.16B, V2.16B", urhaddB(1, 0, 0, 2))
	e.f("VST1.P [V0.B16], 16(R10)")
	e.f("SUB $16, R11, R11")
	e.label("tail8")
	e.f("CMP $8, R11")
	e.f("BLT tail4")
	e.f("VLD1.P 8(R8), [V0.B8]")
	e.f("VLD1.P 8(R9), [V2.B8]")
	e.f("WORD $0x%08x // URHADD V0.8B, V0.8B, V2.8B", urhaddB(0, 0, 0, 2))
	e.f("VST1.P [V0.B8], 8(R10)")
	e.f("SUB $8, R11, R11")
	e.label("tail4")
	e.f("CMP $4, R11")
	e.f("BLT tail2")
	e.f("FMOVS (R8), F0")
	e.f("FMOVS (R9), F2")
	e.f("WORD $0x%08x // URHADD V0.8B, V0.8B, V2.8B (four lanes used)", urhaddB(0, 0, 0, 2))
	e.f("FMOVS F0, (R10)")
	e.f("ADD $4, R8, R8")
	e.f("ADD $4, R9, R9")
	e.f("ADD $4, R10, R10")
	e.f("SUB $4, R11, R11")
	e.label("tail2")
	e.f("CBZ R11, next")
	e.f("MOVHU (R8), R12")
	e.f("MOVHU (R9), R13")
	e.f("FMOVS R12, F0")
	e.f("FMOVS R13, F2")
	e.f("WORD $0x%08x // URHADD V0.8B, V0.8B, V2.8B (two lanes used)", urhaddB(0, 0, 0, 2))
	e.f("VMOV V0.H[0], R12")
	e.f("MOVH R12, (R10)")
	e.label("next")
	e.f("ADD R3, R2, R2")
	e.f("ADD R5, R4, R4")
	e.f("ADD R1, R0, R0")
	e.f("SUBS $1, R7, R7")
	e.f("BNE rows")
	e.f("RET")
	e.raw("\n")
}

func avxAverageB(e *emitter) {
	e.raw("// func averageBAVX2(dst *uint8, dstStride int, a *uint8, aStride int, b *uint8, bStride, w, h int)\n")
	e.raw("TEXT ·averageBAVX2(SB), NOSPLIT, $0-64\n")
	e.f("MOVQ dst+0(FP), DI")
	e.f("MOVQ dstStride+8(FP), R8")
	e.f("MOVQ a+16(FP), SI")
	e.f("MOVQ aStride+24(FP), R9")
	e.f("MOVQ b+32(FP), BX")
	e.f("MOVQ bStride+40(FP), R10")
	e.f("MOVQ w+48(FP), CX")
	e.f("MOVQ h+56(FP), DX")
	e.label("rows")
	e.f("MOVQ SI, R12")
	e.f("MOVQ BX, R13")
	e.f("MOVQ DI, AX")
	e.f("MOVQ CX, R11")
	e.label("cols32")
	e.f("CMPQ R11, $32")
	e.f("JLT tail16")
	e.f("VMOVDQU (R12), Y0")
	e.f("VPAVGB (R13), Y0, Y0")
	e.f("VMOVDQU Y0, (AX)")
	e.f("ADDQ $32, R12")
	e.f("ADDQ $32, R13")
	e.f("ADDQ $32, AX")
	e.f("SUBQ $32, R11")
	e.f("JMP cols32")
	e.label("tail16")
	e.f("CMPQ R11, $16")
	e.f("JLT tail8")
	e.f("VMOVDQU (R12), X0")
	e.f("VPAVGB (R13), X0, X0")
	e.f("VMOVDQU X0, (AX)")
	e.f("ADDQ $16, R12")
	e.f("ADDQ $16, R13")
	e.f("ADDQ $16, AX")
	e.f("SUBQ $16, R11")
	e.label("tail8")
	e.f("CMPQ R11, $8")
	e.f("JLT tail4")
	e.f("VMOVQ (R12), X0")
	e.f("VMOVQ (R13), X1")
	e.f("VPAVGB X1, X0, X0")
	e.f("VMOVQ X0, (AX)")
	e.f("ADDQ $8, R12")
	e.f("ADDQ $8, R13")
	e.f("ADDQ $8, AX")
	e.f("SUBQ $8, R11")
	e.label("tail4")
	e.f("CMPQ R11, $4")
	e.f("JLT tail2")
	e.f("VMOVD (R12), X0")
	e.f("VMOVD (R13), X1")
	e.f("VPAVGB X1, X0, X0")
	e.f("VMOVD X0, (AX)")
	e.f("ADDQ $4, R12")
	e.f("ADDQ $4, R13")
	e.f("ADDQ $4, AX")
	e.f("SUBQ $4, R11")
	e.label("tail2")
	e.f("TESTQ R11, R11")
	e.f("JZ next")
	e.f("VPINSRW $0, (R12), X0, X0")
	e.f("VPINSRW $0, (R13), X1, X1")
	e.f("VPAVGB X1, X0, X0")
	e.f("VPEXTRW $0, X0, (AX)")
	e.label("next")
	e.f("ADDQ R9, SI")
	e.f("ADDQ R10, BX")
	e.f("ADDQ R8, DI")
	e.f("DECQ DX")
	e.f("JNZ rows")
	e.f("VZEROUPPER")
	e.f("RET")
	e.raw("\n")
}

// ---- WeightUni, WeightBi ----

func neonWeightPrologueB(e *emitter, bi bool) {
	e.f("MOVD dst+0(FP), R0")
	e.f("MOVD dstStride+8(FP), R1")
	e.f("MOVD p+16(FP), R2")
	if bi {
		e.f("MOVD p1+24(FP), R10")
		e.f("MOVD pStride+32(FP), R3")
		e.f("MOVD w+40(FP), R4")
		e.f("MOVD h+48(FP), R5")
		e.f("MOVD q+56(FP), R6")
	} else {
		e.f("MOVD pStride+24(FP), R3")
		e.f("MOVD w+32(FP), R4")
		e.f("MOVD h+40(FP), R5")
		e.f("MOVD q+48(FP), R6")
	}
	e.f("LSL $2, R3, R3")
	for k := 0; k < 4; k++ {
		e.f("VLD1R (R6), [V%d.S4]", 16+k)
		if k < 3 {
			e.f("ADD $4, R6, R6")
		}
	}
	e.f("VEOR V25.B16, V25.B16, V25.B16")
}

// neonWeightB writes weightUniB<shift>NEON or weightBiB<shift>NEON, shift being log2Wd for the
// uni kernel and log2Wd + 1 for the bi one.
func neonWeightB(e *emitter, bi bool, shift int) {
	name, decl, frame := fmt.Sprintf("weightUniB%dNEON", shift), "dst *uint8, dstStride int, p *int32, pStride, w, h int, q *int32", 56
	if bi {
		name, decl, frame = fmt.Sprintf("weightBiB%dNEON", shift), "dst *uint8, dstStride int, p *int32, p1 *int32, pStride, w, h int, q *int32", 64
	}
	e.raw(fmt.Sprintf("// func %s(%s)\n", name, decl))
	e.raw(fmt.Sprintf("TEXT ·%s(SB), NOSPLIT, $0-%d\n", name, frame))
	neonWeightPrologueB(e, bi)
	e.label("rows")
	e.f("MOVD R2, R7")
	e.f("MOVD R0, R8")
	e.f("MOVD R4, R9")
	if bi {
		e.f("MOVD R10, R11")
	}
	group := func(cols int) {
		switch cols {
		case 8:
			e.f("VLD1.P 32(R7), [V0.S4, V1.S4]")
			if bi {
				e.f("VLD1.P 32(R11), [V2.S4, V3.S4]")
			}
		case 4:
			e.f("VLD1.P 16(R7), [V0.S4]")
			if bi {
				e.f("VLD1.P 16(R11), [V2.S4]")
			}
		default:
			e.f("FMOVD (R7), F0")
			e.f("ADD $8, R7, R7")
			if bi {
				e.f("FMOVD (R11), F2")
				e.f("ADD $8, R11, R11")
			}
		}
		vecs := 1
		if cols == 8 {
			vecs = 2
		}
		for v := 0; v < vecs; v++ {
			a, b := v, 2+v
			if bi {
				e.f("VMUL V16.S4, V%d.S4, V%d.S4", a, a)
				e.f("VMLA V17.S4, V%d.S4, V%d.S4", b, a)
				e.f("VADD V18.S4, V%d.S4, V%d.S4", a, a)
			} else {
				e.f("VMUL V16.S4, V%d.S4, V%d.S4", a, a)
				e.f("VADD V17.S4, V%d.S4, V%d.S4", a, a)
			}
			e.f("VSSHR $%d, V%d.S4, V%d.S4", shift, a, a)
			if !bi {
				e.f("VADD V18.S4, V%d.S4, V%d.S4", a, a)
			}
			e.f("VSMAX V25.S4, V%d.S4, V%d.S4", a, a)
			e.f("VSMIN V19.S4, V%d.S4, V%d.S4", a, a)
		}
		// the samples are within 0 .. 255 here: narrow twice, 32 to 16 to 8 bits
		e.f("VXTN V0.S4, V4.H4")
		if cols == 8 {
			e.f("VXTN2 V1.S4, V4.H8")
		}
		e.f("VXTN V4.H8, V5.B8")
		switch cols {
		case 8:
			e.f("VST1.P [V5.B8], 8(R8)")
		case 4:
			e.f("FMOVS F5, (R8)")
			e.f("ADD $4, R8, R8")
		default:
			e.f("VMOV V5.H[0], R12")
			e.f("MOVH R12, (R8)")
			e.f("ADD $2, R8, R8")
		}
		e.f("SUB $%d, R9, R9", cols)
	}
	e.label("cols8")
	e.f("CMP $8, R9")
	e.f("BLT tail4")
	group(8)
	e.f("B cols8")
	e.label("tail4")
	e.f("CMP $4, R9")
	e.f("BLT tail2")
	group(4)
	e.label("tail2")
	e.f("CBZ R9, next")
	group(2)
	e.label("next")
	e.f("ADD R3, R2, R2")
	if bi {
		e.f("ADD R3, R10, R10")
	}
	e.f("ADD R1, R0, R0")
	e.f("SUBS $1, R5, R5")
	e.f("BNE rows")
	e.f("RET")
	e.raw("\n")
}

// neonWeightBGo is the Go side of the arm64 byte weighting kernels.
func neonWeightBGo() string {
	out := "// Code generated by internal/genhevc; DO NOT EDIT.\n\n//go:build arm64 && !purego\n\npackage hevc\n\n"
	for s := 2; s <= 13; s++ {
		out += fmt.Sprintf("//go:noescape\nfunc weightUniB%dNEON(dst *uint8, dstStride int, p *int32, pStride, w, h int, q *int32)\n\n", s)
	}
	for s := 3; s <= 14; s++ {
		out += fmt.Sprintf("//go:noescape\nfunc weightBiB%dNEON(dst *uint8, dstStride int, p *int32, p1 *int32, pStride, w, h int, q *int32)\n\n", s)
	}
	out += "var weightUniBNEON = [16]func(dst *uint8, dstStride int, p *int32, pStride, w, h int, q *int32){\n"
	for s := 2; s <= 13; s++ {
		out += fmt.Sprintf("\t%d: weightUniB%dNEON,\n", s, s)
	}
	out += "}\n\nvar weightBiBNEON = [16]func(dst *uint8, dstStride int, p *int32, p1 *int32, pStride, w, h int, q *int32){\n"
	for s := 3; s <= 14; s++ {
		out += fmt.Sprintf("\t%d: weightBiB%dNEON,\n", s, s)
	}
	out += "}\n"
	return formatGo(out)
}

func avxWeightB(e *emitter, bi bool) {
	name, decl, frame := "weightUniBAVX2", "dst *uint8, dstStride int, p *int32, pStride, w, h int, q *int32, shift int", 64
	if bi {
		name, decl, frame = "weightBiBAVX2", "dst *uint8, dstStride int, p *int32, p1 *int32, pStride, w, h int, q *int32, shift int", 72
	}
	e.raw(fmt.Sprintf("// func %s(%s)\n", name, decl))
	e.raw(fmt.Sprintf("TEXT ·%s(SB), NOSPLIT, $0-%d\n", name, frame))
	e.f("MOVQ dst+0(FP), DI")
	e.f("MOVQ dstStride+8(FP), R8")
	e.f("MOVQ p+16(FP), SI")
	if bi {
		e.f("MOVQ p1+24(FP), BX")
		e.f("MOVQ pStride+32(FP), R9")
		e.f("MOVQ w+40(FP), CX")
		e.f("MOVQ h+48(FP), DX")
		e.f("MOVQ q+56(FP), AX")
		e.f("MOVQ shift+64(FP), R10")
	} else {
		e.f("MOVQ pStride+24(FP), R9")
		e.f("MOVQ w+32(FP), CX")
		e.f("MOVQ h+40(FP), DX")
		e.f("MOVQ q+48(FP), AX")
		e.f("MOVQ shift+56(FP), R10")
	}
	e.f("MOVQ R10, X14")
	e.f("SHLQ $2, R9")
	for k := 0; k < 4; k++ {
		e.f("VPBROADCASTD %d(AX), Y%d", 4*k, k)
	}
	e.f("VPXOR Y4, Y4, Y4")
	e.label("rows")
	e.f("MOVQ SI, R12")
	e.f("MOVQ DI, R13")
	e.f("MOVQ CX, R11")
	if bi {
		e.f("MOVQ BX, R10")
	}
	group := func(cols int) {
		reg := "Y"
		if cols < 8 {
			reg = "X"
		}
		switch cols {
		case 8, 4:
			e.f("VMOVDQU (R12), %s5", reg)
			if bi {
				e.f("VMOVDQU (R10), %s6", reg)
			}
		default:
			e.f("VMOVQ (R12), X5")
			if bi {
				e.f("VMOVQ (R10), X6")
			}
		}
		if bi {
			e.f("VPMULLD %s0, %s5, %s5", reg, reg, reg)
			e.f("VPMULLD %s1, %s6, %s6", reg, reg, reg)
			e.f("VPADDD %s6, %s5, %s5", reg, reg, reg)
			e.f("VPADDD %s2, %s5, %s5", reg, reg, reg)
			e.f("VPSRAD X14, %s5, %s5", reg, reg)
		} else {
			e.f("VPMULLD %s0, %s5, %s5", reg, reg, reg)
			e.f("VPADDD %s1, %s5, %s5", reg, reg, reg)
			e.f("VPSRAD X14, %s5, %s5", reg, reg)
			e.f("VPADDD %s2, %s5, %s5", reg, reg, reg)
		}
		e.f("VPMAXSD %s4, %s5, %s5", reg, reg, reg)
		e.f("VPMINSD %s3, %s5, %s5", reg, reg, reg)
		e.f("VPACKUSDW %s5, %s5, %s5", reg, reg, reg)
		if cols == 8 {
			e.f("VPERMQ $8, Y5, Y5")
		}
		e.f("VPACKUSWB X5, X5, X5")
		switch cols {
		case 8:
			e.f("VMOVQ X5, (R13)")
		case 4:
			e.f("VMOVD X5, (R13)")
		default:
			e.f("VPEXTRW $0, X5, (R13)")
		}
		e.f("ADDQ $%d, R12", 4*cols)
		if bi {
			e.f("ADDQ $%d, R10", 4*cols)
		}
		e.f("ADDQ $%d, R13", cols)
		e.f("SUBQ $%d, R11", cols)
	}
	e.label("cols8")
	e.f("CMPQ R11, $8")
	e.f("JLT tail4")
	group(8)
	e.f("JMP cols8")
	e.label("tail4")
	e.f("CMPQ R11, $4")
	e.f("JLT tail2")
	group(4)
	e.label("tail2")
	e.f("TESTQ R11, R11")
	e.f("JZ next")
	group(2)
	e.label("next")
	e.f("ADDQ R9, SI")
	if bi {
		e.f("ADDQ R9, BX")
	}
	e.f("ADDQ R8, DI")
	e.f("DECQ DX")
	e.f("JNZ rows")
	e.f("VZEROUPPER")
	e.f("RET")
	e.raw("\n")
}

// ---- SAOBand, SAOEdge ----

func neonSAOPrologueB(e *emitter, edge bool) {
	e.f("MOVD dst+0(FP), R0")
	e.f("MOVD dstStride+8(FP), R1")
	e.f("MOVD src+16(FP), R2")
	e.f("MOVD srcStride+24(FP), R3")
	e.f("MOVD w+32(FP), R4")
	e.f("MOVD h+40(FP), R5")
	if edge {
		e.f("MOVD offA+48(FP), R10")
		e.f("MOVD offB+56(FP), R11")
		e.f("MOVD p+64(FP), R6")
	} else {
		e.f("MOVD p+48(FP), R6")
	}
	for k := 0; k < 9; k++ {
		e.f("VLD1R (R6), [V%d.H8]", 16+k)
		e.f("ADD $2, R6, R6")
	}
	e.f("VEOR V25.B16, V25.B16, V25.B16")
}

// neonSAOBandB writes saoBand8BNEON: bit depth 8, the band is the sample >> 3; the width is a
// multiple of eight.
func neonSAOBandB(e *emitter) {
	e.raw("// func saoBand8BNEON(dst *uint8, dstStride int, src *uint8, srcStride, w, h int, p *int16)\n")
	e.raw("TEXT ·saoBand8BNEON(SB), NOSPLIT, $0-56\n")
	neonSAOPrologueB(e, false)
	e.label("rows")
	e.f("MOVD R2, R7")
	e.f("MOVD R0, R8")
	e.f("MOVD R4, R9")
	e.label("cols")
	e.f("VLD1.P 8(R7), [V0.B8]")
	e.f("VUXTL V0.B8, V0.H8")
	e.f("VUSHR $3, V0.H8, V1.H8")
	for k := 0; k < 4; k++ {
		e.f("VCMEQ V%d.H8, V1.H8, V%d.H8", 20+k, 2+k)
		e.f("VAND V%d.B16, V%d.B16, V%d.B16", 16+k, 2+k, 2+k)
	}
	e.f("VORR V2.B16, V3.B16, V2.B16")
	e.f("VORR V4.B16, V5.B16, V4.B16")
	e.f("VORR V2.B16, V4.B16, V2.B16")
	e.f("VADD V0.H8, V2.H8, V0.H8")
	e.f("VSMAX V25.H8, V0.H8, V0.H8")
	e.f("VSMIN V24.H8, V0.H8, V0.H8")
	e.f("VXTN V0.H8, V6.B8")
	e.f("VST1.P [V6.B8], 8(R8)")
	e.f("SUBS $8, R9, R9")
	e.f("BNE cols")
	e.f("ADD R3, R2, R2")
	e.f("ADD R1, R0, R0")
	e.f("SUBS $1, R5, R5")
	e.f("BNE rows")
	e.f("RET")
	e.raw("\n")
}

// neonSAOEdgeB writes saoEdge8BNEON: the neighbours are at the byte offsets offA and offB.
func neonSAOEdgeB(e *emitter) {
	e.raw("// func saoEdge8BNEON(dst *uint8, dstStride int, src *uint8, srcStride, w, h, offA, offB int, p *int16)\n")
	e.raw("TEXT ·saoEdge8BNEON(SB), NOSPLIT, $0-72\n")
	neonSAOPrologueB(e, true)
	e.label("rows")
	e.f("MOVD R2, R7")
	e.f("ADD R10, R2, R12")
	e.f("ADD R11, R2, R13")
	e.f("MOVD R0, R8")
	e.f("MOVD R4, R9")
	e.label("cols")
	e.f("VLD1.P 8(R7), [V0.B8]")
	e.f("VLD1.P 8(R12), [V1.B8]")
	e.f("VLD1.P 8(R13), [V2.B8]")
	e.f("VUXTL V0.B8, V0.H8")
	e.f("VUXTL V1.B8, V1.H8")
	e.f("VUXTL V2.B8, V2.H8")
	e.f("VCMGT V1.H8, V0.H8, V3.H8")
	e.f("VCMGT V0.H8, V1.H8, V4.H8")
	e.f("VCMGT V2.H8, V0.H8, V5.H8")
	e.f("VCMGT V0.H8, V2.H8, V6.H8")
	e.f("VADD V4.H8, V6.H8, V4.H8")
	e.f("VADD V3.H8, V5.H8, V3.H8")
	e.f("VSUB V3.H8, V4.H8, V4.H8")
	e.f("VCMEQ V20.H8, V4.H8, V5.H8")
	e.f("VAND V16.B16, V5.B16, V5.B16")
	e.f("VCMEQ V21.H8, V4.H8, V6.H8")
	e.f("VAND V17.B16, V6.B16, V6.B16")
	e.f("VCMEQ V22.H8, V4.H8, V7.H8")
	e.f("VAND V18.B16, V7.B16, V7.B16")
	e.f("VCMEQ V23.H8, V4.H8, V4.H8")
	e.f("VAND V19.B16, V4.B16, V4.B16")
	e.f("VORR V5.B16, V6.B16, V5.B16")
	e.f("VORR V7.B16, V4.B16, V7.B16")
	e.f("VORR V5.B16, V7.B16, V5.B16")
	e.f("VADD V0.H8, V5.H8, V0.H8")
	e.f("VSMAX V25.H8, V0.H8, V0.H8")
	e.f("VSMIN V24.H8, V0.H8, V0.H8")
	e.f("VXTN V0.H8, V6.B8")
	e.f("VST1.P [V6.B8], 8(R8)")
	e.f("SUBS $8, R9, R9")
	e.f("BNE cols")
	e.f("ADD R3, R2, R2")
	e.f("ADD R1, R0, R0")
	e.f("SUBS $1, R5, R5")
	e.f("BNE rows")
	e.f("RET")
	e.raw("\n")
}

func avxSAOPrologueB(e *emitter, edge bool) {
	e.f("MOVQ dst+0(FP), DI")
	e.f("MOVQ dstStride+8(FP), R8")
	e.f("MOVQ src+16(FP), SI")
	e.f("MOVQ srcStride+24(FP), R9")
	e.f("MOVQ w+32(FP), CX")
	e.f("MOVQ h+40(FP), DX")
	if edge {
		e.f("MOVQ offA+48(FP), R10")
		e.f("MOVQ offB+56(FP), BX")
		e.f("MOVQ p+64(FP), AX")
	} else {
		e.f("MOVQ $3, R10")
		e.f("MOVQ R10, X14")
		e.f("MOVQ p+48(FP), AX")
	}
	for k := 0; k < 9; k++ {
		e.f("VPBROADCASTW %d(AX), Y%d", 2*k, k)
	}
	e.f("VPXOR Y9, Y9, Y9")
}

// avxStoreB packs the 16 (Y) or 8 (X) clipped words of register 10 to bytes and stores them.
func avxStoreB(e *emitter, reg string) {
	e.f("VPACKUSWB %s10, %s10, %s10", reg, reg, reg)
	if reg == "Y" {
		e.f("VPERMQ $8, Y10, Y10")
		e.f("VMOVDQU X10, (R13)")
	} else {
		e.f("VMOVQ X10, (R13)")
	}
}

// avxSAOBandB writes saoBandBAVX2: groups of sixteen then one of eight.
func avxSAOBandB(e *emitter) {
	e.raw("// func saoBandBAVX2(dst *uint8, dstStride int, src *uint8, srcStride, w, h int, p *int16)\n")
	e.raw("TEXT ·saoBandBAVX2(SB), NOSPLIT, $0-56\n")
	avxSAOPrologueB(e, false)
	e.label("rows")
	e.f("MOVQ SI, R12")
	e.f("MOVQ DI, R13")
	e.f("MOVQ CX, R11")
	group := func(reg string, n int) {
		e.f("VPMOVZXBW (R12), %s10", reg)
		e.f("VPSRLW X14, %s10, %s11", reg, reg)
		for k := 0; k < 4; k++ {
			e.f("VPCMPEQW %s%d, %s11, %s12", reg, 4+k, reg, reg)
			e.f("VPAND %s%d, %s12, %s12", reg, k, reg, reg)
			if k == 0 {
				e.f("VMOVDQA %s12, %s13", reg, reg)
			} else {
				e.f("VPOR %s12, %s13, %s13", reg, reg, reg)
			}
		}
		e.f("VPADDW %s10, %s13, %s10", reg, reg, reg)
		e.f("VPMAXSW %s9, %s10, %s10", reg, reg, reg)
		e.f("VPMINSW %s8, %s10, %s10", reg, reg, reg)
		avxStoreB(e, reg)
		e.f("ADDQ $%d, R12", n)
		e.f("ADDQ $%d, R13", n)
		e.f("SUBQ $%d, R11", n)
	}
	e.label("cols16")
	e.f("CMPQ R11, $16")
	e.f("JLT cols8")
	group("Y", 16)
	e.f("JMP cols16")
	e.label("cols8")
	e.f("TESTQ R11, R11")
	e.f("JZ next")
	group("X", 8)
	e.label("next")
	e.f("ADDQ R9, SI")
	e.f("ADDQ R8, DI")
	e.f("DECQ DX")
	e.f("JNZ rows")
	e.f("VZEROUPPER")
	e.f("RET")
	e.raw("\n")
}

// avxSAOEdgeB writes saoEdgeBAVX2.
func avxSAOEdgeB(e *emitter) {
	e.raw("// func saoEdgeBAVX2(dst *uint8, dstStride int, src *uint8, srcStride, w, h, offA, offB int, p *int16)\n")
	e.raw("TEXT ·saoEdgeBAVX2(SB), NOSPLIT, $0-72\n")
	avxSAOPrologueB(e, true)
	e.label("rows")
	e.f("MOVQ SI, R12")
	e.f("MOVQ DI, R13")
	e.f("MOVQ CX, R11")
	group := func(reg string, n int) {
		e.f("VPMOVZXBW (R12), %s10", reg)
		e.f("VPMOVZXBW (R12)(R10*1), %s11", reg)
		e.f("VPMOVZXBW (R12)(BX*1), %s12", reg)
		e.f("VPCMPGTW %s11, %s10, %s13", reg, reg, reg) // cur > a
		e.f("VPCMPGTW %s10, %s11, %s11", reg, reg, reg) // a > cur
		e.f("VPCMPGTW %s12, %s10, %s14", reg, reg, reg) // cur > b
		e.f("VPCMPGTW %s10, %s12, %s12", reg, reg, reg) // b > cur
		e.f("VPADDW %s12, %s11, %s11", reg, reg, reg)
		e.f("VPADDW %s14, %s13, %s13", reg, reg, reg)
		e.f("VPSUBW %s13, %s11, %s11", reg, reg, reg) // e = lt - gt
		for k := 0; k < 4; k++ {
			e.f("VPCMPEQW %s%d, %s11, %s12", reg, 4+k, reg, reg)
			e.f("VPAND %s%d, %s12, %s12", reg, k, reg, reg)
			if k == 0 {
				e.f("VMOVDQA %s12, %s15", reg, reg)
			} else {
				e.f("VPOR %s12, %s15, %s15", reg, reg, reg)
			}
		}
		e.f("VPADDW %s10, %s15, %s10", reg, reg, reg)
		e.f("VPMAXSW %s9, %s10, %s10", reg, reg, reg)
		e.f("VPMINSW %s8, %s10, %s10", reg, reg, reg)
		avxStoreB(e, reg)
		e.f("ADDQ $%d, R12", n)
		e.f("ADDQ $%d, R13", n)
		e.f("SUBQ $%d, R11", n)
	}
	e.label("cols16")
	e.f("CMPQ R11, $16")
	e.f("JLT cols8")
	group("Y", 16)
	e.f("JMP cols16")
	e.label("cols8")
	e.f("TESTQ R11, R11")
	e.f("JZ next")
	group("X", 8)
	e.label("next")
	e.f("ADDQ R9, SI")
	e.f("ADDQ R8, DI")
	e.f("DECQ DX")
	e.f("JNZ rows")
	e.f("VZEROUPPER")
	e.f("RET")
	e.raw("\n")
}
