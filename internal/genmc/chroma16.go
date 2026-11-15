package main

import (
	"fmt"
	"strings"
)

// Cb and Cr in one pass, sixteen lanes, strong vertical edge: the eight lines
// of the Cb edge in the low half of every register and the eight lines of the
// Cr edge in the high half, so the arithmetic of chroma.go runs once for both
// planes, each with its own alpha and beta. Row r of each plane (four bytes
// each) is packed into one eight-byte row, so a single 8x8 transpose yields all
// eight columns, none of them wasted: half the transposition cost per line of
// the one-plane kernels.
//
// Only this edge is generated: the normal forms and the strong horizontal one
// were written, measured against two one-plane calls (a tie, a result inside
// the noise, and a loss: see lf/chroma_pair_arm64.go) and not kept.
//
// V0..V3 are p1, p0, q0, q1 (sixteen lanes). V20 and V21 are alpha and beta
// (Cb low, Cr high), V23 = 2 (sixteen-bit lanes).

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

// chroma16StrongV writes
//
//	func name(cb, cr *byte, stride int, alphaCb, betaCb, alphaCr, betaCr int)
func chroma16StrongV(name string) string {
	var a asm
	fmt.Fprintf(&a.b, "// func %s(cb, cr *byte, stride int, alphaCb, betaCb, alphaCr, betaCr int)\n", name)
	fmt.Fprintf(&a.b, "TEXT ·%s(SB), NOSPLIT, $0-56\n", name)
	a.f("MOVD cb+0(FP), R0")
	a.f("MOVD cr+8(FP), R9")
	a.f("MOVD stride+16(FP), R1")
	a.f("MOVD alphaCb+24(FP), R4")
	a.f("MOVD betaCb+32(FP), R5")
	a.f("MOVD alphaCr+40(FP), R11")
	a.f("MOVD betaCr+48(FP), R12")

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

	// Thresholds per plane: Cb in the low half, Cr in the high.
	a.f("VDUP R4, V20.B16")
	a.f("VDUP R11, V8.B16")
	a.f("VMOV V8.D[0], V20.D[1]")
	a.f("VDUP R5, V21.B16")
	a.f("VDUP R12, V8.B16")
	a.f("VMOV V8.D[0], V21.D[1]")
	a.f("MOVD $2, R8")
	a.f("VDUP R8, V23.H8")
	a.chroma16StrongCore()

	// The high halves of V0..V3 are the Cr columns; with the low halves they
	// are the eight columns of the transposed block.
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
	a.f("RET")
	return a.b.String() + "\n"
}

func generateChroma16() string {
	var s strings.Builder
	s.WriteString(header)
	s.WriteString("//go:build arm64 && !purego\n\n#include \"textflag.h\"\n\n")
	s.WriteString("// H.264 chroma deblocking of Cb and Cr in one pass (4:2:0, 8-bit, bS 4,\n")
	s.WriteString("// vertical edge), NEON: the eight lines of each plane share a register.\n")
	s.WriteString("// See internal/genmc/chroma16.go.\n\n")
	s.WriteString(chroma16StrongV("deblockChromaPairStrongV"))
	return s.String()
}
