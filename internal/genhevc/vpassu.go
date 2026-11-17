package main

import (
	"fmt"
	"go/format"
)

// The one-dimensional filter over the samples of a picture straight into int32, for a block
// whose fractional position is in one direction only (ITU-T H.265 (V11) (01/2026), clauses
// 8.5.3.3.3.2 and 8.5.3.3.3.3, Equations 8-228 and 8-244 and their chroma twins): the sum of the
// taps times the samples one row apart (vertical: vpassU) or one column apart (horizontal:
// hpassW), shifted right by shift1 = bitDepth - 8. It is the passes of the two-dimensional
// filter with uint16 samples and int32 results, the shift an argument, so the loop nests are
// neonV and avxV's (and neonH and avxH's, without the narrowing to int16).
//
// The samples are uint16 (pictures above 8 bits) or uint8 (8-bit pictures, the "bytes" forms,
// names ending in B): a byte kernel loads bytes and widens them in registers, reading exactly
// the samples of the footprint, the stride in bytes.

// filterName is vpassU<taps> (vertical) or hpassW<taps> (horizontal), with a B for a byte source.
func filterName(ntaps int, horiz, bytes bool) string {
	n := fmt.Sprintf("vpassU%d", ntaps)
	if horiz {
		n = fmt.Sprintf("hpassW%d", ntaps)
	}
	if bytes {
		n += "B"
	}
	return n
}

// elem is the Go type of a source sample and its size in bytes.
func elem(bytes bool) (string, int) {
	if bytes {
		return "uint8", 1
	}
	return "uint16", 2
}

// neonLoadBytes loads cols bytes from (reg) into the low half-words of V2, widened to 16 bits:
// 8 bytes by one load, 4 by a single-precision load, 2 through a general register; the bytes
// above stay zero, so the 4-lane multiplies see zeros there.
func neonLoadBytes(e *emitter, cols int, reg string) {
	switch cols {
	case 8:
		e.f("VLD1 (%s), [V2.B8]", reg)
	case 4:
		e.f("FMOVS (%s), F2", reg)
	default:
		e.f("MOVHU (%s), R14", reg)
		e.f("FMOVS R14, F2")
	}
	e.f("VUXTL V2.B8, V2.H8")
}

// neonVU writes vpassU<taps>NEON<shift> or hpassW<taps>NEON<shift> (with B for bytes): taps 8
// (luma) or 4 (chroma, widths even).
func neonVU(e *emitter, ntaps, shift int, horiz, bytes bool) {
	name := fmt.Sprintf("%sNEON%d", filterName(ntaps, horiz, bytes), shift)
	typ, size := elem(bytes)
	step := "ADD R3, R10, R10"
	if horiz {
		step = fmt.Sprintf("ADD $%d, R10, R10", size)
	}
	e.raw(fmt.Sprintf("// func %s(dst *int32, dstStride int, src *%s, srcStride, w, h int, c *int16)\n", name, typ))
	e.raw(fmt.Sprintf("TEXT ·%s(SB), NOSPLIT, $0-56\n", name))
	e.f("MOVD dst+0(FP), R0")
	e.f("MOVD dstStride+8(FP), R1")
	e.f("MOVD src+16(FP), R2")
	e.f("MOVD srcStride+24(FP), R3")
	e.f("MOVD w+32(FP), R4")
	e.f("MOVD h+40(FP), R5")
	e.f("MOVD c+48(FP), R6")
	e.f("LSL $2, R1, R1")
	if !bytes {
		e.f("LSL $1, R3, R3")
	}
	for k := 0; k < ntaps; k++ {
		e.f("VLD1R (R6), [V%d.H8]", 16+k)
		if k < ntaps-1 {
			e.f("ADD $2, R6, R6")
		}
	}
	e.label("rows")
	e.f("MOVD R2, R7")
	e.f("MOVD R0, R8")
	e.f("MOVD R4, R9")
	group := func(cols int) {
		half := cols == 8
		e.f("VEOR V0.B16, V0.B16, V0.B16")
		if half {
			e.f("VEOR V1.B16, V1.B16, V1.B16")
		}
		e.f("MOVD R7, R10")
		for k := 0; k < ntaps; k++ {
			if bytes {
				neonLoadBytes(e, cols, "R10")
			} else {
				switch cols {
				case 8:
					e.f("VLD1 (R10), [V2.H8]")
				case 4:
					e.f("VLD1 (R10), [V2.H4]")
				default:
					e.f("FMOVS (R10), F2")
				}
			}
			e.f("VSMLAL V%d.H4, V2.H4, V0.S4", 16+k)
			if half {
				e.f("VSMLAL2 V%d.H8, V2.H8, V1.S4", 16+k)
			}
			if k < ntaps-1 {
				e.f("%s", step)
			}
		}
		if shift > 0 {
			e.f("VSSHR $%d, V0.S4, V0.S4", shift)
			if half {
				e.f("VSSHR $%d, V1.S4, V1.S4", shift)
			}
		}
		switch cols {
		case 8:
			e.f("VST1 [V0.S4, V1.S4], (R8)")
		case 4:
			e.f("VST1 [V0.S4], (R8)")
		default:
			e.f("FMOVD F0, (R8)")
		}
		e.f("ADD $%d, R7, R7", size*cols)
		e.f("ADD $%d, R8, R8", 4*cols)
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
	e.f("ADD R1, R0, R0")
	e.f("SUBS $1, R5, R5")
	e.f("BNE rows")
	e.f("RET")
	e.raw("\n")
}

// neonWiden is widenNEON (widenBNEON for bytes): dst[y*dstStride+x] = int32(src[y*srcStride+x])
// << shift, the shift in a register (a left shift by a positive count).
func neonWiden(e *emitter, bytes bool) {
	name := "widenNEON"
	if bytes {
		name = "widenBNEON"
	}
	typ, _ := elem(bytes)
	e.raw(fmt.Sprintf("// func %s(dst *int32, dstStride int, src *%s, srcStride, w, h, shift int)\n", name, typ))
	e.raw(fmt.Sprintf("TEXT ·%s(SB), NOSPLIT, $0-56\n", name))
	e.f("MOVD dst+0(FP), R0")
	e.f("MOVD dstStride+8(FP), R1")
	e.f("MOVD src+16(FP), R2")
	e.f("MOVD srcStride+24(FP), R3")
	e.f("MOVD w+32(FP), R4")
	e.f("MOVD h+40(FP), R5")
	e.f("MOVD shift+48(FP), R6")
	e.f("LSL $2, R1, R1")
	if !bytes {
		e.f("LSL $1, R3, R3")
	}
	e.f("VDUP R6, V20.S4")
	e.label("rows")
	e.f("MOVD R2, R7")
	e.f("MOVD R0, R8")
	e.f("MOVD R4, R9")
	e.label("cols8")
	e.f("CMP $8, R9")
	e.f("BLT tail4")
	if bytes {
		neonLoadBytes(e, 8, "R7")
		e.f("ADD $8, R7, R7")
	} else {
		e.f("VLD1.P 16(R7), [V2.H8]")
	}
	e.f("VUXTL V2.H4, V0.S4")
	e.f("VUXTL2 V2.H8, V1.S4")
	e.f("VSSHL V20.S4, V0.S4, V0.S4")
	e.f("VSSHL V20.S4, V1.S4, V1.S4")
	e.f("VST1.P [V0.S4, V1.S4], 32(R8)")
	e.f("SUB $8, R9, R9")
	e.f("B cols8")
	e.label("tail4")
	e.f("CMP $4, R9")
	e.f("BLT tail2")
	if bytes {
		neonLoadBytes(e, 4, "R7")
		e.f("ADD $4, R7, R7")
	} else {
		e.f("VLD1.P 8(R7), [V2.H4]")
	}
	e.f("VUXTL V2.H4, V0.S4")
	e.f("VSSHL V20.S4, V0.S4, V0.S4")
	e.f("VST1.P [V0.S4], 16(R8)")
	e.f("SUB $4, R9, R9")
	e.label("tail2")
	e.f("CBZ R9, next")
	if bytes {
		neonLoadBytes(e, 2, "R7")
	} else {
		e.f("FMOVS (R7), F2")
	}
	e.f("VUXTL V2.H4, V0.S4")
	e.f("VSSHL V20.S4, V0.S4, V0.S4")
	e.f("FMOVD F0, (R8)")
	e.label("next")
	e.f("ADD R3, R2, R2")
	e.f("ADD R1, R0, R0")
	e.f("SUBS $1, R5, R5")
	e.f("BNE rows")
	e.f("RET")
	e.raw("\n")
}

// neonVUGo is the Go side: the declarations and the tables that pick a kernel by shift.
func neonVUGo() string {
	out := "// Code generated by internal/genhevc; DO NOT EDIT.\n\n//go:build arm64 && !purego\n\npackage hevc\n\n"
	for _, bytes := range []bool{false, true} {
		typ, _ := elem(bytes)
		for _, horiz := range []bool{false, true} {
			for _, n := range []int{8, 4} {
				base := filterName(n, horiz, bytes)
				for s := 0; s <= 4; s++ {
					out += fmt.Sprintf("//go:noescape\nfunc %sNEON%d(dst *int32, dstStride int, src *%s, srcStride, w, h int, c *int16)\n\n", base, s, typ)
				}
				out += fmt.Sprintf("var %sNEON = [5]func(dst *int32, dstStride int, src *%s, srcStride, w, h int, c *int16){\n", base, typ)
				for s := 0; s <= 4; s++ {
					out += fmt.Sprintf("\t%d: %sNEON%d,\n", s, base, s)
				}
				out += "}\n\n"
			}
		}
	}
	out += "//go:noescape\nfunc widenNEON(dst *int32, dstStride int, src *uint16, srcStride, w, h, shift int)\n\n"
	out += "//go:noescape\nfunc widenBNEON(dst *int32, dstStride int, src *uint8, srcStride, w, h, shift int)\n"
	formatted, _ := format.Source([]byte(out)) // an error leaves it empty, which the up-to-date test catches
	return string(formatted)
}

// avxLoadBytes loads cols bytes from (reg) zero-extended to dwords into acc9's register (Y9 for 8
// columns, X9 below): 8 and 4 bytes by one extending load, 2 through a general register so that
// no byte beyond the footprint is read.
func avxLoadBytes(e *emitter, cols int, reg, dst string) {
	switch cols {
	case 8:
		e.f("VPMOVZXBD (%s), Y%s", reg, dst)
	case 4:
		e.f("VPMOVZXBD (%s), X%s", reg, dst)
	default:
		e.f("MOVWLZX (%s), BX", reg)
		e.f("MOVQ BX, X%s", dst)
		e.f("VPMOVZXBD X%s, X%s", dst, dst)
	}
}

// avxVU writes vpassU8AVX2, vpassU4AVX2, hpassW8AVX2 or hpassW4AVX2 (B for bytes), the shift an
// argument in X14.
func avxVU(e *emitter, ntaps int, horiz, bytes bool) {
	name := filterName(ntaps, horiz, bytes) + "AVX2"
	typ, size := elem(bytes)
	step := "ADDQ R9, R10"
	if horiz {
		step = fmt.Sprintf("ADDQ $%d, R10", size)
	}
	e.raw(fmt.Sprintf("// func %s(dst *int32, dstStride int, src *%s, srcStride, w, h int, c *int16, shift int)\n", name, typ))
	e.raw(fmt.Sprintf("TEXT ·%s(SB), NOSPLIT, $0-64\n", name))
	e.f("MOVQ dst+0(FP), DI")
	e.f("MOVQ dstStride+8(FP), R8")
	e.f("MOVQ src+16(FP), SI")
	e.f("MOVQ srcStride+24(FP), R9")
	e.f("MOVQ w+32(FP), CX")
	e.f("MOVQ h+40(FP), DX")
	e.f("MOVQ c+48(FP), AX")
	e.f("SHLQ $2, R8")
	if !bytes {
		e.f("SHLQ $1, R9")
	}
	for k := 0; k < ntaps; k++ {
		e.f("MOVWQSX %d(AX), R10", 2*k)
		e.f("MOVQ R10, X%d", k)
		e.f("VPBROADCASTD X%d, Y%d", k, k)
	}
	e.f("MOVQ shift+56(FP), R10")
	e.f("MOVQ R10, X14")
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
		for k := 0; k < ntaps; k++ {
			if bytes {
				avxLoadBytes(e, cols, "R10", "9")
			} else {
				switch cols {
				case 8, 4:
					e.f("VPMOVZXWD (R10), %s9", reg)
				default:
					e.f("VMOVD (R10), X9")
					e.f("VPMOVZXWD X9, X9")
				}
			}
			e.f("VPMULLD %s%d, %s9, %s9", reg, k, reg, reg)
			e.f("VPADDD %s9, %s, %s", reg, acc, acc)
			if k < ntaps-1 {
				e.f("%s", step)
			}
		}
		e.f("VPSRAD X14, %s, %s", acc, acc)
		switch cols {
		case 8:
			e.f("VMOVDQU Y8, (R13)")
		case 4:
			e.f("VMOVDQU X8, (R13)")
		default:
			e.f("VMOVQ X8, (R13)")
		}
		e.f("ADDQ $%d, R12", size*cols)
		e.f("ADDQ $%d, R13", 4*cols)
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

// avxWiden is widenAVX2 (widenBAVX2 for bytes): dst = int32(src) << shift, the shift in X14.
func avxWiden(e *emitter, bytes bool) {
	name := "widenAVX2"
	if bytes {
		name = "widenBAVX2"
	}
	typ, size := elem(bytes)
	e.raw(fmt.Sprintf("// func %s(dst *int32, dstStride int, src *%s, srcStride, w, h, shift int)\n", name, typ))
	e.raw(fmt.Sprintf("TEXT ·%s(SB), NOSPLIT, $0-56\n", name))
	e.f("MOVQ dst+0(FP), DI")
	e.f("MOVQ dstStride+8(FP), R8")
	e.f("MOVQ src+16(FP), SI")
	e.f("MOVQ srcStride+24(FP), R9")
	e.f("MOVQ w+32(FP), CX")
	e.f("MOVQ h+40(FP), DX")
	e.f("MOVQ shift+48(FP), R10")
	e.f("MOVQ R10, X14")
	e.f("SHLQ $2, R8")
	if !bytes {
		e.f("SHLQ $1, R9")
	}
	e.label("rows")
	e.f("MOVQ SI, R12")
	e.f("MOVQ DI, R13")
	e.f("MOVQ CX, R11")
	e.label("cols8")
	e.f("CMPQ R11, $8")
	e.f("JLT tail4")
	if bytes {
		avxLoadBytes(e, 8, "R12", "0")
	} else {
		e.f("VPMOVZXWD (R12), Y0")
	}
	e.f("VPSLLD X14, Y0, Y0")
	e.f("VMOVDQU Y0, (R13)")
	e.f("ADDQ $%d, R12", 8*size)
	e.f("ADDQ $32, R13")
	e.f("SUBQ $8, R11")
	e.f("JMP cols8")
	e.label("tail4")
	e.f("CMPQ R11, $4")
	e.f("JLT tail2")
	if bytes {
		avxLoadBytes(e, 4, "R12", "0")
	} else {
		e.f("VPMOVZXWD (R12), X0")
	}
	e.f("VPSLLD X14, X0, X0")
	e.f("VMOVDQU X0, (R13)")
	e.f("ADDQ $%d, R12", 4*size)
	e.f("ADDQ $16, R13")
	e.f("SUBQ $4, R11")
	e.label("tail2")
	e.f("TESTQ R11, R11")
	e.f("JZ next")
	if bytes {
		avxLoadBytes(e, 2, "R12", "0")
	} else {
		e.f("VMOVD (R12), X0")
		e.f("VPMOVZXWD X0, X0")
	}
	e.f("VPSLLD X14, X0, X0")
	e.f("VMOVQ X0, (R13)")
	e.label("next")
	e.f("ADDQ R9, SI")
	e.f("ADDQ R8, DI")
	e.f("DECQ DX")
	e.f("JNZ rows")
	e.f("VZEROUPPER")
	e.f("RET")
	e.raw("\n")
}
