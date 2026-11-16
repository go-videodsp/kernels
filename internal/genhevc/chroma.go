package main

import "fmt"

// The four-tap chroma passes (ITU-T H.265 (V11) (01/2026), clause 8.5.3.3.3.3, Equations
// 8-241 to 8-261): the same loop nests as the luma ones with four taps, and a group of two
// columns after the groups of eight and four (chroma widths are even, not multiples of 4).

// ---- arm64 ----

func neonPrologue4(e *emitter, dstShift, srcShift int) {
	e.f("MOVD dst+0(FP), R0")
	e.f("MOVD dstStride+8(FP), R1")
	e.f("MOVD src+16(FP), R2")
	e.f("MOVD srcStride+24(FP), R3")
	e.f("MOVD w+32(FP), R4")
	e.f("MOVD h+40(FP), R5")
	e.f("MOVD c+48(FP), R6")
	e.f("LSL $%d, R1, R1", dstShift)
	e.f("LSL $%d, R3, R3", srcShift)
	for k := 0; k < 4; k++ {
		e.f("VLD1R (R6), [V%d.H8]", 16+k)
		if k < 3 {
			e.f("ADD $2, R6, R6")
		}
	}
}

// neonPass4 writes hpass4NEON<shift> (horizontal: taps 2 bytes apart, uint16 to int16) or vpass4NEON
// (vertical: taps one source row apart, int16 to int32, shift 6).
func neonPass4(e *emitter, vertical bool, shift int) {
	name, dstShift, step, outBytes := fmt.Sprintf("hpass4NEON%d", shift), 1, "ADD $2, R10, R10", 2
	decl := "dst *int16, dstStride int, src *uint16, srcStride, w, h int, c *int16"
	if vertical {
		name, dstShift, step, outBytes = "vpass4NEON", 2, "ADD R3, R10, R10", 4
		decl = "dst *int32, dstStride int, src *int16, srcStride, w, h int, c *int16"
		shift = 6
	}
	e.raw(fmt.Sprintf("// func %s(%s)\n", name, decl))
	e.raw(fmt.Sprintf("TEXT ·%s(SB), NOSPLIT, $0-56\n", name))
	neonPrologue4(e, dstShift, 1)
	e.label("rows")
	e.f("MOVD R2, R7")
	e.f("MOVD R0, R8")
	e.f("MOVD R4, R9")
	// group: the loads, the multiply-accumulates, the shift, the narrowing and the store
	group := func(lbl string, cols int) {
		half := cols == 8
		e.f("VEOR V0.B16, V0.B16, V0.B16")
		if half {
			e.f("VEOR V1.B16, V1.B16, V1.B16")
		}
		e.f("MOVD R7, R10")
		for k := 0; k < 4; k++ {
			switch cols {
			case 8:
				e.f("VLD1 (R10), [V2.H8]")
			case 4:
				e.f("VLD1 (R10), [V2.H4]")
			default:
				e.f("FMOVS (R10), F2")
			}
			e.f("VSMLAL V%d.H4, V2.H4, V0.S4", 16+k)
			if half {
				e.f("VSMLAL2 V%d.H8, V2.H8, V1.S4", 16+k)
			}
			if k < 3 {
				e.f("%s", step)
			}
		}
		if shift > 0 {
			e.f("VSSHR $%d, V0.S4, V0.S4", shift)
			if half {
				e.f("VSSHR $%d, V1.S4, V1.S4", shift)
			}
		}
		if !vertical {
			e.f("VXTN V0.S4, V3.H4")
			if half {
				e.f("VXTN2 V1.S4, V3.H8")
			}
		}
		switch {
		case !vertical && cols == 8:
			e.f("VST1 [V3.H8], (R8)")
		case !vertical && cols == 4:
			e.f("VST1 [V3.H4], (R8)")
		case !vertical:
			e.f("FMOVS F3, (R8)")
		case cols == 8:
			e.f("VST1 [V0.S4, V1.S4], (R8)")
		case cols == 4:
			e.f("VST1 [V0.S4], (R8)")
		default:
			e.f("FMOVD F0, (R8)")
		}
		e.f("ADD $%d, R7, R7", 2*cols)
		e.f("ADD $%d, R8, R8", outBytes*cols)
		e.f("SUB $%d, R9, R9", cols)
	}
	e.label("cols8")
	e.f("CMP $8, R9")
	e.f("BLT tail4")
	group("", 8)
	e.f("B cols8")
	e.label("tail4")
	e.f("CMP $4, R9")
	e.f("BLT tail2")
	group("", 4)
	e.label("tail2")
	e.f("CBZ R9, next")
	group("", 2)
	e.label("next")
	e.f("ADD R3, R2, R2")
	e.f("ADD R1, R0, R0")
	e.f("SUBS $1, R5, R5")
	e.f("BNE rows")
	e.f("RET")
	e.raw("\n")
}

// ---- amd64 ----

func avxPrologue4(e *emitter, dstShift, srcShift int) {
	e.f("MOVQ dst+0(FP), DI")
	e.f("MOVQ dstStride+8(FP), R8")
	e.f("MOVQ src+16(FP), SI")
	e.f("MOVQ srcStride+24(FP), R9")
	e.f("MOVQ w+32(FP), CX")
	e.f("MOVQ h+40(FP), DX")
	e.f("MOVQ c+48(FP), AX")
	e.f("SHLQ $%d, R8", dstShift)
	e.f("SHLQ $%d, R9", srcShift)
	for k := 0; k < 4; k++ {
		e.f("MOVWQSX %d(AX), R10", 2*k)
		e.f("MOVQ R10, X%d", k)
		e.f("VPBROADCASTD X%d, Y%d", k, k)
	}
}

// avxPass4 writes hpass4AVX2 (the shift is an argument) or vpass4AVX2.
func avxPass4(e *emitter, vertical bool) {
	name, dstShift, step := "hpass4AVX2", 1, "ADDQ $2, R10"
	decl := "dst *int16, dstStride int, src *uint16, srcStride, w, h int, c *int16, shift int"
	frame, widen := 64, "VPMOVZXWD"
	if vertical {
		name, dstShift, step = "vpass4AVX2", 2, "ADDQ R9, R10"
		decl = "dst *int32, dstStride int, src *int16, srcStride, w, h int, c *int16"
		frame, widen = 56, "VPMOVSXWD"
	}
	e.raw(fmt.Sprintf("// func %s(%s)\n", name, decl))
	e.raw(fmt.Sprintf("TEXT ·%s(SB), NOSPLIT, $0-%d\n", name, frame))
	avxPrologue4(e, dstShift, 1)
	if !vertical {
		e.f("MOVQ shift+56(FP), R10")
		e.f("MOVQ R10, X14")
		e.f("VMOVDQU shufmask<>(SB), Y10")
	}
	e.label("rows")
	e.f("MOVQ SI, R12")
	e.f("MOVQ DI, R13")
	e.f("MOVQ CX, R11")
	group := func(cols int) {
		reg, acc := "Y", "Y8"
		if cols < 8 {
			reg, acc = "X", "X8"
		}
		e.f("VPXOR %s, %s, %s", acc, acc, acc)
		e.f("MOVQ R12, R10")
		for k := 0; k < 4; k++ {
			switch cols {
			case 8, 4:
				e.f("%s (R10), %s9", widen, reg)
			default:
				e.f("VMOVD (R10), X9")
				e.f("%s X9, X9", widen)
			}
			e.f("VPMULLD %s%d, %s9, %s9", reg, k, reg, reg)
			e.f("VPADDD %s9, %s, %s", reg, acc, acc)
			if k < 3 {
				e.f("%s", step)
			}
		}
		if vertical {
			e.f("VPSRAD $6, %s, %s", acc, acc)
		} else {
			e.f("VPSRAD X14, %s, %s", acc, acc)
			e.f("VPSHUFB %s10, %s, %s", reg, acc, acc)
			if cols == 8 {
				e.f("VPERMQ $8, Y8, Y8")
			}
		}
		out := 2
		if vertical {
			out = 4
		}
		switch {
		case cols == 8 && vertical:
			e.f("VMOVDQU Y8, (R13)")
		case cols == 8, cols == 4 && vertical:
			e.f("VMOVDQU X8, (R13)")
		case cols == 4, cols == 2 && vertical:
			e.f("VMOVQ X8, (R13)")
		default:
			e.f("VMOVD X8, (R13)")
		}
		e.f("ADDQ $%d, R12", 2*cols)
		e.f("ADDQ $%d, R13", out*cols)
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
	e.f("ADDQ R8, DI")
	e.f("DECQ DX")
	e.f("JNZ rows")
	e.f("VZEROUPPER")
	e.f("RET")
	e.raw("\n")
}
