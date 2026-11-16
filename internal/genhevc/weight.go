package main

import (
	"fmt"
	"go/format"
)

// The weighted sample prediction kernels of HEVC (ITU-T H.265 (V11) (01/2026), clause
// 8.5.3.3.4.3, Equations 8-275 and 8-277; the default process of 8.5.3.3.4.2 is the same
// with weights 1, offsets 0 and log2Wd = Max( 2, 14 - bitDepth )): from the 32-bit
// intermediates of the interpolation to the clipped samples of the picture.
//
//	uni: dst = Clip( ( ( p * w + 2^(log2Wd - 1) ) >> log2Wd ) + o )
//	bi:  dst = Clip( ( p0 * w0 + p1 * w1 + c ) >> (log2Wd + 1) ),  c = ( o0 + o1 + 1 ) << log2Wd
//
// A block's parameters are an int32 array p[0] w (or w0), p[1] rounding (or w1), p[2] o (or
// c), p[3] the largest sample value.

// ---- arm64: one kernel per shift (the shift is an immediate) ----

func neonWeightPrologue(e *emitter, bi bool) {
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
	e.f("LSL $1, R1, R1")
	e.f("LSL $2, R3, R3")
	for k := 0; k < 4; k++ {
		e.f("VLD1R (R6), [V%d.S4]", 16+k)
		if k < 3 {
			e.f("ADD $4, R6, R6")
		}
	}
	e.f("VEOR V25.B16, V25.B16, V25.B16")
}

// neonWeight writes weightUni<shift>NEON or weightBi<shift>NEON, shift being log2Wd for the
// uni kernel and log2Wd + 1 for the bi one.
func neonWeight(e *emitter, bi bool, shift int) {
	name, decl, frame := fmt.Sprintf("weightUni%dNEON", shift), "dst *uint16, dstStride int, p *int32, pStride, w, h int, q *int32", 56
	if bi {
		name, decl, frame = fmt.Sprintf("weightBi%dNEON", shift), "dst *uint16, dstStride int, p *int32, p1 *int32, pStride, w, h int, q *int32", 64
	}
	e.raw(fmt.Sprintf("// func %s(%s)\n", name, decl))
	e.raw(fmt.Sprintf("TEXT ·%s(SB), NOSPLIT, $0-%d\n", name, frame))
	neonWeightPrologue(e, bi)
	e.label("rows")
	e.f("MOVD R2, R7")
	e.f("MOVD R0, R8")
	e.f("MOVD R4, R9")
	if bi {
		e.f("MOVD R10, R11")
	}
	group := func(cols int) {
		n := cols / 4 // vectors (cols is 8, 4 or 2: two lanes of a vector for 2)
		if cols == 2 {
			n = 1
		}
		for v := 0; v < n; v++ {
			switch cols {
			case 8:
				if v == 0 {
					e.f("VLD1.P 32(R7), [V0.S4, V1.S4]")
					if bi {
						e.f("VLD1.P 32(R11), [V2.S4, V3.S4]")
					}
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
		switch cols {
		case 8:
			e.f("VXTN V0.S4, V4.H4")
			e.f("VXTN2 V1.S4, V4.H8")
			e.f("VST1.P [V4.H8], 16(R8)")
		case 4:
			e.f("VXTN V0.S4, V4.H4")
			e.f("VST1.P [V4.H4], 8(R8)")
		default:
			e.f("VXTN V0.S4, V4.H4")
			e.f("FMOVS F4, (R8)")
			e.f("ADD $4, R8, R8")
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

// ---- amd64: the shift is a count in X14 ----

func avxWeight(e *emitter, bi bool) {
	name, decl, frame := "weightUniAVX2", "dst *uint16, dstStride int, p *int32, pStride, w, h int, q *int32, shift int", 64
	if bi {
		name, decl, frame = "weightBiAVX2", "dst *uint16, dstStride int, p *int32, p1 *int32, pStride, w, h int, q *int32, shift int", 72
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
	e.f("SHLQ $1, R8")
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
		switch cols {
		case 8:
			e.f("VPERMQ $8, Y5, Y5")
			e.f("VMOVDQU X5, (R13)")
		case 4:
			e.f("VMOVQ X5, (R13)")
		default:
			e.f("VMOVD X5, (R13)")
		}
		e.f("ADDQ $%d, R12", 4*cols)
		if bi {
			e.f("ADDQ $%d, R10", 4*cols)
		}
		e.f("ADDQ $%d, R13", 2*cols)
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

// neonWeightGo is the Go side of the arm64 weighting kernels: their declarations and the
// tables that pick one by shift.
func neonWeightGo() string {
	out := "// Code generated by internal/genhevc; DO NOT EDIT.\n\n//go:build arm64 && !purego\n\npackage hevc\n\n"
	for s := 2; s <= 13; s++ {
		out += fmt.Sprintf("//go:noescape\nfunc weightUni%dNEON(dst *uint16, dstStride int, p *int32, pStride, w, h int, q *int32)\n\n", s)
	}
	for s := 3; s <= 14; s++ {
		out += fmt.Sprintf("//go:noescape\nfunc weightBi%dNEON(dst *uint16, dstStride int, p *int32, p1 *int32, pStride, w, h int, q *int32)\n\n", s)
	}
	out += "var weightUniNEON = [16]func(dst *uint16, dstStride int, p *int32, pStride, w, h int, q *int32){\n"
	for s := 2; s <= 13; s++ {
		out += fmt.Sprintf("\t%d: weightUni%dNEON,\n", s, s)
	}
	out += "}\n\nvar weightBiNEON = [16]func(dst *uint16, dstStride int, p *int32, p1 *int32, pStride, w, h int, q *int32){\n"
	for s := 3; s <= 14; s++ {
		out += fmt.Sprintf("\t%d: weightBi%dNEON,\n", s, s)
	}
	out += "}\n"
	formatted, _ := format.Source([]byte(out)) // an error leaves it empty, which the up-to-date test catches
	return string(formatted)
}
