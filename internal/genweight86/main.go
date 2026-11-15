// Command genweight86 writes the amd64 AVX2 assembly for H.264 weighted
// sample prediction (package mc: WeightUni and WeightBi) and the Go
// declarations that go with it.
//
// Run from the repository root: go run ./internal/genweight86
//
// The kernels are written from ITU-T H.264 (V16) (06/2026), clause 8.4.2.3.2,
// Equations 8-274 to 8-276, not from any decoder.
//
// Arithmetic:
//
//   - uni, Equations 8-274 and 8-275: with p in 0..255 and a weight in
//     -128..128, p*w + rnd lies in -32640..32704, and after the shift the
//     offset (-128..127) keeps the result in -32768..32767, so the whole
//     formula is exact in sixteen-bit lanes: PMULLW, PADDW rnd, PSRAW, PADDW
//     offset, then the unsigned saturating pack, which is the Clip1;
//   - bi, Equation 8-276: p0*w0 + p1*w1 needs seventeen bits, so the pair of
//     samples (p0, p1) is interleaved and multiplied by (w0, w1) with
//     PMADDWD into thirty-two-bit lanes. The offset o = (o0 + o1 + 1) >> 1 is
//     folded into the rounding constant, K = 2^logWD + (o << (logWD + 1)):
//     ((S + 2^logWD) >> (logWD + 1)) + o = (S + K) >> (logWD + 1) because
//     o << (logWD + 1) is a multiple of the divisor. The shifted value lies in
//     -32768..32767 (|S| <= 65280), so the signed pack to sixteen bits is
//     exact, then the unsigned pack clips.
//
// Rows are processed in groups that fill a vector: two rows of sixteen or
// eight samples, four rows of four (every supported height is a multiple of
// that), and a kernel reads and writes exactly the bytes of its block.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type gen struct {
	b strings.Builder
	w int // 4, 8 or 16
}

func (g *gen) f(format string, a ...any) { fmt.Fprintf(&g.b, "\t"+format+"\n", a...) }

// rows is how many block rows one iteration handles.
func (g *gen) rows() int {
	switch g.w {
	case 16, 8:
		return 2
	}
	return 4
}

// gather loads the rows of the block at base (stride in reg) that one
// iteration handles, zero-extended to sixteen-bit lanes in Y(n), through
// scratch registers tmp (a pointer) and X(t).
func (g *gen) gather(n int, base, stride, tmp string, t int) {
	switch g.w {
	case 16:
		g.f("VPMOVZXBW (%s), Y%d", base, n)
		g.f("VPMOVZXBW (%s)(%s*1), Y%d", base, stride, t)
	case 8:
		g.f("VMOVQ (%s), X%d", base, n)
		g.f("VPINSRQ $1, (%s)(%s*1), X%d, X%d", base, stride, n, n)
		g.f("VPMOVZXBW X%d, Y%d", n, n)
	default:
		g.f("VMOVD (%s), X%d", base, n)
		g.f("VPINSRD $1, (%s)(%s*1), X%d, X%d", base, stride, n, n)
		g.f("LEAQ (%s)(%s*2), %s", base, stride, tmp)
		g.f("VMOVD (%s), X%d", tmp, t)
		g.f("VPINSRD $1, (%s)(%s*1), X%d, X%d", tmp, stride, t, t)
		g.f("VPUNPCKLQDQ X%d, X%d, X%d", t, n, n)
		g.f("VPMOVZXBW X%d, Y%d", n, n)
	}
}

// scatter packs the sixteen-bit lanes (clipped to 0..255) and stores the rows
// to dst (stride DX). Width 16 has the two rows in Y0 and Y1; the others have
// all rows of the iteration in Y0.
func (g *gen) scatter() {
	switch g.w {
	case 16:
		g.f("VPACKUSWB Y1, Y0, Y2")
		g.f("VPERMQ $0xD8, Y2, Y2")
		g.f("VMOVDQU X2, (DI)")
		g.f("VEXTRACTI128 $1, Y2, (DI)(DX*1)")
	case 8:
		g.f("VEXTRACTI128 $1, Y0, X1")
		g.f("VPACKUSWB X1, X0, X0")
		g.f("VMOVQ X0, (DI)")
		g.f("VPEXTRQ $1, X0, (DI)(DX*1)")
	default:
		g.f("VEXTRACTI128 $1, Y0, X1")
		g.f("VPACKUSWB X1, X0, X0")
		g.f("VMOVD X0, (DI)")
		g.f("VPEXTRD $1, X0, (DI)(DX*1)")
		g.f("VPEXTRD $2, X0, (R9)")
		g.f("VPEXTRD $3, X0, (R9)(DX*1)")
	}
}

func (g *gen) advance(base, stride string) {
	switch g.rows() {
	case 2:
		g.f("LEAQ (%s)(%s*2), %s", base, stride, base)
	default:
		g.f("LEAQ (%s)(%s*4), %s", base, stride, base)
	}
}

func uniName(w int) string { return fmt.Sprintf("weightUni%d", w) }
func biName(w int) string  { return fmt.Sprintf("weightBi%d", w) }

func (g *gen) uni() string {
	g.b.Reset()
	fmt.Fprintf(&g.b, "// func %s(dst *byte, stride, h, weight, rnd, shift, off int)\n", uniName(g.w))
	fmt.Fprintf(&g.b, "TEXT ·%s(SB), NOSPLIT, $0-56\n", uniName(g.w))
	g.f("MOVQ dst+0(FP), DI")
	g.f("MOVQ stride+8(FP), DX")
	g.f("MOVQ h+16(FP), R8")
	g.f("MOVQ weight+24(FP), AX")
	g.f("VMOVD AX, X10")
	g.f("VPBROADCASTW X10, Y10")
	g.f("MOVQ rnd+32(FP), AX")
	g.f("VMOVD AX, X11")
	g.f("VPBROADCASTW X11, Y11")
	g.f("MOVQ shift+40(FP), AX")
	g.f("VMOVQ AX, X13")
	g.f("MOVQ off+48(FP), AX")
	g.f("VMOVD AX, X12")
	g.f("VPBROADCASTW X12, Y12")
	g.b.WriteString("loop:\n")
	g.gather(0, "DI", "DX", "R9", 1)
	last := 0
	if g.w == 16 {
		last = 1
	}
	for n := 0; n <= last; n++ {
		g.f("VPMULLW Y10, Y%d, Y%d", n, n)
		g.f("VPADDW Y11, Y%d, Y%d", n, n)
		g.f("VPSRAW X13, Y%d, Y%d", n, n)
		g.f("VPADDW Y12, Y%d, Y%d", n, n)
	}
	g.scatter()
	g.advance("DI", "DX")
	g.f("SUBQ $%d, R8", g.rows())
	g.f("JNZ loop")
	g.f("VZEROUPPER")
	g.f("RET")
	g.b.WriteString("\n")
	return g.b.String()
}

func (g *gen) bi() string {
	g.b.Reset()
	fmt.Fprintf(&g.b, "// func %s(dst *byte, dstStride int, src *byte, srcStride, h, wpair, k, shift int)\n", biName(g.w))
	fmt.Fprintf(&g.b, "TEXT ·%s(SB), NOSPLIT, $0-64\n", biName(g.w))
	g.f("MOVQ dst+0(FP), DI")
	g.f("MOVQ dstStride+8(FP), DX")
	g.f("MOVQ src+16(FP), SI")
	g.f("MOVQ srcStride+24(FP), CX")
	g.f("MOVQ h+32(FP), R8")
	g.f("MOVQ wpair+40(FP), AX")
	g.f("VMOVD AX, X10")
	g.f("VPBROADCASTD X10, Y10")
	g.f("MOVQ k+48(FP), AX")
	g.f("VMOVD AX, X11")
	g.f("VPBROADCASTD X11, Y11")
	g.f("MOVQ shift+56(FP), AX")
	g.f("VMOVQ AX, X13")
	g.b.WriteString("loop:\n")
	// Rows of the iteration: width 16 is one row per pass of the body (the
	// two-row grouping gains nothing before the multiply), the others fill a
	// vector.
	if g.w == 16 {
		g.f("VPMOVZXBW (DI), Y0")
		g.f("VPMOVZXBW (SI), Y1")
	} else {
		g.gather(0, "DI", "DX", "R9", 4)
		g.gather(1, "SI", "CX", "R10", 4)
	}
	g.f("VPUNPCKLWD Y1, Y0, Y2")
	g.f("VPUNPCKHWD Y1, Y0, Y3")
	for _, n := range []int{2, 3} {
		g.f("VPMADDWD Y10, Y%d, Y%d", n, n)
		g.f("VPADDD Y11, Y%d, Y%d", n, n)
		g.f("VPSRAD X13, Y%d, Y%d", n, n)
	}
	g.f("VPACKSSDW Y3, Y2, Y0")
	if g.w == 16 {
		g.f("VEXTRACTI128 $1, Y0, X1")
		g.f("VPACKUSWB X1, X0, X0")
		g.f("VMOVDQU X0, (DI)")
		g.f("ADDQ DX, DI")
		g.f("ADDQ CX, SI")
		g.f("DECQ R8")
	} else {
		g.scatter()
		g.advance("DI", "DX")
		g.advance("SI", "CX")
		g.f("SUBQ $%d, R8", g.rows())
	}
	g.f("JNZ loop")
	g.f("VZEROUPPER")
	g.f("RET")
	g.b.WriteString("\n")
	return g.b.String()
}

const header = "// Code generated by internal/genweight86; DO NOT EDIT.\n\n"

func generate() (string, string) {
	var s strings.Builder
	s.WriteString(header)
	s.WriteString("//go:build amd64 && !purego\n\n#include \"textflag.h\"\n\n")
	var t strings.Builder
	t.WriteString(header)
	t.WriteString("//go:build amd64 && !purego\n\npackage mc\n\n")
	for _, w := range []int{4, 8, 16} {
		g := &gen{w: w}
		s.WriteString(g.uni())
		s.WriteString(g.bi())
		fmt.Fprintf(&t, "//go:noescape\nfunc %s(dst *byte, stride, h, weight, rnd, shift, off int)\n\n", uniName(w))
		fmt.Fprintf(&t, "//go:noescape\nfunc %s(dst *byte, dstStride int, src *byte, srcStride, h, wpair, k, shift int)\n\n", biName(w))
	}
	return s.String(), strings.TrimRight(t.String(), "\n") + "\n"
}

var osExit = os.Exit

// files maps each generated file, relative to the repository root, to its
// content.
func files() map[string]string {
	asm, decl := generate()
	return map[string]string{
		"mc/weight_amd64.s":      asm,
		"mc/weight_amd64_tab.go": decl,
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
		fmt.Fprintln(os.Stderr, "genweight86:", err)
		osExit(1)
	}
}
