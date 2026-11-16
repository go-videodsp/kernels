package main

import "fmt"

// The sample adaptive offset kernels (ITU-T H.265 (V11) (01/2026), clause 8.7.3): band offset
// (8-414, Table 8-14 style lookup of the four bands from bandPos) and edge offset (8-411,
// 8-412). Both work on groups of eight (NEON) or sixteen and eight (AVX2) uint16 samples, and
// read nothing but the samples of their block and the neighbours the edge class names.
//
// A block's parameters are a [12]int16 array, p[0..3] the four offsets, p[4..7] the four
// band numbers (band offset) or the edge values -2, -1, 1, 2 (edge offset), p[8] the largest
// sample value; the offset of an edge value is p[0] for -2, p[1] for -1, p[2] for 1 and p[3]
// for 2 (the value 0 has none).

// ---- arm64 ----

func neonSAOPrologue(e *emitter, edge bool) {
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
		e.f("LSL $1, R10, R10")
		e.f("LSL $1, R11, R11")
	} else {
		e.f("MOVD p+48(FP), R6")
	}
	e.f("LSL $1, R1, R1")
	e.f("LSL $1, R3, R3")
	// V16..V19 the offsets, V20..V23 p[4..7], V24 the maximum, V25 zero
	for k := 0; k < 9; k++ {
		reg := 16 + k
		e.f("VLD1R (R6), [V%d.H8]", reg)
		e.f("ADD $2, R6, R6")
	}
	e.f("VEOR V25.B16, V25.B16, V25.B16")
}

// neonSAOBand writes saoBand8NEON<shift>: a block whose width is a multiple of eight.
func neonSAOBand(e *emitter, shift int) {
	e.raw(fmt.Sprintf("// func saoBand8NEON%d(dst *uint16, dstStride int, src *uint16, srcStride, w, h int, p *int16)\n", shift))
	e.raw(fmt.Sprintf("TEXT ·saoBand8NEON%d(SB), NOSPLIT, $0-56\n", shift))
	neonSAOPrologue(e, false)
	e.label("rows")
	e.f("MOVD R2, R7")
	e.f("MOVD R0, R8")
	e.f("MOVD R4, R9")
	e.label("cols")
	e.f("VLD1.P 16(R7), [V0.H8]")
	e.f("VUSHR $%d, V0.H8, V1.H8", shift)
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
	e.f("VST1.P [V0.H8], 16(R8)")
	e.f("SUBS $8, R9, R9")
	e.f("BNE cols")
	e.f("ADD R3, R2, R2")
	e.f("ADD R1, R0, R0")
	e.f("SUBS $1, R5, R5")
	e.f("BNE rows")
	e.f("RET")
	e.raw("\n")
}

// neonSAOEdge writes saoEdge8NEON: the neighbours are at the element offsets offA and offB
// from the sample.
func neonSAOEdge(e *emitter) {
	e.raw("// func saoEdge8NEON(dst *uint16, dstStride int, src *uint16, srcStride, w, h, offA, offB int, p *int16)\n")
	e.raw("TEXT ·saoEdge8NEON(SB), NOSPLIT, $0-72\n")
	neonSAOPrologue(e, true)
	e.label("rows")
	e.f("MOVD R2, R7")
	e.f("ADD R10, R2, R12")
	e.f("ADD R11, R2, R13")
	e.f("MOVD R0, R8")
	e.f("MOVD R4, R9")
	e.label("cols")
	e.f("VLD1.P 16(R7), [V0.H8]")
	e.f("VLD1.P 16(R12), [V1.H8]")
	e.f("VLD1.P 16(R13), [V2.H8]")
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
	e.f("VST1.P [V0.H8], 16(R8)")
	e.f("SUBS $8, R9, R9")
	e.f("BNE cols")
	e.f("ADD R3, R2, R2")
	e.f("ADD R1, R0, R0")
	e.f("SUBS $1, R5, R5")
	e.f("BNE rows")
	e.f("RET")
	e.raw("\n")
}

// ---- amd64 ----

func avxSAOPrologue(e *emitter, edge bool) {
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
		e.f("SHLQ $1, R10")
		e.f("SHLQ $1, BX")
	} else {
		e.f("MOVQ shift+56(FP), R10")
		e.f("MOVQ R10, X14")
		e.f("MOVQ p+48(FP), AX")
	}
	e.f("SHLQ $1, R8")
	e.f("SHLQ $1, R9")
	// Y0..Y3 the offsets, Y4..Y7 p[4..7], Y8 the maximum, Y9 zero
	for k := 0; k < 9; k++ {
		e.f("VPBROADCASTW %d(AX), Y%d", 2*k, k)
	}
	e.f("VPXOR Y9, Y9, Y9")
}

// avxSAOBand writes saoBand16AVX2: groups of sixteen then one of eight; the width is a
// multiple of eight.
func avxSAOBand(e *emitter) {
	e.raw("// func saoBandAVX2(dst *uint16, dstStride int, src *uint16, srcStride, w, h int, p *int16, shift int)\n")
	e.raw("TEXT ·saoBandAVX2(SB), NOSPLIT, $0-64\n")
	avxSAOPrologue(e, false)
	e.label("rows")
	e.f("MOVQ SI, R12")
	e.f("MOVQ DI, R13")
	e.f("MOVQ CX, R11")
	group := func(reg string, bytes int, n int) {
		e.f("VMOVDQU (R12), %s10", reg)
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
		e.f("VMOVDQU %s10, (R13)", reg)
		e.f("ADDQ $%d, R12", bytes)
		e.f("ADDQ $%d, R13", bytes)
		e.f("SUBQ $%d, R11", n)
	}
	e.label("cols16")
	e.f("CMPQ R11, $16")
	e.f("JLT cols8")
	group("Y", 32, 16)
	e.f("JMP cols16")
	e.label("cols8")
	e.f("TESTQ R11, R11")
	e.f("JZ next")
	group("X", 16, 8)
	e.label("next")
	e.f("ADDQ R9, SI")
	e.f("ADDQ R8, DI")
	e.f("DECQ DX")
	e.f("JNZ rows")
	e.f("VZEROUPPER")
	e.f("RET")
	e.raw("\n")
}

// avxSAOEdge writes saoEdgeAVX2.
func avxSAOEdge(e *emitter) {
	e.raw("// func saoEdgeAVX2(dst *uint16, dstStride int, src *uint16, srcStride, w, h, offA, offB int, p *int16)\n")
	e.raw("TEXT ·saoEdgeAVX2(SB), NOSPLIT, $0-72\n")
	avxSAOPrologue(e, true)
	e.label("rows")
	e.f("MOVQ SI, R12")
	e.f("MOVQ DI, R13")
	e.f("MOVQ CX, R11")
	group := func(reg string, bytes int, n int) {
		e.f("VMOVDQU (R12), %s10", reg)
		e.f("VMOVDQU (R12)(R10*1), %s11", reg)
		e.f("VMOVDQU (R12)(BX*1), %s12", reg)
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
		e.f("VMOVDQU %s10, (R13)", reg)
		e.f("ADDQ $%d, R12", bytes)
		e.f("ADDQ $%d, R13", bytes)
		e.f("SUBQ $%d, R11", n)
	}
	e.label("cols16")
	e.f("CMPQ R11, $16")
	e.f("JLT cols8")
	group("Y", 32, 16)
	e.f("JMP cols16")
	e.label("cols8")
	e.f("TESTQ R11, R11")
	e.f("JZ next")
	group("X", 16, 8)
	e.label("next")
	e.f("ADDQ R9, SI")
	e.f("ADDQ R8, DI")
	e.f("DECQ DX")
	e.f("JNZ rows")
	e.f("VZEROUPPER")
	e.f("RET")
	e.raw("\n")
}
