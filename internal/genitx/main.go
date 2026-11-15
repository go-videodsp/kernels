// Command genitx writes the amd64 AVX2 and arm64 NEON assembly for the H.264
// 8x8 inverse transform fused with the scaling of the coefficients and the
// add to the prediction (package itx: Dequant8x8IDCTAdd).
//
// Run from the repository root: go run ./internal/genitx
//
// The kernels are written from ITU-T H.264 (V16) (06/2026): clause 8.5.9
// (scaling of the 8x8 coefficients, Equations 8-337 and the two qP ranges),
// clause 8.5.13 (the one-dimensional transform, Equations 8-338 to 8-364, rows
// first, then columns) and clause 8.5.14 (Equation 8-406, the final
// (x + 32) >> 6, and the Clip1 of the sum with the prediction), not from any
// decoder.
//
// Arithmetic is the standard's in thirty-two-bit two's complement, so the
// vector code wraps exactly where the reference does:
//
//   - scaling: (c * ls << sl + rnd) >> sr, with sl = qP/6 - 6, rnd = 0, sr = 0
//     when qP >= 36 and sl = 0, rnd = 2^(5 - qP/6), sr = 6 - qP/6 below;
//   - the rows and the columns are the same butterfly on eight vectors: the
//     eight rows sit in eight registers (AVX2) or in sixteen (NEON, two per
//     row), the transform runs on registers as the "samples" of a column, so
//     one pass transforms every column at once; a transposition turns the
//     rows into columns for the first pass and back for the second;
//   - a block whose only coefficient is the first one (the common case in real
//     streams) adds the one value (c0 + 32) >> 6 to every sample, which is what
//     both passes produce for it, and skips both transforms;
//   - the sum with the prediction is clipped with a signed saturating narrow
//     (NEON) or an unsigned min and pack (AVX2).
//
// The AVX2 kernel reads and writes exactly the eight rows of eight bytes.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var osExit = os.Exit

type gen struct{ b strings.Builder }

func (g *gen) f(format string, a ...any) { fmt.Fprintf(&g.b, "\t"+format+"\n", a...) }
func (g *gen) l(format string, a ...any) { fmt.Fprintf(&g.b, format+"\n", a...) }

// ---------------------------------------------------------------- amd64

func y(n int) string { return fmt.Sprintf("Y%d", n) }

func yregs(from int) (r [8]string) {
	for i := range r {
		r[i] = y(from + i)
	}
	return r
}

// idct8x is the butterfly of 8.5.13 on eight registers in s, with the eight
// registers of t as scratch. It returns the registers holding the eight
// outputs and the eight that are free afterwards.
func (g *gen) idct8x(s, t [8]string) (out, free [8]string) {
	// even part: a0, a2, a4, a6 then b0, b2, b4, b6
	g.f("VPADDD %s, %s, %s", s[4], s[0], t[0])
	g.f("VPSUBD %s, %s, %s", s[4], s[0], t[1])
	g.f("VPSRAD $1, %s, %s", s[2], t[2])
	g.f("VPSUBD %s, %s, %s", s[6], t[2], t[2])
	g.f("VPSRAD $1, %s, %s", s[6], t[3])
	g.f("VPADDD %s, %s, %s", s[2], t[3], t[3])
	g.f("VPADDD %s, %s, %s", t[3], t[0], t[4]) // b0
	g.f("VPSUBD %s, %s, %s", t[3], t[0], t[0]) // b6
	g.f("VPADDD %s, %s, %s", t[2], t[1], t[3]) // b2
	g.f("VPSUBD %s, %s, %s", t[2], t[1], t[1]) // b4
	// odd part: a1, a3, a5, a7 into s0, s2, s4, s6
	g.f("VPSUBD %s, %s, %s", s[3], s[5], s[0])
	g.f("VPSRAD $1, %s, %s", s[7], t[2])
	g.f("VPSUBD %s, %s, %s", s[7], s[0], s[0])
	g.f("VPSUBD %s, %s, %s", t[2], s[0], s[0])
	g.f("VPADDD %s, %s, %s", s[7], s[1], s[2])
	g.f("VPSUBD %s, %s, %s", s[3], s[2], s[2])
	g.f("VPSRAD $1, %s, %s", s[3], t[2])
	g.f("VPSUBD %s, %s, %s", t[2], s[2], s[2])
	g.f("VPSUBD %s, %s, %s", s[1], s[7], s[4])
	g.f("VPADDD %s, %s, %s", s[5], s[4], s[4])
	g.f("VPSRAD $1, %s, %s", s[5], t[2])
	g.f("VPADDD %s, %s, %s", t[2], s[4], s[4])
	g.f("VPADDD %s, %s, %s", s[5], s[3], s[6])
	g.f("VPADDD %s, %s, %s", s[1], s[6], s[6])
	g.f("VPSRAD $1, %s, %s", s[1], t[2])
	g.f("VPADDD %s, %s, %s", t[2], s[6], s[6])
	// b1, b3, b5, b7 into s1, s3, s5, s7
	g.f("VPSRAD $2, %s, %s", s[6], s[1])
	g.f("VPADDD %s, %s, %s", s[0], s[1], s[1])
	g.f("VPSRAD $2, %s, %s", s[4], s[3])
	g.f("VPADDD %s, %s, %s", s[2], s[3], s[3])
	g.f("VPSRAD $2, %s, %s", s[2], s[5])
	g.f("VPSUBD %s, %s, %s", s[4], s[5], s[5])
	g.f("VPSRAD $2, %s, %s", s[0], s[7])
	g.f("VPSUBD %s, %s, %s", s[7], s[6], s[7])
	// outputs
	g.f("VPADDD %s, %s, %s", s[7], t[4], s[0]) // o0 = b0 + b7
	g.f("VPSUBD %s, %s, %s", s[7], t[4], s[7]) // o7 = b0 - b7
	g.f("VPADDD %s, %s, %s", s[5], t[3], s[2]) // o1 = b2 + b5
	g.f("VPSUBD %s, %s, %s", s[5], t[3], s[6]) // o6 = b2 - b5
	g.f("VPADDD %s, %s, %s", s[3], t[1], s[4]) // o2 = b4 + b3
	g.f("VPSUBD %s, %s, %s", s[3], t[1], t[2]) // o5 = b4 - b3
	g.f("VPADDD %s, %s, %s", s[1], t[0], t[5]) // o3 = b6 + b1
	g.f("VPSUBD %s, %s, %s", s[1], t[0], t[6]) // o4 = b6 - b1
	out = [8]string{s[0], s[2], s[4], t[5], t[6], t[2], s[6], s[7]}
	free = [8]string{s[1], s[3], s[5], t[0], t[1], t[3], t[4], t[7]}
	return out, free
}

// transpose8 transposes the 8x8 matrix of thirty-two-bit lanes held in the
// registers a (one row each) into the registers t, which it returns; a is free
// afterwards.
func (g *gen) transpose8(a, t [8]string) (out, free [8]string) {
	for i := 0; i < 4; i++ {
		g.f("VPUNPCKLDQ %s, %s, %s", a[2*i+1], a[2*i], t[2*i])
		g.f("VPUNPCKHDQ %s, %s, %s", a[2*i+1], a[2*i], t[2*i+1])
	}
	for i := 0; i < 2; i++ {
		g.f("VPUNPCKLQDQ %s, %s, %s", t[4*i+2], t[4*i], a[4*i])
		g.f("VPUNPCKHQDQ %s, %s, %s", t[4*i+2], t[4*i], a[4*i+1])
		g.f("VPUNPCKLQDQ %s, %s, %s", t[4*i+3], t[4*i+1], a[4*i+2])
		g.f("VPUNPCKHQDQ %s, %s, %s", t[4*i+3], t[4*i+1], a[4*i+3])
	}
	for c := 0; c < 4; c++ {
		g.f("VPERM2I128 $0x20, %s, %s, %s", a[c+4], a[c], t[c])
		g.f("VPERM2I128 $0x31, %s, %s, %s", a[c+4], a[c], t[c+4])
	}
	return t, a
}

// add86 adds the eight rows of residual r to the eight rows of bytes at DI
// (stride DX), clips and stores them; t holds scratch registers, and the
// constant 255 is made in t[7].
func (g *gen) add86(r, t [8]string) {
	g.f("MOVL $255, AX")
	g.f("VMOVD AX, X%s", t[7][1:])
	g.f("VPBROADCASTD X%s, %s", t[7][1:], t[7])
	for p := 0; p < 4; p++ {
		a, b := t[0], t[1]
		g.f("VPMOVZXBD (DI), %s", a)
		g.f("VPMOVZXBD (DI)(DX*1), %s", b)
		g.f("VPADDD %s, %s, %s", r[2*p], a, a)
		g.f("VPADDD %s, %s, %s", r[2*p+1], b, b)
		g.f("VPMINSD %s, %s, %s", t[7], a, a)
		g.f("VPMINSD %s, %s, %s", t[7], b, b)
		g.f("VPACKUSDW %s, %s, %s", b, a, a)
		g.f("VPACKUSWB %s, %s, %s", a, a, a)
		g.f("VPERMQ $0x08, %s, %s", a, a)
		g.f("VPSHUFD $0xD8, %s, %s", a, a)
		g.f("VMOVQ X%s, (DI)", a[1:])
		g.f("VPEXTRQ $1, X%s, (DI)(DX*1)", a[1:])
		if p < 3 {
			g.f("LEAQ (DI)(DX*2), DI")
		}
	}
}

func (g *gen) amd64() string {
	g.b.Reset()
	g.l("// Code generated by internal/genitx; DO NOT EDIT.")
	g.l("")
	g.l("//go:build amd64 && !purego")
	g.l("")
	g.l("#include \"textflag.h\"")
	g.l("")
	g.l("DATA idct8Round<>+0(SB)/4, $32")
	g.l("GLOBL idct8Round<>(SB), RODATA|NOPTR, $32")
	g.l("")
	g.l("// func idct8AVX2(dst *byte, stride int, coef, scale *[64]int32, sl, rnd, sr int)")
	g.l("TEXT ·idct8AVX2(SB), NOSPLIT, $0-56")
	g.f("MOVQ dst+0(FP), DI")
	g.f("MOVQ stride+8(FP), DX")
	g.f("MOVQ coef+16(FP), SI")
	g.f("MOVQ scale+24(FP), R8")
	g.f("MOVQ sl+32(FP), AX")
	g.f("VMOVQ AX, X8")
	g.f("MOVQ rnd+40(FP), AX")
	g.f("VMOVD AX, X9")
	g.f("VPBROADCASTD X9, Y9")
	g.f("MOVQ sr+48(FP), AX")
	g.f("VMOVQ AX, X10")
	for i := 0; i < 8; i++ {
		g.f("VMOVDQU %d(SI), Y%d", 32*i, i)
		g.f("VPMULLD %d(R8), Y%d, Y%d", 32*i, i, i)
		g.f("VPSLLD X8, Y%d, Y%d", i, i)
		g.f("VPADDD Y9, Y%d, Y%d", i, i)
		g.f("VPSRAD X10, Y%d, Y%d", i, i)
	}
	// DC only: every other coefficient is zero
	g.f("VPOR Y1, Y2, Y8")
	g.f("VPOR Y3, Y4, Y9")
	g.f("VPOR Y5, Y6, Y10")
	g.f("VPOR Y9, Y8, Y8")
	g.f("VPOR Y7, Y10, Y10")
	g.f("VPOR Y10, Y8, Y8")
	g.f("VPXOR Y9, Y9, Y9")
	g.f("VPBLENDD $0x01, Y9, Y0, Y10")
	g.f("VPOR Y10, Y8, Y8")
	g.f("VPTEST Y8, Y8")
	g.f("JZ dc")
	g.f("VPADDD idct8Round<>(SB), Y0, Y0")
	out, free := g.transpose8(yregs(0), yregs(8))
	out, free = g.idct8x(out, free)
	out, free = g.transpose8(out, free)
	out, free = g.idct8x(out, free)
	for _, r := range out {
		g.f("VPSRAD $6, %s, %s", r, r)
	}
	g.add86(out, free)
	g.f("VZEROUPPER")
	g.f("RET")
	g.l("dc:")
	g.f("VPADDD idct8Round<>(SB), Y0, Y0")
	g.f("VPSRAD $6, Y0, Y0")
	g.f("VPBROADCASTD X0, Y0")
	dc := [8]string{"Y0", "Y0", "Y0", "Y0", "Y0", "Y0", "Y0", "Y0"}
	g.add86(dc, yregs(8))
	g.f("VZEROUPPER")
	g.f("RET")
	return g.b.String()
}

// ---------------------------------------------------------------- arm64

func v(n int) string { return fmt.Sprintf("V%d", n) }

// idct8n is idct8x for NEON: the same butterfly on eight vectors of four
// lanes.
func (g *gen) idct8n(s, t [8]string) (out, free [8]string) {
	add := func(a, b, d string) { g.f("VADD %s.S4, %s.S4, %s.S4", b, a, d) } // d = a + b
	sub := func(a, b, d string) { g.f("VSUB %s.S4, %s.S4, %s.S4", b, a, d) } // d = a - b
	sra := func(a string, n int, d string) { g.f("VSSHR $%d, %s.S4, %s.S4", n, a, d) }
	sra(s[2], 1, t[2])
	add(s[0], s[4], t[0])
	sub(s[0], s[4], t[1])
	sub(t[2], s[6], t[2])
	sra(s[6], 1, t[3])
	add(t[3], s[2], t[3])
	add(t[0], t[3], t[4]) // b0
	sub(t[0], t[3], t[0]) // b6
	add(t[1], t[2], t[3]) // b2
	sub(t[1], t[2], t[1]) // b4
	sub(s[5], s[3], s[0])
	sra(s[7], 1, t[2])
	sub(s[0], s[7], s[0])
	sub(s[0], t[2], s[0])
	add(s[1], s[7], s[2])
	sub(s[2], s[3], s[2])
	sra(s[3], 1, t[2])
	sub(s[2], t[2], s[2])
	sub(s[7], s[1], s[4])
	add(s[4], s[5], s[4])
	sra(s[5], 1, t[2])
	add(s[4], t[2], s[4])
	add(s[3], s[5], s[6])
	add(s[6], s[1], s[6])
	sra(s[1], 1, t[2])
	add(s[6], t[2], s[6])
	sra(s[6], 2, s[1])
	add(s[1], s[0], s[1])
	sra(s[4], 2, s[3])
	add(s[3], s[2], s[3])
	sra(s[2], 2, s[5])
	sub(s[5], s[4], s[5])
	sra(s[0], 2, s[7])
	sub(s[6], s[7], s[7])
	add(t[4], s[7], s[0]) // o0 = b0 + b7
	sub(t[4], s[7], s[7]) // o7 = b0 - b7
	add(t[3], s[5], s[2]) // o1 = b2 + b5
	sub(t[3], s[5], s[6]) // o6 = b2 - b5
	add(t[1], s[3], s[4]) // o2 = b4 + b3
	sub(t[1], s[3], t[2]) // o5 = b4 - b3
	add(t[0], s[1], t[5]) // o3 = b6 + b1
	sub(t[0], s[1], t[6]) // o4 = b6 - b1
	out = [8]string{s[0], s[2], s[4], t[5], t[6], t[2], s[6], s[7]}
	free = [8]string{s[1], s[3], s[5], t[0], t[1], t[3], t[4], t[7]}
	return out, free
}

// transpose4n transposes the 4x4 matrix of thirty-two-bit lanes held in a
// (one row each) in place, using the four registers of t.
func (g *gen) transpose4n(a, t [4]string) {
	g.f("VTRN1 %s.S4, %s.S4, %s.S4", a[1], a[0], t[0])
	g.f("VTRN2 %s.S4, %s.S4, %s.S4", a[1], a[0], t[1])
	g.f("VTRN1 %s.S4, %s.S4, %s.S4", a[3], a[2], t[2])
	g.f("VTRN2 %s.S4, %s.S4, %s.S4", a[3], a[2], t[3])
	g.f("VTRN1 %s.D2, %s.D2, %s.D2", t[2], t[0], a[0])
	g.f("VTRN1 %s.D2, %s.D2, %s.D2", t[3], t[1], a[1])
	g.f("VTRN2 %s.D2, %s.D2, %s.D2", t[2], t[0], a[2])
	g.f("VTRN2 %s.D2, %s.D2, %s.D2", t[3], t[1], a[3])
}

// add64 adds the eight rows (lo and hi halves) of residual to the bytes at R2
// (stride R3), clips and stores them. lo and hi hold the row vectors; t holds
// scratch registers.
func (g *gen) add64(lo, hi, t [8]string) {
	for r := 0; r < 8; r++ {
		g.f("VLD1 (R2), [%s.B8]", t[0])
		g.f("VUXTL %s.B8, %s.H8", t[0], t[0])
		g.f("VUXTL %s.H4, %s.S4", t[0], t[1])
		g.f("VUXTL2 %s.H8, %s.S4", t[0], t[2])
		g.f("VADD %s.S4, %s.S4, %s.S4", lo[r], t[1], t[1])
		g.f("VADD %s.S4, %s.S4, %s.S4", hi[r], t[2], t[2])
		g.f("VSQXTN %s.S4, %s.H4", t[1], t[3])
		g.f("VSQXTN2 %s.S4, %s.H8", t[2], t[3])
		g.f("VSQXTUN %s.H8, %s.B8", t[3], t[3])
		g.f("VST1 [%s.B8], (R2)", t[3])
		if r < 7 {
			g.f("ADD R3, R2, R2")
		}
	}
}

func (g *gen) arm64() string {
	g.b.Reset()
	g.l("// Code generated by internal/genitx; DO NOT EDIT.")
	g.l("")
	g.l("//go:build arm64 && !purego")
	g.l("")
	g.l("#include \"textflag.h\"")
	g.l("")
	g.l("// func idct8NEON(dst *byte, stride int, coef, scale *[64]int32, sl, rnd, sr int)")
	g.l("TEXT ·idct8NEON(SB), NOSPLIT, $0-56")
	g.f("MOVD dst+0(FP), R2")
	g.f("MOVD stride+8(FP), R3")
	g.f("MOVD coef+16(FP), R0")
	g.f("MOVD scale+24(FP), R1")
	g.f("MOVD sl+32(FP), R4")
	g.f("MOVD rnd+40(FP), R5")
	g.f("MOVD sr+48(FP), R6")
	g.f("NEG R6, R6")
	g.f("VDUP R4, V28.S4")
	g.f("VDUP R5, V29.S4")
	g.f("VDUP R6, V30.S4")
	for c := 0; c < 4; c++ {
		a, b, d, e := v(4*c), v(4*c+1), v(4*c+2), v(4*c+3)
		g.f("VLD1.P 64(R0), [%s.S4, %s.S4, %s.S4, %s.S4]", a, b, d, e)
		g.f("VLD1.P 64(R1), [V16.S4, V17.S4, V18.S4, V19.S4]")
		for k, r := range []string{a, b, d, e} {
			g.f("VMUL V%d.S4, %s.S4, %s.S4", 16+k, r, r)
			g.f("VSSHL V28.S4, %s.S4, %s.S4", r, r)
			g.f("VADD V29.S4, %s.S4, %s.S4", r, r)
			g.f("VSSHL V30.S4, %s.S4, %s.S4", r, r)
		}
	}
	// DC only
	g.f("MOVD ZR, R9")
	g.f("VMOV V0.S[0], R10")
	g.f("VMOV R9, V0.S[0]")
	g.f("VORR V1.B16, V2.B16, V16.B16")
	g.f("VORR V3.B16, V4.B16, V17.B16")
	g.f("VORR V5.B16, V6.B16, V18.B16")
	g.f("VORR V7.B16, V8.B16, V19.B16")
	g.f("VORR V9.B16, V10.B16, V20.B16")
	g.f("VORR V11.B16, V12.B16, V21.B16")
	g.f("VORR V13.B16, V14.B16, V22.B16")
	g.f("VORR V15.B16, V0.B16, V23.B16")
	g.f("VORR V17.B16, V16.B16, V16.B16")
	g.f("VORR V19.B16, V18.B16, V18.B16")
	g.f("VORR V21.B16, V20.B16, V20.B16")
	g.f("VORR V23.B16, V22.B16, V22.B16")
	g.f("VORR V18.B16, V16.B16, V16.B16")
	g.f("VORR V22.B16, V20.B16, V20.B16")
	g.f("VORR V20.B16, V16.B16, V16.B16")
	g.f("VMOV V16.D[0], R11")
	g.f("VMOV V16.D[1], R12")
	g.f("ORR R12, R11, R11")
	g.f("CBZ R11, dc")
	g.f("VMOV R10, V0.S[0]")
	g.f("ADDW $32, R10, R10")
	g.f("VMOV R10, V0.S[0]")
	// the first four of the row-major 2x(4x4) quadrants
	rowLo := func(r int) string { return v(2 * r) }
	rowHi := func(r int) string { return v(2*r + 1) }
	quad := func(f func(int) string, r0 int) [4]string {
		return [4]string{f(r0), f(r0 + 1), f(r0 + 2), f(r0 + 3)}
	}
	tmp := [4]string{"V16", "V17", "V18", "V19"}
	for _, q := range [][4]string{quad(rowLo, 0), quad(rowHi, 0), quad(rowLo, 4), quad(rowHi, 4)} {
		g.transpose4n(q, tmp)
	}
	// set A: rows 0-3 of every column; set B: rows 4-7
	sA := [8]string{"V0", "V2", "V4", "V6", "V1", "V3", "V5", "V7"}
	sB := [8]string{"V8", "V10", "V12", "V14", "V9", "V11", "V13", "V15"}
	tA := [8]string{"V16", "V17", "V18", "V19", "V20", "V21", "V22", "V23"}
	tB := [8]string{"V24", "V25", "V26", "V27", "V28", "V29", "V30", "V31"}
	oA, fA := g.idct8n(sA, tA)
	oB, fB := g.idct8n(sB, tB)
	// transpose back: the four 4x4 pieces of (A, B) x (columns 0-3, 4-7)
	free := append(append([]string{}, fA[:]...), fB[:]...)
	ft := [4]string{free[0], free[1], free[2], free[3]}
	g.transpose4n([4]string{oA[0], oA[1], oA[2], oA[3]}, ft)
	g.transpose4n([4]string{oA[4], oA[5], oA[6], oA[7]}, ft)
	g.transpose4n([4]string{oB[0], oB[1], oB[2], oB[3]}, ft)
	g.transpose4n([4]string{oB[4], oB[5], oB[6], oB[7]}, ft)
	// rows 0-7 low halves, then high halves
	loRows := [8]string{oA[0], oA[1], oA[2], oA[3], oB[0], oB[1], oB[2], oB[3]}
	hiRows := [8]string{oA[4], oA[5], oA[6], oA[7], oB[4], oB[5], oB[6], oB[7]}
	used := map[string]bool{}
	for _, r := range append(loRows[:], hiRows[:]...) {
		used[r] = true
	}
	var spare []string
	for i := 0; i < 32; i++ {
		if !used[v(i)] {
			spare = append(spare, v(i))
		}
	}
	tLo := [8]string(spare[:8])
	tHi := [8]string(spare[8:16])
	lo, _ := g.idct8n(loRows, tLo)
	hi, _ := g.idct8n(hiRows, tHi)
	// everything but the sixteen result registers is scratch for the add
	res := map[string]bool{}
	for _, r := range append(lo[:], hi[:]...) {
		res[r] = true
	}
	var scratch []string
	for i := 0; i < 32; i++ {
		if !res[v(i)] {
			scratch = append(scratch, v(i))
		}
	}
	for _, r := range append(lo[:], hi[:]...) {
		g.f("VSSHR $6, %s.S4, %s.S4", r, r)
	}
	g.add64(lo, hi, [8]string(scratch[:8]))
	g.f("RET")
	g.l("dc:")
	g.f("ADDW $32, R10, R10")
	g.f("SXTW R10, R10")
	g.f("ASR $6, R10, R10")
	g.f("VDUP R10, V0.S4")
	same := [8]string{"V0", "V0", "V0", "V0", "V0", "V0", "V0", "V0"}
	g.add64(same, same, [8]string{"V16", "V17", "V18", "V19", "V20", "V21", "V22", "V23"})
	g.f("RET")
	return g.b.String()
}

func files() map[string]string {
	var g gen
	return map[string]string{
		"itx/idct8_amd64.s": g.amd64(),
		"itx/idct8_arm64.s": g.arm64(),
		"itx/idct4_amd64.s": g.amd64x4(),
		"itx/idct4_arm64.s": g.arm64x4(),
	}
}

func run(root string) error {
	for name, body := range files() {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	dir := "."
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	if err := run(dir); err != nil {
		fmt.Fprintln(os.Stderr, "genitx:", err)
		osExit(1)
	}
}
