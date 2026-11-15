package main

import (
	"fmt"
	"strings"
)

// NEON chroma deblocking, 4:2:0, 8-bit (ITU-T H.264 (V16) (06/2026), clauses
// 8.7.2.2, 8.7.2.3 and 8.7.2.4 with chromaStyleFilteringFlag 1): eight lines
// per call, the four samples p1, p0, q0, q1 across the edge in V0..V3, eight
// lines per register (low eight bytes). Only p0 and q0 change.

// lt8 leaves in dst the mask (|x - y| < limit) on eight byte lanes.
func (a *asm) lt8(x, y, limit, dst int) {
	a.f("VUQSUB V%d.B8, V%d.B8, V8.B8", y, x)
	a.f("VUQSUB V%d.B8, V%d.B8, V9.B8", x, y)
	a.f("VORR V9.B8, V8.B8, V8.B8")
	a.f("VCMHI V8.B8, V%d.B8, V%d.B8", limit, dst)
}

// chromaNormalCore: V20 = alpha, V21 = beta, V22 = tC0 per line, V23 =
// 0xff on the lines whose bS is not 0 (bytes), V24 = 4 and V13 = 1
// (sixteen-bit lanes). Equations 8-466 (tC = tC0 + 1), 8-467 to 8-469.
func (a *asm) chromaNormalCore() {
	a.lt8(1, 2, 20, 10) // alpha > |p0 - q0|
	a.lt8(0, 1, 21, 11) // beta > |p1 - p0|
	a.lt8(3, 2, 21, 12) // beta > |q1 - q0|
	a.f("VAND V11.B8, V10.B8, V10.B8")
	a.f("VAND V12.B8, V10.B8, V10.B8")
	a.f("VAND V23.B8, V10.B8, V10.B8") // filterSamplesFlag, 8-460
	a.f("VUXTL V1.B8, V25.H8")         // p0
	a.f("VUXTL V2.B8, V26.H8")         // q0
	a.f("VUXTL V0.B8, V27.H8")         // p1
	a.f("VUXTL V3.B8, V28.H8")         // q1
	a.f("VSUB V25.H8, V26.H8, V29.H8")
	a.f("VADD V29.H8, V29.H8, V29.H8")
	a.f("VADD V29.H8, V29.H8, V29.H8")
	a.f("VSUB V28.H8, V27.H8, V30.H8")
	a.f("VADD V30.H8, V29.H8, V29.H8")
	a.f("VADD V24.H8, V29.H8, V29.H8")
	a.f("VSSHR $3, V29.H8, V29.H8")
	a.f("VUXTL V22.B8, V30.H8")
	a.f("VADD V13.H8, V30.H8, V30.H8") // tC = tC0 + 1
	a.f("VNEG V30.H8, V31.H8")
	a.f("VSMIN V30.H8, V29.H8, V29.H8")
	a.f("VSMAX V31.H8, V29.H8, V29.H8")
	a.f("VADD V29.H8, V25.H8, V8.H8")
	a.f("VSQXTUN V8.H8, V17.B8")
	a.f("VSUB V29.H8, V26.H8, V8.H8")
	a.f("VSQXTUN V8.H8, V18.B8")
	a.f("VBIT V10.B8, V17.B8, V1.B8")
	a.f("VBIT V10.B8, V18.B8, V2.B8")
}

// chromaStrongCore: V20 = alpha, V21 = beta, V23 = 2 (sixteen-bit lanes).
// Equations 8-480 and 8-487.
func (a *asm) chromaStrongCore() {
	a.lt8(1, 2, 20, 10)
	a.lt8(0, 1, 21, 11)
	a.lt8(3, 2, 21, 12)
	a.f("VAND V11.B8, V10.B8, V10.B8")
	a.f("VAND V12.B8, V10.B8, V10.B8")
	a.f("VUXTL V0.B8, V25.H8") // p1
	a.f("VUXTL V1.B8, V26.H8") // p0
	a.f("VUXTL V2.B8, V27.H8") // q0
	a.f("VUXTL V3.B8, V28.H8") // q1
	a.f("VADD V25.H8, V25.H8, V8.H8")
	a.f("VADD V26.H8, V8.H8, V8.H8")
	a.f("VADD V28.H8, V8.H8, V8.H8")
	a.f("VADD V23.H8, V8.H8, V8.H8")
	a.f("VSHRN $2, V8.H8, V17.B8") // (2*p1 + p0 + q1 + 2) >> 2
	a.f("VADD V28.H8, V28.H8, V9.H8")
	a.f("VADD V27.H8, V9.H8, V9.H8")
	a.f("VADD V25.H8, V9.H8, V9.H8")
	a.f("VADD V23.H8, V9.H8, V9.H8")
	a.f("VSHRN $2, V9.H8, V18.B8") // (2*q1 + q0 + p1 + 2) >> 2
	a.f("VBIT V10.B8, V17.B8, V1.B8")
	a.f("VBIT V10.B8, V18.B8, V2.B8")
}

func chromaFunc(name string, strong, vertical bool) string {
	var a asm
	if strong {
		fmt.Fprintf(&a.b, "// func %s(pix *byte, stride int, alpha, beta int)\n", name)
		fmt.Fprintf(&a.b, "TEXT ·%s(SB), NOSPLIT, $0-32\n", name)
	} else {
		fmt.Fprintf(&a.b, "// func %s(pix *byte, stride int, alpha, beta int, tc0, mask uint32)\n", name)
		fmt.Fprintf(&a.b, "TEXT ·%s(SB), NOSPLIT, $0-40\n", name)
	}
	a.f("MOVD pix+0(FP), R0")
	a.f("MOVD stride+8(FP), R1")
	a.f("MOVD alpha+16(FP), R4")
	a.f("MOVD beta+24(FP), R5")
	if !strong {
		a.f("MOVWU tc0+32(FP), R6")
		a.f("MOVWU mask+36(FP), R7")
	}

	if vertical {
		// Eight rows of four bytes at pix - 2, transposed: V0..V3 become the
		// columns p1, p0, q0, q1 (V4..V7 hold zero columns).
		a.f("SUB $2, R0, R2")
		for i := 0; i < 8; i++ {
			a.f("FMOVS (R2), F%d", i)
			a.f("ADD R1, R2")
		}
		a.transpose8(seq(0), seq(8), seq(16), seq(0))
	} else {
		a.f("LSL $1, R1, R8")
		a.f("SUB R8, R0, R2")
		for i := 0; i < 4; i++ {
			a.f("FMOVD (R2), F%d", i)
			a.f("ADD R1, R2")
		}
	}

	a.f("VDUP R4, V20.B8")
	a.f("VDUP R5, V21.B8")
	if strong {
		a.f("MOVD $2, R8")
		a.f("VDUP R8, V23.H8")
		a.chromaStrongCore()
	} else {
		a.f("VMOV R6, V22.S[0]")
		a.f("VZIP1 V22.B8, V22.B8, V22.B8") // two lines per group
		a.f("VMOV R7, V23.S[0]")
		a.f("VZIP1 V23.B8, V23.B8, V23.B8")
		a.f("MOVD $4, R8")
		a.f("VDUP R8, V24.H8")
		a.f("MOVD $1, R8")
		a.f("VDUP R8, V13.H8")
		a.chromaNormalCore()
	}

	if vertical {
		a.transpose8(seq(0), seq(8), seq(16), seq(8))
		a.f("SUB $2, R0, R2")
		for i := 0; i < 8; i++ {
			a.f("FMOVS F%d, (R2)", 8+i)
			a.f("ADD R1, R2")
		}
	} else {
		a.f("SUB R1, R0, R2") // p0 is the row above pix
		a.f("FMOVD F1, (R2)")
		a.f("FMOVD F2, (R0)")
	}
	a.f("RET")
	return a.b.String() + "\n"
}

func generateChroma() string {
	var s strings.Builder
	s.WriteString(header)
	s.WriteString("//go:build arm64 && !purego\n\n#include \"textflag.h\"\n\n")
	s.WriteString("// H.264 chroma deblocking (4:2:0, 8-bit), NEON: eight lines per call, the\n")
	s.WriteString("// four samples p1..q1 across the edge in V0..V3. See internal/genmc/chroma.go.\n\n")
	s.WriteString(chromaFunc("deblockChromaNormalH", false, false))
	s.WriteString(chromaFunc("deblockChromaNormalV", false, true))
	s.WriteString(chromaFunc("deblockChromaStrongH", true, false))
	// No strong vertical kernel: eight lines of the weak p0/q0 rewrite cost
	// the transposes more than they save (measured: a tie with portable Go).
	return s.String()
}
