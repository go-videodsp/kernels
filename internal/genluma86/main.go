// Command genluma86 writes the amd64 AVX2 assembly for the H.264 luma
// quarter-sample kernels of package mc, and the Go declarations and dispatch
// table that go with it.
//
// Run from the repository root: go run ./internal/genluma86
//
// The kernels are written from ITU-T H.264 (V16) (06/2026), clause 8.4.2.2.1
// (Figure 8-4, Equations 8-241 to 8-261, Table 8-12) and clause 8.4.2.3.1
// (Equation 8-273), and from docs/design/design.md, not from any decoder.
//
// Arithmetic, per sample of one row:
//
//   - the six-tap sum a - 5b + 20c + 20d - 5e + f of Equations 8-241 and
//     8-242 is evaluated on sixteen-bit lanes as (a+f) + 5 * (4(c+d) - (b+e));
//     its range is -2550..10710, so the lanes are exact;
//   - Clip1Y((x + 16) >> 5) (8-243, 8-244, 8-248, 8-249) is PMULHRSW by 1024,
//     which computes (x*1024 + 2^14) >> 15 = (x + 16) >> 5, followed by the
//     unsigned saturating pack to bytes (a clip to 0..255);
//   - the centre sample j (8-246, 8-247) is two passes. Pass one writes the
//     horizontal intermediates b1 of rows -2..h+2 to the stack, and the word
//     pairs (b1 of row r, b1 of row r+1) next to them; pass two evaluates the
//     vertical six-tap with three PMADDWD (coefficient pairs (1,-5), (20,20),
//     (-5,1)) on thirty-two-bit lanes, adds 512, shifts by 10, and packs with
//     signed then unsigned saturation, which together clip to 0..255;
//   - the rounding average of two predictions (8-250 to 8-261, 8-273) is
//     PAVGB, (x + y + 1) >> 1, on the clipped bytes.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type kind byte

const (
	kG kind = 'G' // full sample displaced by (dx, dy)
	kB kind = 'B' // horizontal half sample: b (dy 0), s (dy 1)
	kH kind = 'H' // vertical half sample: h (dx 0), m (dx 1)
	kJ kind = 'J' // centre half sample
)

type term struct {
	k      kind
	dx, dy int
}

type position struct {
	letter string
	xf, yf int
	t      []term // one term, or two to average (Equations 8-250 to 8-261)
}

var (
	tb = term{kB, 0, 0}
	ts = term{kB, 0, 1}
	th = term{kH, 0, 0}
	tm = term{kH, 1, 0}
	tj = term{kJ, 0, 0}
	tg = term{kG, 0, 0}
)

// positions is Table 8-12 with the averages of 8-250 to 8-261.
var positions = []position{
	{"G", 0, 0, []term{tg}},
	{"d", 0, 1, []term{tg, th}},
	{"h", 0, 2, []term{th}},
	{"n", 0, 3, []term{{kG, 0, 1}, th}},
	{"a", 1, 0, []term{tg, tb}},
	{"e", 1, 1, []term{tb, th}},
	{"i", 1, 2, []term{th, tj}},
	{"p", 1, 3, []term{th, ts}},
	{"b", 2, 0, []term{tb}},
	{"f", 2, 1, []term{tb, tj}},
	{"j", 2, 2, []term{tj}},
	{"q", 2, 3, []term{tj, ts}},
	{"c", 3, 0, []term{{kG, 1, 0}, tb}},
	{"g", 3, 1, []term{tb, tm}},
	{"k", 3, 2, []term{tj, tm}},
	{"r", 3, 3, []term{tm, ts}},
}

func (p position) usesJ() bool {
	for _, t := range p.t {
		if t.k == kJ {
			return true
		}
	}
	return false
}

func funcName(avg bool, letter string, w int) string {
	op := "put"
	if avg {
		op = "avg"
	}
	return fmt.Sprintf("luma%s%s%d", op, letter, w)
}

// Registers. Y0..Y5 hold the six taps, Y6 and Y7 the bytes of the (up to two)
// terms of a position, Y8 temporaries. Y9 is 512 (dwords), Y10 is 5, Y11 is
// 1024, Y12..Y14 are the PMADDWD coefficient pairs. R9 is the stack scratch,
// R10 the pass-one row pointer then the pass-two intermediate pointer, R14 the
// pair pointer, R11 = 3*srcStride, R12 = -srcStride, R13 = -2*srcStride.
const (
	regK512 = 9
	regK5   = 10
	regK1k  = 11
	regC1   = 12
	regC2   = 13
	regC3   = 14
)

type gen struct {
	b   strings.Builder
	w   int // 4, 8 or 16: columns of the block
	avg bool
}

func (g *gen) f(format string, a ...any) { fmt.Fprintf(&g.b, "\t"+format+"\n", a...) }

// vec names register n at the width the kernel computes in: YMM for sixteen
// columns, XMM below.
func (g *gen) vec(n int) string {
	if g.w == 16 {
		return fmt.Sprintf("Y%d", n)
	}
	return fmt.Sprintf("X%d", n)
}

func (g *gen) vecBytes() int {
	if g.w == 16 {
		return 32
	}
	return 16
}

// ldw loads w bytes at mem, zero-extended to sixteen-bit lanes of register n,
// reading exactly the w bytes of the block's row.
func (g *gen) ldw(n int, mem string) {
	switch g.w {
	case 16:
		g.f("VPMOVZXBW %s, Y%d", mem, n)
	case 8:
		g.f("VPMOVZXBW %s, X%d", mem, n)
	default:
		g.f("VMOVD %s, X%d", mem, n)
		g.f("VPMOVZXBW X%d, X%d", n, n)
	}
}

// ldb loads w bytes at mem into the low bytes of XMM n.
func (g *gen) ldb(n int, mem string) {
	switch g.w {
	case 16:
		g.f("VMOVDQU %s, X%d", mem, n)
	case 8:
		g.f("VMOVQ %s, X%d", mem, n)
	default:
		g.f("VMOVD %s, X%d", mem, n)
	}
}

func (g *gen) stb(n int, mem string) {
	switch g.w {
	case 16:
		g.f("VMOVDQU X%d, %s", n, mem)
	case 8:
		g.f("VMOVQ X%d, %s", n, mem)
	default:
		g.f("VMOVD X%d, %s", n, mem)
	}
}

// tap6 evaluates the unrounded six-tap sum of the six zero-extended samples
// at addr(0..5), into the lanes of register 2.
func (g *gen) tap6(addr func(k int) string) {
	for k := 0; k < 6; k++ {
		g.ldw(k, addr(k))
	}
	v := g.vec
	g.f("VPADDW %s, %s, %s", v(5), v(0), v(0)) // a + f
	g.f("VPADDW %s, %s, %s", v(4), v(1), v(1)) // b + e
	g.f("VPADDW %s, %s, %s", v(3), v(2), v(2)) // c + d
	g.f("VPSLLW $2, %s, %s", v(2), v(2))
	g.f("VPSUBW %s, %s, %s", v(1), v(2), v(2))
	g.f("VPMULLW %s, %s, %s", v(regK5), v(2), v(2))
	g.f("VPADDW %s, %s, %s", v(0), v(2), v(2))
}

// bytesOf packs the sixteen-bit lanes of register n, clipped to 0..255, into
// the bytes of XMM d (the first w bytes are the row).
func (g *gen) bytesOf(n, d int) {
	if g.w == 16 {
		g.f("VEXTRACTI128 $1, Y%d, X8", n)
		g.f("VPACKUSWB X8, X%d, X%d", n, d)
		return
	}
	g.f("VPACKUSWB X%d, X%d, X%d", n, n, d)
}

// round writes Clip1Y((x + 16) >> 5) of the six-tap sum in register 2 as
// bytes into XMM d.
func (g *gen) round(d int) {
	g.f("VPMULHRSW %s, %s, %s", g.vec(regK1k), g.vec(2), g.vec(2))
	g.bytesOf(2, d)
}

// term writes the bytes of one term of the current output row into XMM d.
// SI addresses G of the row; B is the stack intermediate for positions that
// use j.
func (g *gen) term(t term, d int, j bool) {
	disp := func(k int) string { // columns k-2 of the row (dy rows down)
		if t.dy == 1 {
			return fmt.Sprintf("%d(SI)(CX*1)", k-2)
		}
		return fmt.Sprintf("%d(SI)", k-2)
	}
	switch t.k {
	case kG:
		if t.dy == 1 {
			g.ldb(d, fmt.Sprintf("%d(SI)(CX*1)", t.dx))
		} else {
			g.ldb(d, fmt.Sprintf("%d(SI)", t.dx))
		}
	case kB:
		if j {
			// b1 of this row (b) or the next (s) was stored by pass one.
			g.f("VMOVDQU %d(R10), %s", (2+t.dy)*g.vecBytes(), g.vec(2))
		} else {
			g.tap6(disp)
		}
		g.round(d)
	case kH:
		rows := [6]string{"R13", "R12", "", "CX", "", "R11"}
		g.tap6(func(k int) string {
			switch k {
			case 2:
				return fmt.Sprintf("%d(SI)", t.dx)
			case 4:
				return fmt.Sprintf("%d(SI)(CX*2)", t.dx)
			}
			return fmt.Sprintf("%d(SI)(%s*1)", t.dx, rows[k])
		})
		g.round(d)
	case kJ:
		g.jrow(d)
	}
}

// jrow writes the bytes of the centre sample of the current row into XMM d,
// from the pair intermediates at R14 (pass one).
func (g *gen) jrow(d int) {
	vb := g.vecBytes()
	ps := g.pairBytes()
	halves := 2
	if g.w == 4 {
		halves = 1
	}
	for h := 0; h < halves; h++ {
		o := h * vb
		g.f("VPMADDWD %d(R14), %s, %s", o, g.vec(regC1), g.vec(3*h))
		g.f("VPMADDWD %d(R14), %s, %s", 2*ps+o, g.vec(regC2), g.vec(3*h+1))
		g.f("VPMADDWD %d(R14), %s, %s", 4*ps+o, g.vec(regC3), g.vec(3*h+2))
		g.f("VPADDD %s, %s, %s", g.vec(3*h+1), g.vec(3*h), g.vec(3*h))
		g.f("VPADDD %s, %s, %s", g.vec(3*h+2), g.vec(3*h), g.vec(3*h))
		g.f("VPADDD %s, %s, %s", g.vec(regK512), g.vec(3*h), g.vec(3*h))
		g.f("VPSRAD $10, %s, %s", g.vec(3*h), g.vec(3*h))
	}
	if halves == 2 {
		g.f("VPACKSSDW %s, %s, %s", g.vec(3), g.vec(0), g.vec(0))
	} else {
		g.f("VPACKSSDW X0, X0, X0")
	}
	g.bytesOf(0, d)
}

// pairBytes is the size of one stored pair row: the low and the high halves
// of the interleave of two rows of intermediates (the low half only at
// width 4).
func (g *gen) pairBytes() int {
	if g.w == 4 {
		return 16
	}
	return 2 * g.vecBytes()
}

func (g *gen) maxH() int {
	if g.w == 4 {
		return 8
	}
	return 16
}

func (g *gen) constant(reg int, v uint32) {
	g.f("MOVL $0x%08x, AX", v)
	g.f("VMOVD AX, X%d", reg)
	g.f("VPBROADCASTD X%d, %s", reg, g.vec(reg))
}

// pass1 writes b1 of rows -2..h+2 to the stack and the pairs of consecutive
// rows after them.
func (g *gen) pass1(poff int) {
	vb := g.vecBytes()
	ps := g.pairBytes()
	g.f("LEAQ (SI)(R13*1), R10")
	g.tap6(func(k int) string { return fmt.Sprintf("%d(R10)", k-2) })
	g.f("VMOVDQU %s, (R9)", g.vec(2))
	g.f("VMOVDQA %s, %s", g.vec(2), g.vec(6))
	g.f("LEAQ %d(R9), R14", vb)
	g.f("LEAQ %d(R9), R12", poff)
	g.f("LEAQ 4(R8), BX")
	g.b.WriteString("p1:\n")
	g.f("ADDQ CX, R10")
	g.tap6(func(k int) string { return fmt.Sprintf("%d(R10)", k-2) })
	g.f("VMOVDQU %s, (R14)", g.vec(2))
	g.f("VPUNPCKLWD %s, %s, %s", g.vec(2), g.vec(6), g.vec(7))
	g.f("VMOVDQU %s, (R12)", g.vec(7))
	if g.w != 4 {
		g.f("VPUNPCKHWD %s, %s, %s", g.vec(2), g.vec(6), g.vec(7))
		g.f("VMOVDQU %s, %d(R12)", g.vec(7), vb)
	}
	g.f("VMOVDQA %s, %s", g.vec(2), g.vec(6))
	g.f("ADDQ $%d, R14", vb)
	g.f("ADDQ $%d, R12", ps)
	g.f("DECQ BX")
	g.f("JNZ p1")
}

func (g *gen) kernel(p position) string {
	g.b.Reset()
	name := funcName(g.avg, p.letter, g.w)
	j := p.usesJ()
	needTaps := false
	for _, t := range p.t {
		if t.k != kG {
			needTaps = true
		}
	}
	vb := g.vecBytes()
	poff := (g.maxH() + 5) * vb
	frame := 0
	flags := "NOSPLIT"
	if j {
		frame = poff + (g.maxH()+4)*g.pairBytes()
		flags = "0"
	}
	fmt.Fprintf(&g.b, "// func %s(dst *byte, dstStride int, src *byte, srcStride int, w int, h int)\n", name)
	fmt.Fprintf(&g.b, "TEXT ·%s(SB), %s, $%d-48\n", name, flags, frame)
	g.f("MOVQ dst+0(FP), DI")
	g.f("MOVQ dstStride+8(FP), DX")
	g.f("MOVQ src+16(FP), SI")
	g.f("MOVQ srcStride+24(FP), CX")
	g.f("MOVQ h+40(FP), R8")
	if needTaps {
		g.constant(regK5, 0x00050005)
		g.constant(regK1k, 0x04000400)
		g.f("MOVQ CX, R12")
		g.f("NEGQ R12")
		g.f("LEAQ (R12)(R12*1), R13")
		g.f("LEAQ (CX)(CX*2), R11")
	}
	if j {
		g.constant(regK512, 0x00000200)
		g.constant(regC1, 0xfffb0001)
		g.constant(regC2, 0x00140014)
		g.constant(regC3, 0x0001fffb)
		g.f("MOVQ SP, R9")
		g.pass1(poff)
		g.f("MOVQ R9, R10")
		g.f("LEAQ %d(R9), R14", poff)
		g.f("MOVQ CX, R12")
		g.f("NEGQ R12")
	}
	g.b.WriteString("row:\n")
	for i, t := range p.t {
		g.term(t, 6+i, j)
	}
	if len(p.t) == 2 {
		g.f("VPAVGB X7, X6, X6")
	}
	if g.avg {
		if g.w == 16 {
			g.f("VPAVGB (DI), X6, X6")
		} else {
			g.ldb(8, "(DI)")
			g.f("VPAVGB X8, X6, X6")
		}
	}
	g.stb(6, "(DI)")
	g.f("ADDQ CX, SI")
	g.f("ADDQ DX, DI")
	if j {
		g.f("ADDQ $%d, R10", vb)
		g.f("ADDQ $%d, R14", g.pairBytes())
	}
	g.f("DECQ R8")
	g.f("JNZ row")
	g.f("VZEROUPPER")
	g.f("RET")
	g.b.WriteString("\n")
	return g.b.String()
}

const header = "// Code generated by internal/genluma86; DO NOT EDIT.\n\n"

func generate() (string, string) {
	var s strings.Builder
	s.WriteString(header)
	s.WriteString("//go:build amd64 && !purego\n\n#include \"textflag.h\"\n\n")
	for _, avg := range []bool{false, true} {
		for _, w := range []int{4, 8, 16} {
			for _, p := range positions {
				g := &gen{w: w, avg: avg}
				s.WriteString(g.kernel(p))
			}
		}
	}

	var t strings.Builder
	t.WriteString(header)
	t.WriteString("//go:build amd64 && !purego\n\npackage mc\n\n")
	for _, avg := range []bool{false, true} {
		for _, w := range []int{4, 8, 16} {
			for _, p := range positions {
				fmt.Fprintf(&t, "//go:noescape\nfunc %s(dst *byte, dstStride int, src *byte, srcStride int, w int, h int)\n\n", funcName(avg, p.letter, w))
			}
		}
	}
	t.WriteString("// lumaKernels is indexed [avg][width class: 4, 8, 16][xFrac + 4*yFrac].\n")
	t.WriteString("var lumaKernels = [2][3][16]func(dst *byte, dstStride int, src *byte, srcStride int, w int, h int){\n")
	for _, avg := range []bool{false, true} {
		t.WriteString("\t{\n")
		for _, w := range []int{4, 8, 16} {
			t.WriteString("\t\t{")
			var order [16]string
			for _, p := range positions {
				order[p.xf+4*p.yf] = funcName(avg, p.letter, w)
			}
			t.WriteString(strings.Join(order[:], ", "))
			t.WriteString("},\n")
		}
		t.WriteString("\t},\n")
	}
	t.WriteString("}\n")
	return s.String(), t.String()
}

var osExit = os.Exit

// files maps each generated file, relative to the repository root, to its
// content.
func files() map[string]string {
	asm, tab := generate()
	return map[string]string{
		"mc/luma_amd64.s":      asm,
		"mc/luma_amd64_tab.go": tab,
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
		fmt.Fprintln(os.Stderr, "genluma86:", err)
		osExit(1)
	}
}
