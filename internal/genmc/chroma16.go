package main

import (
	"fmt"
	"strings"
)

// Cb and Cr in one pass, sixteen lanes: the eight lines of the Cb edge in the
// low half of every register and the eight lines of the Cr edge in the high
// half, so the arithmetic of chroma.go runs once for both planes. A vertical
// edge packs row r of each plane (four bytes each) into one eight-byte row, so
// a single 8x8 transpose yields all eight columns, none of them wasted: the
// transposition cost per line is half that of the one-plane kernels.
//
// V0..V3 are p1, p0, q0, q1 (sixteen lanes). Constants are the one-plane
// kernels': V20 = alpha, V21 = beta, V22 = tC0 per line, V23 = the line
// mask, V24 = 4 and V13 = 1 (sixteen-bit lanes); the strong form has V23 = 2.

// chroma16NormalCore is chromaNormalCore on sixteen lanes.
func (a *asm) chroma16NormalCore() {
	a.lessThan(1, 2, 20, 10)
	a.lessThan(0, 1, 21, 11)
	a.lessThan(3, 2, 21, 12)
	a.f("VAND V11.B16, V10.B16, V10.B16")
	a.f("VAND V12.B16, V10.B16, V10.B16")
	a.f("VAND V23.B16, V10.B16, V10.B16")
	for _, hi := range []bool{false, true} {
		a.widen(1, 25, hi) // p0
		a.widen(2, 26, hi) // q0
		a.widen(0, 27, hi) // p1
		a.widen(3, 28, hi) // q1
		a.f("VSUB V25.H8, V26.H8, V29.H8")
		a.f("VADD V29.H8, V29.H8, V29.H8")
		a.f("VADD V29.H8, V29.H8, V29.H8")
		a.f("VSUB V28.H8, V27.H8, V30.H8")
		a.f("VADD V30.H8, V29.H8, V29.H8")
		a.f("VADD V24.H8, V29.H8, V29.H8")
		a.f("VSSHR $3, V29.H8, V29.H8")
		a.widen(22, 30, hi)
		a.f("VADD V13.H8, V30.H8, V30.H8")
		a.f("VNEG V30.H8, V31.H8")
		a.f("VSMIN V30.H8, V29.H8, V29.H8")
		a.f("VSMAX V31.H8, V29.H8, V29.H8")
		a.f("VADD V29.H8, V25.H8, V8.H8")
		a.narrowSat(8, 17, hi)
		a.f("VSUB V29.H8, V26.H8, V8.H8")
		a.narrowSat(8, 18, hi)
	}
	a.f("VBIT V10.B16, V17.B16, V1.B16")
	a.f("VBIT V10.B16, V18.B16, V2.B16")
}

// chroma16StrongCore is chromaStrongCore on sixteen lanes.
func (a *asm) chroma16StrongCore() {
	a.lessThan(1, 2, 20, 10)
	a.lessThan(0, 1, 21, 11)
	a.lessThan(3, 2, 21, 12)
	a.f("VAND V11.B16, V10.B16, V10.B16")
	a.f("VAND V12.B16, V10.B16, V10.B16")
	for _, hi := range []bool{false, true} {
		a.widen(0, 25, hi) // p1
		a.widen(1, 26, hi) // p0
		a.widen(2, 27, hi) // q0
		a.widen(3, 28, hi) // q1
		a.f("VADD V25.H8, V25.H8, V8.H8")
		a.f("VADD V26.H8, V8.H8, V8.H8")
		a.f("VADD V28.H8, V8.H8, V8.H8")
		a.f("VADD V23.H8, V8.H8, V8.H8")
		a.narrowShift(2, 8, 17, hi)
		a.f("VADD V28.H8, V28.H8, V9.H8")
		a.f("VADD V27.H8, V9.H8, V9.H8")
		a.f("VADD V25.H8, V9.H8, V9.H8")
		a.f("VADD V23.H8, V9.H8, V9.H8")
		a.narrowShift(2, 9, 18, hi)
	}
	a.f("VBIT V10.B16, V17.B16, V1.B16")
	a.f("VBIT V10.B16, V18.B16, V2.B16")
}

// chroma16Func writes a two-plane kernel:
//
//	func name(cb, cr *byte, stride int, alpha, beta int, tc0, mask uint32)
//
// (the strong forms take no tc0 and mask).
func chroma16Func(name string, strong, vertical bool) string {
	var a asm
	size := 68
	if strong {
		size = 56
		fmt.Fprintf(&a.b, "// func %s(cb, cr *byte, stride int, alphaCb, betaCb, alphaCr, betaCr int)\n", name)
	} else {
		fmt.Fprintf(&a.b, "// func %s(cb, cr *byte, stride int, alphaCb, betaCb, alphaCr, betaCr int, tc0Cb, tc0Cr, mask uint32)\n", name)
	}
	fmt.Fprintf(&a.b, "TEXT ·%s(SB), NOSPLIT, $0-%d\n", name, size)
	a.f("MOVD cb+0(FP), R0")
	a.f("MOVD cr+8(FP), R9")
	a.f("MOVD stride+16(FP), R1")
	a.f("MOVD alphaCb+24(FP), R4")
	a.f("MOVD betaCb+32(FP), R5")
	a.f("MOVD alphaCr+40(FP), R11")
	a.f("MOVD betaCr+48(FP), R12")
	if !strong {
		a.f("MOVWU tc0Cb+56(FP), R6")
		a.f("MOVWU tc0Cr+60(FP), R13")
		a.f("MOVWU mask+64(FP), R7")
	}

	if vertical {
		// Row r of both planes, four bytes each, side by side in one eight-byte
		// row; the transpose turns rows into the columns p1, p0, q0, q1 of Cb
		// (V0..V3) and of Cr (V4..V7).
		a.f("SUB $2, R0, R2")
		a.f("SUB $2, R9, R10")
		for i := 0; i < 8; i++ {
			a.f("FMOVS (R2), F%d", i)
			a.f("VLD1 (R10), V%d.S[1]", i)
			a.f("ADD R1, R2")
			a.f("ADD R1, R10")
		}
		a.transpose8(seq(0), seq(8), seq(16), seq(0))
		for i := 0; i < 4; i++ {
			a.f("VZIP1 V%d.D2, V%d.D2, V%d.D2", 4+i, i, i)
		}
	} else {
		a.f("LSL $1, R1, R8")
		a.f("SUB R8, R0, R2")
		a.f("SUB R8, R9, R10")
		for i := 0; i < 4; i++ {
			a.f("FMOVD (R2), F%d", i)
			a.f("VLD1 (R10), V%d.D[1]", i)
			a.f("ADD R1, R2")
			a.f("ADD R1, R10")
		}
	}

	// Thresholds and tC0 are per plane: Cb in the low half, Cr in the high.
	a.f("VDUP R4, V20.B16")
	a.f("VDUP R11, V8.B16")
	a.f("VMOV V8.D[0], V20.D[1]")
	a.f("VDUP R5, V21.B16")
	a.f("VDUP R12, V8.B16")
	a.f("VMOV V8.D[0], V21.D[1]")
	if strong {
		a.f("MOVD $2, R8")
		a.f("VDUP R8, V23.H8")
		a.chroma16StrongCore()
	} else {
		a.f("VMOV R6, V22.S[0]")
		a.f("VZIP1 V22.B8, V22.B8, V22.B8") // two lines per group
		a.f("VMOV R13, V8.S[0]")
		a.f("VZIP1 V8.B8, V8.B8, V8.B8")
		a.f("VMOV V8.D[0], V22.D[1]") // Cr's tC0 in the high half
		a.f("VMOV R7, V23.S[0]")
		a.f("VZIP1 V23.B8, V23.B8, V23.B8")
		a.f("VMOV V23.D[0], V23.D[1]") // the strength mask is shared
		a.f("MOVD $4, R8")
		a.f("VDUP R8, V24.H8")
		a.f("MOVD $1, R8")
		a.f("VDUP R8, V13.H8")
		a.chroma16NormalCore()
	}

	if vertical {
		// The high halves of V0..V3 are the Cr columns; with the low halves
		// they are the eight columns of the transposed block.
		for i := 0; i < 4; i++ {
			a.f("VZIP2 V%d.D2, V%d.D2, V%d.D2", i, i, 4+i)
		}
		a.transpose8(seq(0), seq(8), seq(16), seq(8))
		a.f("SUB $2, R0, R2")
		a.f("SUB $2, R9, R10")
		for i := 0; i < 8; i++ {
			a.f("FMOVS F%d, (R2)", 8+i)
			a.f("VST1 V%d.S[1], (R10)", 8+i)
			a.f("ADD R1, R2")
			a.f("ADD R1, R10")
		}
	} else {
		a.f("SUB R1, R0, R2")
		a.f("SUB R1, R9, R10")
		a.f("FMOVD F1, (R2)")
		a.f("VST1 V1.D[1], (R10)")
		a.f("FMOVD F2, (R0)")
		a.f("VST1 V2.D[1], (R9)")
	}
	a.f("RET")
	return a.b.String() + "\n"
}

func generateChroma16() string {
	var s strings.Builder
	s.WriteString(header)
	s.WriteString("//go:build arm64 && !purego\n\n#include \"textflag.h\"\n\n")
	s.WriteString("// H.264 chroma deblocking of Cb and Cr in one pass (4:2:0, 8-bit), NEON: the\n")
	s.WriteString("// eight lines of each plane share a register. See internal/genmc/chroma16.go.\n\n")
	s.WriteString(chroma16Func("deblockChromaPairNormalH", false, false))
	s.WriteString(chroma16Func("deblockChromaPairNormalV", false, true))
	// No two-plane strong horizontal kernel: measured at 11.7 ns against
	// 10.8 ns for two one-plane calls.
	s.WriteString(chroma16Func("deblockChromaPairStrongV", true, true))
	return s.String()
}
