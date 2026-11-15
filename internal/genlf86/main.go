// Command genlf86 writes the amd64 AVX2 assembly for H.264 luma deblocking
// (package lf) and the Go declarations that go with it.
//
// Run from the repository root: go run ./internal/genlf86
//
// The kernels are written from ITU-T H.264 (V16) (06/2026), clauses 8.7.2.2 to
// 8.7.2.4 (Equations 8-451 to 8-489), not from any decoder.
//
// One call filters the sixteen lines of an edge, one line per sixteen-bit
// lane of a YMM register, so every equation is evaluated for all lines at
// once and the filterSamplesFlag, ap < beta and aq < beta conditions become
// lane masks:
//
//   - |a - b| < t is VPCMPGTW (t, |a - b|) on VPABSW (VPSUBW a b); samples
//     are 0..255, so the signed compares and the sixteen-bit sums (at most
//     8 * 255) are exact;
//   - bS < 4 (8.7.2.3): tc = tc0 + (ap < beta) + (aq < beta) is tc0 minus the
//     two masks (each is -1); delta is Clip3(-tc, tc, ((q0 - p0) << 2 +
//     (p1 - q1) + 4) >> 3); the p1 and q1 corrections use VPAVGW, which is
//     (p0 + q0 + 1) >> 1; p0 and q0 saturate in the final pack, which is the
//     Clip1;
//   - bS = 4 (8.7.2.4): the three strong equations per side are sums of the
//     samples, selected by VPBLENDVB on the masks ap < beta && |p0 - q0| <
//     (alpha >> 2) + 2 and filterSamplesFlag. The two sides run as two passes
//     that read the original samples, and the results are stored at the end.
//
// A horizontal edge reads each of its rows with one VPMOVZXBW (sixteen
// adjacent samples). A vertical edge is transposed first: sixteen rows of
// eight bytes become eight rows of sixteen lines in a stack scratch, the same
// code filters the scratch, and the eight rows are transposed back.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type gen struct {
	b strings.Builder
	// row returns the memory operand of sample row i, p3 = 0 .. q3 = 7, of the
	// sixteen lines.
	row func(i int) string
}

func (g *gen) f(format string, a ...any) { fmt.Fprintf(&g.b, "\t"+format+"\n", a...) }

func hRow(i int) string {
	return [8]string{"(DI)(R12*1)", "(DI)(R11*1)", "(DI)(R10*1)", "(DI)(R9*1)", "(DI)", "(DI)(DX*1)", "(DI)(DX*2)", "(DI)(R13*1)"}[i]
}

func vRow(i int) string { return fmt.Sprintf("%d(R14)", 16*i) }

// ld loads sample row i, zero-extended to sixteen-bit lanes, into Y(n).
func (g *gen) ld(n, i int) { g.f("VPMOVZXBW %s, Y%d", g.row(i), n) }

// st stores the sixteen bytes of X(n) to sample row i.
func (g *gen) st(n, i int) { g.f("VMOVDQU X%d, %s", n, g.row(i)) }

// stHi stores the high half of Y(n) to sample row i.
func (g *gen) stHi(n, i int) { g.f("VEXTRACTI128 $1, Y%d, %s", n, g.row(i)) }

// absLess writes into Y(dst) the mask (|Y(a) - Y(b)| < Y(t)), using Y(tmp).
func (g *gen) absLess(dst, a, b, t, tmp int) {
	g.f("VPSUBW Y%d, Y%d, Y%d", b, a, tmp)
	g.f("VPABSW Y%d, Y%d", tmp, tmp)
	g.f("VPCMPGTW Y%d, Y%d, Y%d", tmp, t, dst)
}

// packPair packs Y(a) (first) and Y(b) (second) to bytes with unsigned
// saturation into the low and high halves of Y(dst): [a | b], sixteen bytes
// each.
func (g *gen) packPair(dst, a, b int) {
	g.f("VPACKUSWB Y%d, Y%d, Y%d", b, a, dst)
	g.f("VPERMQ $0xD8, Y%d, Y%d", dst, dst)
}

// packOne packs Y(a) to the sixteen bytes of X(dst).
func (g *gen) packOne(dst, a int) {
	g.f("VPACKUSWB Y%d, Y%d, Y%d", a, a, dst)
	g.f("VPERMQ $0x08, Y%d, Y%d", dst, dst)
}

// normalCore is clause 8.7.2.3 on the rows of g.row. Y6 alpha, Y7 beta, Y8
// tc0 (one word per line), Y9 the filterSamplesFlag lanes of bS != 0.
func (g *gen) normalCore() {
	// p2 Y0, p1 Y1, p0 Y2, q0 Y3, q1 Y4, q2 Y5.
	for n := 0; n < 6; n++ {
		g.ld(n, n+1)
	}
	g.absLess(12, 2, 3, 6, 11) // |p0-q0| < alpha
	g.absLess(13, 1, 2, 7, 11) // |p1-p0| < beta
	g.f("VPAND Y13, Y12, Y12")
	g.absLess(13, 4, 3, 7, 11) // |q1-q0| < beta
	g.f("VPAND Y13, Y12, Y12")
	g.f("VPAND Y9, Y12, Y12")  // Y12 = filterSamplesFlag lanes
	g.absLess(13, 0, 2, 7, 11) // ap < beta (8-463)
	g.absLess(14, 5, 3, 7, 11) // aq < beta (8-464)
	g.f("VPSUBW Y13, Y8, Y15") // tc = tc0 + (ap<beta) + (aq<beta) (8-465)
	g.f("VPSUBW Y14, Y15, Y15")
	// delta (8-467)
	g.f("VPSUBW Y2, Y3, Y11")
	g.f("VPSLLW $2, Y11, Y11")
	g.f("VPSUBW Y4, Y1, Y9")
	g.f("VPADDW Y9, Y11, Y11")
	g.f("VPADDW lfc4<>(SB), Y11, Y11")
	g.f("VPSRAW $3, Y11, Y11")
	g.f("VPXOR Y9, Y9, Y9")
	g.f("VPSUBW Y15, Y9, Y9") // -tc
	g.f("VPMINSW Y15, Y11, Y11")
	g.f("VPMAXSW Y9, Y11, Y11")
	g.f("VPAND Y12, Y11, Y11")
	g.f("VPADDW Y11, Y2, Y9")  // p0 + delta (8-468), clipped by the pack
	g.f("VPSUBW Y11, Y3, Y11") // q0 - delta (8-469)
	// p1 (8-470 and 8-471)
	g.f("VPAVGW Y3, Y2, Y15") // (p0 + q0 + 1) >> 1
	g.f("VPADDW Y15, Y0, Y15")
	g.f("VPADDW Y1, Y1, Y10")
	g.f("VPSUBW Y10, Y15, Y15")
	g.f("VPSRAW $1, Y15, Y15")
	g.f("VPXOR Y10, Y10, Y10")
	g.f("VPSUBW Y8, Y10, Y10") // -tc0
	g.f("VPMINSW Y8, Y15, Y15")
	g.f("VPMAXSW Y10, Y15, Y15")
	g.f("VPAND Y12, Y13, Y13") // ap && filterSamplesFlag
	g.f("VPAND Y13, Y15, Y15")
	g.f("VPADDW Y15, Y1, Y15") // p1'
	// q1 (8-472 and 8-473)
	g.f("VPAVGW Y3, Y2, Y1")
	g.f("VPADDW Y1, Y5, Y1")
	g.f("VPADDW Y4, Y4, Y13")
	g.f("VPSUBW Y13, Y1, Y1")
	g.f("VPSRAW $1, Y1, Y1")
	g.f("VPMINSW Y8, Y1, Y1")
	g.f("VPMAXSW Y10, Y1, Y1")
	g.f("VPAND Y12, Y14, Y14")
	g.f("VPAND Y14, Y1, Y1")
	g.f("VPADDW Y1, Y4, Y1") // q1'
	// p1' Y15, p0' Y9, q0' Y11, q1' Y1
	g.packPair(2, 15, 9)
	g.st(2, 2)
	g.stHi(2, 3)
	g.packPair(3, 11, 1)
	g.st(3, 4)
	g.stHi(3, 5)
}

// strongPass is the filter of clause 8.7.2.4 for one side: p when side is 0
// (rows p3..q1 in Y0..Y5, result rows p2, p1, p0), q when side is 1 (rows
// q3..p1, result rows q2, q1, q0). Y8 alpha, Y9 beta, Y10 alpha >> 2 + 2. The
// packed results go to Y(out) (the two outer rows, [first | second] in memory
// order) and Y(out+1) (the edge row).
func (g *gen) strongPass(side, out int) {
	var rows [6]int
	if side == 0 {
		rows = [6]int{0, 1, 2, 3, 4, 5} // p3 p2 p1 p0 q0 q1
	} else {
		rows = [6]int{7, 6, 5, 4, 3, 2} // q3 q2 q1 q0 p0 p1
	}
	for n := 0; n < 6; n++ {
		g.ld(n, rows[n])
	}
	// Y0 = x3, Y1 = x2, Y2 = x1, Y3 = x0 (this side), Y4 = y0, Y5 = y1 (the other).
	g.f("VPSUBW Y4, Y3, Y12")
	g.f("VPABSW Y12, Y12")
	g.f("VPCMPGTW Y12, Y8, Y11")  // |p0-q0| < alpha
	g.f("VPCMPGTW Y12, Y10, Y13") // |p0-q0| < (alpha>>2)+2
	g.absLess(14, 2, 3, 9, 12)    // |x1-x0| < beta
	g.f("VPAND Y14, Y11, Y11")
	g.absLess(14, 5, 4, 9, 12) // |y1-y0| < beta
	g.f("VPAND Y14, Y11, Y11") // Y11 = filterSamplesFlag (8-460)
	g.absLess(14, 1, 3, 9, 12) // ax < beta
	g.f("VPAND Y13, Y14, Y14")
	g.f("VPAND Y11, Y14, Y14") // Y14 = strong lanes (8-476 or 8-483)
	// Strong x0 = (x2 + 2*x1 + 2*x0 + 2*y0 + y1 + 4) >> 3
	g.f("VPADDW Y4, Y3, Y12") // s = x0 + y0
	g.f("VPADDW Y2, Y12, Y13")
	g.f("VPADDW Y13, Y13, Y13")
	g.f("VPADDW Y1, Y13, Y13")
	g.f("VPADDW Y5, Y13, Y13")
	g.f("VPADDW lfc4<>(SB), Y13, Y13")
	g.f("VPSRAW $3, Y13, Y13") // Y13 = strong x0
	// Strong x1 = (x2 + x1 + s + 2) >> 2, x2' = (2*(x3 + x2) + x2 + x1 + s + 4) >> 3
	g.f("VPADDW Y1, Y2, Y15")
	g.f("VPADDW Y12, Y15, Y15")
	g.f("VPADDW lfc2<>(SB), Y15, Y15") // x2 + x1 + s + 2
	g.f("VPADDW Y1, Y0, Y12")
	g.f("VPADDW Y12, Y12, Y12")
	g.f("VPADDW Y15, Y12, Y12")
	g.f("VPADDW lfc2<>(SB), Y12, Y12")
	g.f("VPSRAW $3, Y12, Y12") // Y12 = strong x2
	g.f("VPSRAW $2, Y15, Y15") // Y15 = strong x1
	// Weak x0 = (2*x1 + x0 + y1 + 2) >> 2 (8-480)
	g.f("VPADDW Y2, Y2, Y0")
	g.f("VPADDW Y3, Y0, Y0")
	g.f("VPADDW Y5, Y0, Y0")
	g.f("VPADDW lfc2<>(SB), Y0, Y0")
	g.f("VPSRAW $2, Y0, Y0")
	g.f("VPBLENDVB Y14, Y13, Y0, Y0") // strong ? strong x0 : weak x0
	g.f("VPBLENDVB Y11, Y0, Y3, Y3")  // filterSamplesFlag ? x0' : x0
	g.f("VPBLENDVB Y14, Y15, Y2, Y2") // x1'
	g.f("VPBLENDVB Y14, Y12, Y1, Y1") // x2'
	if side == 0 {
		g.packPair(out, 1, 2) // [p2' | p1']
	} else {
		g.packPair(out, 2, 1) // [q1' | q2']
	}
	g.packOne(out+1, 3)
}

// strongCore runs both sides and stores the six rows. The first side's packed
// results wait in Y6 and Y7 while the second side runs.
func (g *gen) strongCore() {
	g.strongPass(0, 6)
	g.strongPass(1, 2)
	// Y6 = [p2' | p1'], Y7 = p0', Y2 = [q1' | q2'], Y3 = q0'
	g.st(6, 1)
	g.stHi(6, 2)
	g.st(7, 3)
	g.st(3, 4)
	g.st(2, 5)
	g.stHi(2, 6)
}

func (g *gen) prologueH() {
	g.f("MOVQ pix+0(FP), DI")
	g.f("MOVQ stride+8(FP), DX")
	g.f("MOVQ DX, R9")
	g.f("NEGQ R9")
	g.f("LEAQ (R9)(R9*1), R10")
	g.f("LEAQ (R10)(R9*1), R11")
	g.f("LEAQ (R10)(R10*1), R12")
	g.f("LEAQ (DX)(DX*2), R13")
}

// bcast broadcasts the 16-bit value of the argument into Y(n).
func (g *gen) bcast(arg string, n int) {
	g.f("MOVQ %s, AX", arg)
	g.f("VMOVD AX, X%d", n)
	g.f("VPBROADCASTW X%d, Y%d", n, n)
}

// perLine expands the four bytes of a 32-bit argument (one per group of four
// lines) to the sixteen lines and to sixteen-bit lanes in Y(n): sign-extended
// (the 0xff masks) or zero-extended (tc0).
func (g *gen) perLine(arg string, n int, signed bool) {
	g.f("MOVL %s, AX", arg)
	g.f("VMOVD AX, X%d", n)
	g.f("VPSHUFB lfshuf<>(SB), X%d, X%d", n, n)
	if signed {
		g.f("VPMOVSXBW X%d, Y%d", n, n)
	} else {
		g.f("VPMOVZXBW X%d, Y%d", n, n)
	}
}

func (g *gen) prologueNormal() {
	g.bcast("alpha+16(FP)", 6)
	g.bcast("beta+24(FP)", 7)
	g.perLine("tc0+32(FP)", 8, false)
	g.perLine("mask+36(FP)", 9, true)
}

func (g *gen) prologueStrong() {
	g.bcast("alpha+16(FP)", 8)
	g.bcast("beta+24(FP)", 9)
	g.bcast("thr+32(FP)", 10)
}

// transposeIn loads sixteen rows of eight bytes (base R8 = row 0 at column -4,
// R9, R10, R11 the rows 4, 8, 12, R13 = 3*stride) and writes the eight
// columns, sixteen lines each, to the scratch at R14.
func (g *gen) transposeIn() {
	sub := func(base string, k int) string {
		return [4]string{"(" + base + ")", "(" + base + ")(DX*1)", "(" + base + ")(DX*2)", "(" + base + ")(R13*1)"}[k]
	}
	for k := 0; k < 8; k++ {
		lo, hi := "R8", "R10"
		if k >= 4 {
			lo, hi = "R9", "R11"
		}
		g.f("VMOVQ %s, X%d", sub(lo, k%4), k)
		g.f("VPINSRQ $1, %s, X%d, X%d", sub(hi, k%4), k, k)
	}
	// Stage 1, bytes: rows (0,1) (2,3) (4,5) (6,7); low halves are lines 0-7
	// and high halves lines 8-15.
	for i := 0; i < 4; i++ {
		a, b, lo := 2*i, 2*i+1, 8+i
		g.f("VPUNPCKLBW X%d, X%d, X%d", b, a, lo)
		g.f("VPUNPCKHBW X%d, X%d, X%d", b, a, a)
	}
	// Now L01 X8, L23 X9, L45 X10, L67 X11 and H01 X0, H23 X2, H45 X4, H67 X6.
	// Stage 2, words.
	g.f("VPUNPCKLWD X9, X8, X1")   // lines 0-3 of columns 0-3
	g.f("VPUNPCKHWD X9, X8, X8")   // lines 4-7
	g.f("VPUNPCKLWD X11, X10, X3") // lines 0-3 of columns 4-7
	g.f("VPUNPCKHWD X11, X10, X10")
	g.f("VPUNPCKLWD X2, X0, X5") // lines 8-11
	g.f("VPUNPCKHWD X2, X0, X0")
	g.f("VPUNPCKLWD X6, X4, X7")
	g.f("VPUNPCKHWD X6, X4, X4")
	// Stage 3, dwords: two lines of columns 0-7 side by side.
	g.f("VPUNPCKLDQ X3, X1, X9") // columns 0,1 of lines 0-7 ... see below
	g.f("VPUNPCKHDQ X3, X1, X1")
	g.f("VPUNPCKLDQ X10, X8, X3")
	g.f("VPUNPCKHDQ X10, X8, X8")
	g.f("VPUNPCKLDQ X7, X5, X10")
	g.f("VPUNPCKHDQ X7, X5, X5")
	g.f("VPUNPCKLDQ X4, X0, X7")
	g.f("VPUNPCKHDQ X4, X0, X0")
	// Stage 4, quadwords: lines 0-7 and 8-15 of one column side by side.
	g.f("VPUNPCKLQDQ X10, X9, X11") // column 0
	g.f("VPUNPCKHQDQ X10, X9, X9")  // column 1
	g.f("VPUNPCKLQDQ X5, X1, X10")  // column 2
	g.f("VPUNPCKHQDQ X5, X1, X1")   // column 3
	g.f("VPUNPCKLQDQ X7, X3, X5")   // column 4
	g.f("VPUNPCKHQDQ X7, X3, X3")   // column 5
	g.f("VPUNPCKLQDQ X0, X8, X7")   // column 6
	g.f("VPUNPCKHQDQ X0, X8, X8")   // column 7
	for i, r := range []int{11, 9, 10, 1, 5, 3, 7, 8} {
		g.f("VMOVDQU X%d, %d(R14)", r, 16*i)
	}
}

// transposeOut is the inverse: the eight rows of the scratch at R14 (columns
// of sixteen lines) are written as sixteen rows of eight bytes.
func (g *gen) transposeOut() {
	for i := 0; i < 8; i++ {
		g.f("VMOVDQU %d(R14), X%d", 16*i, i)
	}
	// Stage 1, bytes: columns (0,1) (2,3) (4,5) (6,7); the low unpack holds
	// lines 0-7 and the high one lines 8-15.
	for i := 0; i < 4; i++ {
		a, b, lo := 2*i, 2*i+1, 8+i
		g.f("VPUNPCKLBW X%d, X%d, X%d", b, a, lo)
		g.f("VPUNPCKHBW X%d, X%d, X%d", b, a, a)
	}
	g.f("VPUNPCKLWD X9, X8, X1") // lines 0-3, columns 0-3
	g.f("VPUNPCKHWD X9, X8, X8") // lines 4-7
	g.f("VPUNPCKLWD X11, X10, X3")
	g.f("VPUNPCKHWD X11, X10, X10")
	g.f("VPUNPCKLWD X2, X0, X5")
	g.f("VPUNPCKHWD X2, X0, X0")
	g.f("VPUNPCKLWD X6, X4, X7")
	g.f("VPUNPCKHWD X6, X4, X4")
	// Stage 3, dwords: each result holds two whole lines (eight bytes each).
	g.f("VPUNPCKLDQ X3, X1, X9")  // lines 0, 1
	g.f("VPUNPCKHDQ X3, X1, X1")  // lines 2, 3
	g.f("VPUNPCKLDQ X10, X8, X3") // lines 4, 5
	g.f("VPUNPCKHDQ X10, X8, X8") // lines 6, 7
	g.f("VPUNPCKLDQ X7, X5, X10") // lines 8, 9
	g.f("VPUNPCKHDQ X7, X5, X5")  // lines 10, 11
	g.f("VPUNPCKLDQ X4, X0, X7")  // lines 12, 13
	g.f("VPUNPCKHDQ X4, X0, X0")  // lines 14, 15
	sub := func(base string, k int) string {
		return [4]string{"(" + base + ")", "(" + base + ")(DX*1)", "(" + base + ")(DX*2)", "(" + base + ")(R13*1)"}[k]
	}
	for i, r := range []int{9, 1, 3, 8, 10, 5, 7, 0} {
		base := [4]string{"R8", "R9", "R10", "R11"}[i/2]
		g.f("VMOVQ X%d, %s", r, sub(base, 2*(i%2)))
		g.f("VPEXTRQ $1, X%d, %s", r, sub(base, 2*(i%2)+1))
	}
}

func (g *gen) prologueV() {
	g.f("MOVQ pix+0(FP), DI")
	g.f("MOVQ stride+8(FP), DX")
	g.f("LEAQ (DX)(DX*2), R13")
	g.f("LEAQ -4(DI), R8")
	g.f("LEAQ (R8)(DX*4), R9")
	g.f("LEAQ (R9)(DX*4), R10")
	g.f("LEAQ (R10)(DX*4), R11")
	g.f("MOVQ SP, R14")
}

const header = "// Code generated by internal/genlf86; DO NOT EDIT.\n\n"

// kernel names and signatures.
const (
	normalSig = "pix *byte, stride int, alpha, beta int, tc0, mask uint32"
	strongSig = "pix *byte, stride int, alpha, beta, thr int"
)

func (g *gen) kernel(name string, vertical, strong bool) string {
	g.b.Reset()
	frame, sig, args := 0, normalSig, 40
	if strong {
		sig = strongSig
	}
	flags := "NOSPLIT"
	if vertical {
		frame, flags = 128, "0"
		g.row = vRow
	} else {
		g.row = hRow
	}
	fmt.Fprintf(&g.b, "// func %s(%s)\n", name, sig)
	fmt.Fprintf(&g.b, "TEXT ·%s(SB), %s, $%d-%d\n", name, flags, frame, args)
	if vertical {
		g.prologueV()
	} else {
		g.prologueH()
	}
	if vertical {
		g.transposeIn() // before the constants: it uses X0..X11
	}
	if strong {
		g.prologueStrong()
	} else {
		g.prologueNormal()
	}
	if strong {
		g.strongCore()
	} else {
		g.normalCore()
	}
	if vertical {
		g.transposeOut()
	}
	g.f("VZEROUPPER")
	g.f("RET")
	g.b.WriteString("\n")
	return g.b.String()
}

func generate() (string, string) {
	var s, t strings.Builder
	s.WriteString(header)
	s.WriteString("//go:build amd64 && !purego\n\n#include \"textflag.h\"\n\n")
	// Per-line expansion of the four group bytes, and the small constants.
	s.WriteString("DATA lfshuf<>+0(SB)/8, $0x0101010100000000\n")
	s.WriteString("DATA lfshuf<>+8(SB)/8, $0x0303030302020202\n")
	s.WriteString("GLOBL lfshuf<>(SB), RODATA|NOPTR, $16\n\n")
	for _, c := range []struct {
		name string
		v    uint64
	}{{"lfc2", 0x0002000200020002}, {"lfc4", 0x0004000400040004}} {
		for o := 0; o < 32; o += 8 {
			fmt.Fprintf(&s, "DATA %s<>+%d(SB)/8, $0x%016x\n", c.name, o, c.v)
		}
		fmt.Fprintf(&s, "GLOBL %s<>(SB), RODATA|NOPTR, $32\n\n", c.name)
	}
	t.WriteString(header)
	t.WriteString("//go:build amd64 && !purego\n\npackage lf\n\n")
	g := &gen{}
	for _, k := range []struct {
		name             string
		vertical, strong bool
	}{
		{"deblockNormalH", false, false}, {"deblockNormalV", true, false},
		{"deblockStrongH", false, true}, {"deblockStrongV", true, true},
	} {
		s.WriteString(g.kernel(k.name, k.vertical, k.strong))
		sig := normalSig
		if k.strong {
			sig = strongSig
		}
		fmt.Fprintf(&t, "//go:noescape\nfunc %s(%s)\n\n", k.name, sig)
	}
	return s.String(), strings.TrimRight(t.String(), "\n") + "\n"
}

var osExit = os.Exit

// files maps each generated file, relative to the repository root, to its
// content.
func files() map[string]string {
	asm, decl := generate()
	return map[string]string{
		"lf/deblock_amd64.s":      asm,
		"lf/deblock_amd64_tab.go": decl,
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
		fmt.Fprintln(os.Stderr, "genlf86:", err)
		osExit(1)
	}
}
