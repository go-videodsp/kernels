package main

import (
	"fmt"
	"strings"
)

// NEON luma deblocking (ITU-T H.264 (V16) (06/2026), clauses 8.7.2.2 to
// 8.7.2.4), sixteen lines at a time: the eight samples p3..q3 across the edge
// sit in V0..V7, sixteen lines per register. A horizontal edge loads them as
// eight rows; a vertical edge loads sixteen rows of eight bytes and transposes.

type asm struct{ b strings.Builder }

func (a *asm) f(format string, args ...any) { fmt.Fprintf(&a.b, "\t"+format+"\n", args...) }

// transpose8 transposes the 8x8 bytes held in the low halves of in[0..7]
// (row r in in[r]) into c[0..7] (column i in c[i]), through t and u.
func (a *asm) transpose8(in, t, u, c [8]int) {
	for j := 0; j < 4; j++ {
		a.f("VTRN1 V%d.B8, V%d.B8, V%d.B8", in[2*j+1], in[2*j], t[2*j])
		a.f("VTRN2 V%d.B8, V%d.B8, V%d.B8", in[2*j+1], in[2*j], t[2*j+1])
	}
	h := func(x, y, d1, d2 int) {
		a.f("VTRN1 V%d.H4, V%d.H4, V%d.H4", t[y], t[x], u[d1])
		a.f("VTRN2 V%d.H4, V%d.H4, V%d.H4", t[y], t[x], u[d2])
	}
	h(0, 2, 0, 2)
	h(1, 3, 1, 3)
	h(4, 6, 4, 6)
	h(5, 7, 5, 7)
	w := func(x, y, d1, d2 int) {
		a.f("VTRN1 V%d.S2, V%d.S2, V%d.S2", u[y], u[x], c[d1])
		a.f("VTRN2 V%d.S2, V%d.S2, V%d.S2", u[y], u[x], c[d2])
	}
	w(0, 4, 0, 4)
	w(1, 5, 1, 5)
	w(2, 6, 2, 6)
	w(3, 7, 3, 7)
}

func seq(from int) (r [8]int) {
	for i := range r {
		r[i] = from + i
	}
	return
}

// expand turns the four bytes of the 32-bit value in V<n>.S[0] into sixteen,
// each repeated for the four lines of its group.
func (a *asm) expand(n int) {
	a.f("VZIP1 V%d.B16, V%d.B16, V%d.B16", n, n, n)
	a.f("VZIP1 V%d.H8, V%d.H8, V%d.H8", n, n, n)
}

// absdiff leaves |x - y| (unsigned bytes) in V8, through V9.
func (a *asm) absdiff(x, y int) {
	a.f("VUQSUB V%d.B16, V%d.B16, V8.B16", y, x)
	a.f("VUQSUB V%d.B16, V%d.B16, V9.B16", x, y)
	a.f("VORR V9.B16, V8.B16, V8.B16")
}

// lessThan leaves in dst the mask (|x - y| < limit) where limit is a byte
// vector register: all ones where true.
func (a *asm) lessThan(x, y, limit, dst int) {
	a.absdiff(x, y)
	a.f("VCMHI V8.B16, V%d.B16, V%d.B16", limit, dst)
}

// widen puts the low (hi false) or high (hi true) eight bytes of V<src> into
// V<dst> as unsigned sixteen-bit lanes.
func (a *asm) widen(src, dst int, hi bool) {
	if hi {
		a.f("VUXTL2 V%d.B16, V%d.H8", src, dst)
	} else {
		a.f("VUXTL V%d.B8, V%d.H8", src, dst)
	}
}

// narrowSat puts V<src> (signed sixteen-bit lanes) into the low or high
// eight bytes of V<dst>, saturated to 0..255.
func (a *asm) narrowSat(src, dst int, hi bool) {
	if hi {
		a.f("VSQXTUN2 V%d.H8, V%d.B16", src, dst)
	} else {
		a.f("VSQXTUN V%d.H8, V%d.B8", src, dst)
	}
}

// narrowShift puts V<src> >> n into the low or high eight bytes of V<dst>.
func (a *asm) narrowShift(n, src, dst int, hi bool) {
	if hi {
		a.f("VSHRN2 $%d, V%d.H8, V%d.B16", n, src, dst)
	} else {
		a.f("VSHRN $%d, V%d.H8, V%d.B8", n, src, dst)
	}
}

// normalCore is clause 8.7.2.3 on V0..V7. V20 = alpha, V21 = beta (bytes),
// V22 = tC0 per line, V23 = 0xff on the lines whose bS is not 0, V24 = 4
// (sixteen-bit lanes). It rewrites V2..V5.
func (a *asm) normalCore() {
	a.lessThan(3, 4, 20, 10) // alpha > |p0 - q0|
	a.lessThan(2, 3, 21, 11) // beta > |p1 - p0|
	a.lessThan(5, 4, 21, 12) // beta > |q1 - q0|
	a.f("VAND V11.B16, V10.B16, V10.B16")
	a.f("VAND V12.B16, V10.B16, V10.B16")
	a.f("VAND V23.B16, V10.B16, V10.B16") // filterSamplesFlag, 8-460
	a.lessThan(1, 3, 21, 13)              // ap < beta
	a.lessThan(6, 4, 21, 14)              // aq < beta
	a.f("VSUB V13.B16, V22.B16, V15.B16") // tC = tC0 + (ap<beta) + (aq<beta), 8-465
	a.f("VSUB V14.B16, V15.B16, V15.B16")
	a.f("VURHADD V4.B16, V3.B16, V16.B16") // (p0 + q0 + 1) >> 1
	for _, hi := range []bool{false, true} {
		a.widen(3, 25, hi) // p0
		a.widen(4, 26, hi) // q0
		a.widen(2, 27, hi) // p1
		a.widen(5, 28, hi) // q1
		// delta = Clip3(-tC, tC, (((q0 - p0) << 2) + (p1 - q1) + 4) >> 3), 8-467
		a.f("VSUB V25.H8, V26.H8, V29.H8")
		a.f("VADD V29.H8, V29.H8, V29.H8")
		a.f("VADD V29.H8, V29.H8, V29.H8")
		a.f("VSUB V28.H8, V27.H8, V30.H8")
		a.f("VADD V30.H8, V29.H8, V29.H8")
		a.f("VADD V24.H8, V29.H8, V29.H8")
		a.f("VSSHR $3, V29.H8, V29.H8")
		a.widen(15, 30, hi)
		a.f("VNEG V30.H8, V31.H8")
		a.f("VSMIN V30.H8, V29.H8, V29.H8")
		a.f("VSMAX V31.H8, V29.H8, V29.H8")
		a.f("VADD V29.H8, V25.H8, V8.H8") // p0 + delta, 8-468
		a.narrowSat(8, 17, hi)
		a.f("VSUB V29.H8, V26.H8, V8.H8") // q0 - delta, 8-469
		a.narrowSat(8, 18, hi)
		// p1' = p1 + Clip3(-tC0, tC0, (p2 + avg - (p1 << 1)) >> 1), 8-470
		a.widen(22, 30, hi)
		a.f("VNEG V30.H8, V31.H8")
		a.widen(1, 8, hi)
		a.widen(16, 9, hi)
		a.f("VADD V9.H8, V8.H8, V8.H8")
		a.f("VSUB V27.H8, V8.H8, V8.H8")
		a.f("VSUB V27.H8, V8.H8, V8.H8")
		a.f("VSSHR $1, V8.H8, V8.H8")
		a.f("VSMIN V30.H8, V8.H8, V8.H8")
		a.f("VSMAX V31.H8, V8.H8, V8.H8")
		a.f("VADD V27.H8, V8.H8, V8.H8")
		a.narrowSat(8, 19, hi)
		// q1', 8-472
		a.widen(6, 8, hi)
		a.f("VADD V9.H8, V8.H8, V8.H8")
		a.f("VSUB V28.H8, V8.H8, V8.H8")
		a.f("VSUB V28.H8, V8.H8, V8.H8")
		a.f("VSSHR $1, V8.H8, V8.H8")
		a.f("VSMIN V30.H8, V8.H8, V8.H8")
		a.f("VSMAX V31.H8, V8.H8, V8.H8")
		a.f("VADD V28.H8, V8.H8, V8.H8")
		a.narrowSat(8, 12, hi)
	}
	a.f("VAND V10.B16, V13.B16, V13.B16") // filter and ap < beta
	a.f("VAND V10.B16, V14.B16, V14.B16") // filter and aq < beta
	a.f("VBIT V10.B16, V17.B16, V3.B16")
	a.f("VBIT V10.B16, V18.B16, V4.B16")
	a.f("VBIT V13.B16, V19.B16, V2.B16")
	a.f("VBIT V14.B16, V12.B16, V5.B16")
}

// strongSide computes one side of clause 8.7.2.4 into the pixel registers
// x2, x1, x0 given the registers of x3, x2, x1, x0, y0, y1 (byte vectors),
// the mask F in V10 and the side's mask in sm. The results go through
// V11, V12, V15, V16 and V17 is a spare.
func (a *asm) strongSide(x3, x2, x1, x0, y0, y1, sm int) {
	for _, hi := range []bool{false, true} {
		a.widen(x3, 24, hi)
		a.widen(x2, 25, hi)
		a.widen(x1, 26, hi)
		a.widen(x0, 27, hi)
		a.widen(y0, 28, hi)
		a.widen(y1, 29, hi)
		// T = x1 + x0 + y0
		a.f("VADD V27.H8, V26.H8, V30.H8")
		a.f("VADD V28.H8, V30.H8, V30.H8")
		// x1' = (x2 + T + 2) >> 2, 8-478
		a.f("VADD V25.H8, V30.H8, V31.H8")
		a.f("VADD V23.H8, V31.H8, V31.H8")
		a.narrowShift(2, 31, 12, hi)
		// x0' = (x2 + 2T + y1 + 4) >> 3, 8-477
		a.f("VADD V30.H8, V30.H8, V31.H8")
		a.f("VADD V25.H8, V31.H8, V31.H8")
		a.f("VADD V29.H8, V31.H8, V31.H8")
		a.f("VADD V19.H8, V31.H8, V31.H8")
		a.narrowShift(3, 31, 15, hi)
		// x2' = (2*x3 + 3*x2 + T + 4) >> 3, 8-479
		a.f("VADD V24.H8, V24.H8, V31.H8")
		a.f("VADD V25.H8, V31.H8, V31.H8")
		a.f("VADD V25.H8, V31.H8, V31.H8")
		a.f("VADD V25.H8, V31.H8, V31.H8")
		a.f("VADD V30.H8, V31.H8, V31.H8")
		a.f("VADD V19.H8, V31.H8, V31.H8")
		a.narrowShift(3, 31, 11, hi)
		// weak x0' = (2*x1 + x0 + y1 + 2) >> 2, 8-480
		a.f("VADD V26.H8, V26.H8, V31.H8")
		a.f("VADD V27.H8, V31.H8, V31.H8")
		a.f("VADD V29.H8, V31.H8, V31.H8")
		a.f("VADD V23.H8, V31.H8, V31.H8")
		a.narrowShift(2, 31, 16, hi)
	}
	a.f("VBIT V10.B16, V16.B16, V%d.B16", x0) // filtered: weak form
	a.f("VBIT V%d.B16, V15.B16, V%d.B16", sm, x0)
	a.f("VBIT V%d.B16, V12.B16, V%d.B16", sm, x1)
	a.f("VBIT V%d.B16, V11.B16, V%d.B16", sm, x2)
}

// strongCore is clause 8.7.2.4 on V0..V7. V20 = alpha, V21 = beta, V22 = (alpha >> 2) + 2
// (bytes), V19 = 4 and V23 = 2 (sixteen-bit lanes). It rewrites V1..V6.
func (a *asm) strongCore() {
	a.lessThan(3, 4, 20, 10)              // alpha > |p0 - q0|
	a.f("VCMHI V8.B16, V22.B16, V18.B16") // (alpha >> 2) + 2 > |p0 - q0|
	a.lessThan(2, 3, 21, 11)
	a.lessThan(5, 4, 21, 12)
	a.f("VAND V11.B16, V10.B16, V10.B16")
	a.f("VAND V12.B16, V10.B16, V10.B16") // filterSamplesFlag, 8-460
	a.lessThan(1, 3, 21, 13)
	a.lessThan(6, 4, 21, 14)
	a.f("VAND V10.B16, V13.B16, V13.B16")
	a.f("VAND V18.B16, V13.B16, V13.B16") // 8-476
	a.f("VAND V10.B16, V14.B16, V14.B16")
	a.f("VAND V18.B16, V14.B16, V14.B16") // 8-483
	a.f("VMOV V3.B16, V8.B16")            // original p0 and p1: the q side reads them
	a.f("VMOV V2.B16, V9.B16")
	a.strongSide(0, 1, 2, 3, 4, 5, 13)
	a.strongSide(7, 6, 5, 4, 8, 9, 14)
}

func (a *asm) loadRows(first, n int, base string) {
	for i := 0; i < n; i++ {
		a.f("VLD1 (%s), [V%d.B16]", base, first+i)
		a.f("ADD R1, %s", base)
	}
}

func (a *asm) storeRows(first, n int, base string) {
	for i := 0; i < n; i++ {
		a.f("VST1 [V%d.B16], (%s)", first+i, base)
		a.f("ADD R1, %s", base)
	}
}

// header for the four entry points: pix in R0, stride in R1, alpha R4, beta R5.
func deblockFunc(name string, strong, vertical bool) string {
	var a asm
	if strong {
		fmt.Fprintf(&a.b, "// func %s(pix *byte, stride int, alpha, beta, thr int)\n", name)
	} else {
		fmt.Fprintf(&a.b, "// func %s(pix *byte, stride int, alpha, beta int, tc0, mask uint32)\n", name)
	}
	fmt.Fprintf(&a.b, "TEXT ·%s(SB), NOSPLIT, $0-40\n", name)
	a.f("MOVD pix+0(FP), R0")
	a.f("MOVD stride+8(FP), R1")
	a.f("MOVD alpha+16(FP), R4")
	a.f("MOVD beta+24(FP), R5")
	if strong {
		a.f("MOVD thr+32(FP), R6")
	} else {
		a.f("MOVWU tc0+32(FP), R6")
		a.f("MOVWU mask+36(FP), R7")
	}

	if vertical {
		// 16 rows of 8 bytes at pix - 4, transposed into V0..V7.
		a.f("SUB $4, R0, R2")
		for i := 0; i < 8; i++ {
			a.f("FMOVD (R2), F%d", i)
			a.f("ADD R1, R2")
		}
		a.transpose8(seq(0), seq(8), seq(0), seq(8))
		for i := 0; i < 8; i++ {
			a.f("FMOVD (R2), F%d", i)
			a.f("ADD R1, R2")
		}
		a.transpose8(seq(0), seq(16), seq(24), seq(16))
		// Hold the first half's columns while V8..V15 are free again.
		for i := 0; i < 8; i++ {
			a.f("VZIP1 V%d.D2, V%d.D2, V%d.D2", 16+i, 8+i, i)
		}
	} else {
		a.f("LSL $2, R1, R8")
		a.f("SUB R8, R0, R2")
		a.loadRows(0, 8, "R2")
	}

	a.f("VDUP R4, V20.B16")
	a.f("VDUP R5, V21.B16")
	if strong {
		a.f("VDUP R6, V22.B16")
		a.f("MOVD $4, R8")
		a.f("VDUP R8, V19.H8")
		a.f("MOVD $2, R8")
		a.f("VDUP R8, V23.H8")
		a.strongCore()
	} else {
		a.f("VMOV R6, V22.S[0]")
		a.expand(22)
		a.f("VMOV R7, V23.S[0]")
		a.expand(23)
		a.f("MOVD $4, R8")
		a.f("VDUP R8, V24.H8")
		a.normalCore()
	}

	if vertical {
		// Back to rows: the low halves of V0..V7 are lines 0..7, the high
		// halves lines 8..15.
		for i := 0; i < 8; i++ {
			a.f("VZIP2 V%d.D2, V%d.D2, V%d.D2", i, i, 16+i)
		}
		a.transpose8(seq(0), seq(8), seq(0), seq(8))
		a.f("SUB $4, R0, R2")
		for i := 0; i < 8; i++ {
			a.f("FMOVD F%d, (R2)", 8+i)
			a.f("ADD R1, R2")
		}
		a.transpose8(seq(16), seq(0), seq(24), seq(16))
		for i := 0; i < 8; i++ {
			a.f("FMOVD F%d, (R2)", 16+i)
			a.f("ADD R1, R2")
		}
	} else {
		a.f("LSL $2, R1, R8")
		a.f("SUB R8, R0, R2")
		a.storeRows(0, 8, "R2")
	}
	a.f("RET")
	return a.b.String() + "\n"
}

func generateDeblock() string {
	var s strings.Builder
	s.WriteString(header)
	s.WriteString("//go:build arm64 && !purego\n\n#include \"textflag.h\"\n\n")
	s.WriteString("// H.264 luma deblocking, NEON: sixteen lines per call, the eight samples\n")
	s.WriteString("// across the edge in V0..V7. See internal/genmc/deblock.go.\n\n")
	s.WriteString(deblockFunc("deblockNormalH", false, false))
	s.WriteString(deblockFunc("deblockNormalV", false, true))
	s.WriteString(deblockFunc("deblockStrongH", true, false))
	s.WriteString(deblockFunc("deblockStrongV", true, true))
	return s.String()
}
