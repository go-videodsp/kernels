// Command genmc writes the arm64 NEON assembly for the H.264 luma
// quarter-sample kernels of package mc, and the Go declarations and dispatch
// table that go with it.
//
// Run from the repository root: go run ./internal/genmc
//
// The kernels are written from ITU-T H.264 (V16) (06/2026), clause 8.4.2.2.1
// (Figure 8-4, Equations 8-241 to 8-261, Table 8-12) and clause 8.4.2.3.1
// (Equation 8-273), and from docs/design/design.md, not from any decoder.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// kind is one of the samples a position is built from (Figure 8-4).
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

func (p position) usesTaps() bool {
	for _, t := range p.t {
		if t.k != kG {
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

type emitter struct {
	b strings.Builder
	w int // 4 or 8: columns per strip
}

func (e *emitter) f(format string, a ...any) { fmt.Fprintf(&e.b, "\t"+format+"\n", a...) }
func (e *emitter) label(s string)            { fmt.Fprintf(&e.b, "%s:\n", s) }

// ld loads one row fragment (4 bytes for width 4, 8 bytes otherwise) into
// the low lanes of Vn, zeroing the rest.
func (e *emitter) ld(n, off int, base string) {
	if e.w == 4 {
		e.f("FMOVS %d(%s), F%d", off, base, n)
	} else {
		e.f("FMOVD %d(%s), F%d", off, base, n)
	}
}

func (e *emitter) st(n int, base string) {
	if e.w == 4 {
		e.f("FMOVS F%d, (%s)", n, base)
	} else {
		e.f("FMOVD F%d, (%s)", n, base)
	}
}

// sixTap leaves the unrounded six-tap sum (Equations 8-241, 8-242) of the
// six zero-extended samples in V0..V5 in V6, as sixteen-bit lanes:
// (V0 + V5) + 20 * (V2 + V3) - 5 * (V1 + V4). The result lies in
// -2550..10710, so the lanes hold it exactly. V20 holds 20, V22 holds -5.
func (e *emitter) sixTap() {
	for i := 0; i < 6; i++ {
		e.f("VUXTL V%d.B8, V%d.H8", i, i)
	}
	e.f("VADD V5.H8, V0.H8, V6.H8")
	e.f("VADD V4.H8, V1.H8, V7.H8")
	e.f("VADD V3.H8, V2.H8, V8.H8")
	e.f("VMLA V20.H8, V8.H8, V6.H8")
	e.f("VMLA V22.H8, V7.H8, V6.H8")
}

// sqrshrun5 emits SQRSHRUN Vd.8B, Vn.8H, #5: ((x + 16) >> 5) clipped to
// 0..255 (Clip1Y of Equations 8-243, 8-244, 8-248, 8-249). cmd/asm has no
// SQRSHRUN (go-asmgen gap registry candidate), so the word is encoded:
// 0 Q=0 U=1 011110 immh=0001 immb=011 10001 1 Rn Rd.
func (e *emitter) sqrshrun5(d, n int) {
	e.f("WORD $0x%08x // SQRSHRUN V%d.8B, V%d.8H, #5", 0x2f0b8c00|uint32(n)<<5|uint32(d), d, n)
}

// hrow computes b (dy 0) or s (dy 1) of the current output sample row into
// the low lanes of Vout, as clipped bytes. R12 is the row's G pointer.
func (e *emitter) hrow(out int, dy int) {
	base := "R12"
	if dy == 1 {
		e.f("ADD R3, R12, R10")
		base = "R10"
	}
	for k := 0; k < 6; k++ {
		e.ld(k, k-2, base)
	}
	e.sixTap()
	e.sqrshrun5(out, 6)
}

// hrowRaw is hrow's unrounded sixteen-bit result for the row at the pointer
// in base, left in V6.
func (e *emitter) hrowRaw(base string) {
	for k := 0; k < 6; k++ {
		e.ld(k, k-2, base)
	}
	e.sixTap()
}

// vrow computes h (dx 0) or m (dx 1) of the current output sample row.
func (e *emitter) vrow(out int, dx int) {
	e.f("SUB R13, R12, R9")
	for k := 0; k < 6; k++ {
		e.ld(k, dx, "R9")
		if k < 5 {
			e.f("ADD R3, R9")
		}
	}
	e.sixTap()
	e.sqrshrun5(out, 6)
}

// centre computes j (Equations 8-246, 8-247) from the six rows of
// horizontal intermediates in V0..V5, into the low lanes of V16.
func (e *emitter) centre() {
	e.f("VADD V5.H8, V0.H8, V6.H8")
	e.f("VADD V4.H8, V1.H8, V7.H8")
	e.f("VADD V3.H8, V2.H8, V8.H8")
	e.f("VSXTL V6.H4, V9.S4")
	e.f("VSMLAL V20.H4, V8.H4, V9.S4")
	e.f("VSMLAL V22.H4, V7.H4, V9.S4")
	e.f("VSXTL2 V6.H8, V10.S4")
	e.f("VSMLAL2 V20.H8, V8.H8, V10.S4")
	e.f("VSMLAL2 V22.H8, V7.H8, V10.S4")
	e.f("VSRSHR $10, V9.S4, V9.S4")
	e.f("VSRSHR $10, V10.S4, V10.S4")
	e.f("VSQXTUN V9.S4, V11.H4")
	e.f("VSQXTUN2 V10.S4, V11.H8")
	e.f("VUQXTN V11.H8, V16.B8")
}

// plain computes a term that does not involve j into Vout.
func (e *emitter) plain(t term, out int) {
	switch t.k {
	case kG:
		base := "R12"
		if t.dy == 1 {
			e.f("ADD R3, R12, R10")
			base = "R10"
		}
		e.ld(out, t.dx, base)
	case kB:
		e.hrow(out, t.dy)
	case kH:
		e.vrow(out, t.dx)
	}
}

// finish averages V16 and V17 when there are two terms, then stores or
// averages into the destination row at R11.
func (e *emitter) finish(two, avg bool) {
	if two {
		e.f("VURHADD V17.B8, V16.B8, V16.B8")
	}
	if avg {
		e.ld(18, 0, "R11")
		e.f("VURHADD V18.B8, V16.B8, V16.B8")
	}
	e.st(16, "R11")
}

const frame = 352 // 21 rows of 16 bytes, plus the saved link register and slack

func genFunc(sb *strings.Builder, p position, avg bool, w int) {
	e := &emitter{w: w}
	name := funcName(avg, p.letter, w)
	j := p.usesJ()
	fmt.Fprintf(sb, "// func %s(dst *byte, dstStride int, src *byte, srcStride int, w int, h int)\n", name)
	if j {
		fmt.Fprintf(sb, "TEXT ·%s(SB), $%d-48\n", name, frame)
	} else {
		fmt.Fprintf(sb, "TEXT ·%s(SB), NOSPLIT, $0-48\n", name)
	}
	e.f("MOVD dst+0(FP), R0")
	e.f("MOVD dstStride+8(FP), R1")
	e.f("MOVD src+16(FP), R2")
	e.f("MOVD srcStride+24(FP), R3")
	if w == 8 {
		e.f("MOVD w+32(FP), R4")
	}
	e.f("MOVD h+40(FP), R5")
	if p.usesTaps() {
		e.f("LSL $1, R3, R13")
		e.f("MOVD $20, R8")
		e.f("VDUP R8, V20.H8")
		e.f("MOVD $-5, R8")
		e.f("VDUP R8, V22.H8")
	}
	if w == 8 {
		e.label("strip")
	}
	if j {
		// Horizontal intermediates of rows -2..h+2 into the stack buffer.
		e.f("SUB R13, R2, R14")
		e.f("ADD $5, R5, R15")
		e.f("ADD $16, RSP, R6")
		e.label("rows1")
		e.hrowRaw("R14")
		e.f("VST1.P [V6.H8], 16(R6)")
		e.f("ADD R3, R14")
		e.f("SUBS $1, R15")
		e.f("BNE rows1")
		e.f("ADD $16, RSP, R6")
	}
	e.f("MOVD R5, R7")
	e.f("MOVD R0, R11")
	e.f("MOVD R2, R12")
	e.label("rows")
	if j {
		for k := 0; k < 6; k++ {
			e.f("VLD1.P 16(R6), [V%d.H8]", k)
		}
		e.f("SUB $80, R6")
		e.centre()
		for _, t := range p.t {
			if t.k == kJ {
				continue
			}
			switch t.k {
			case kB: // b is row y+2 of the window, s is row y+3
				e.sqrshrun5(17, 2+t.dy)
			default:
				e.plain(t, 17)
			}
		}
		e.finish(len(p.t) == 2, avg)
	} else {
		e.plain(p.t[0], 16)
		if len(p.t) == 2 {
			e.plain(p.t[1], 17)
		}
		e.finish(len(p.t) == 2, avg)
	}
	e.f("ADD R1, R11")
	e.f("ADD R3, R12")
	e.f("SUBS $1, R7")
	e.f("BNE rows")
	if w == 8 {
		e.f("ADD $8, R0")
		e.f("ADD $8, R2")
		e.f("SUBS $8, R4")
		e.f("BNE strip")
	}
	e.f("RET")
	sb.WriteString(e.b.String())
	sb.WriteString("\n")
}

const header = "// Code generated by internal/genmc; DO NOT EDIT.\n\n"

func generate() (asm, tab string) {
	var s strings.Builder
	s.WriteString(header)
	s.WriteString("//go:build arm64 && !purego\n\n#include \"textflag.h\"\n\n")
	s.WriteString("// H.264 luma quarter-sample interpolation, put and average forms, NEON.\n")
	s.WriteString("// V20 = 20 and V22 = -5 in every sixteen-bit lane; V0..V5 hold the six taps;\n")
	s.WriteString("// V16 and V17 the two terms of a quarter position; V18 the destination row.\n\n")
	for _, avg := range []bool{false, true} {
		for _, w := range []int{4, 8} {
			for _, p := range positions {
				genFunc(&s, p, avg, w)
			}
		}
	}

	var t strings.Builder
	t.WriteString(header)
	t.WriteString("//go:build arm64 && !purego\n\npackage mc\n\n")
	for _, avg := range []bool{false, true} {
		for _, w := range []int{4, 8} {
			for _, p := range positions {
				fmt.Fprintf(&t, "//go:noescape\nfunc %s(dst *byte, dstStride int, src *byte, srcStride int, w int, h int)\n\n", funcName(avg, p.letter, w))
			}
		}
	}
	t.WriteString("// lumaKernels is indexed [avg][w != 4][xFrac + 4*yFrac].\n")
	t.WriteString("var lumaKernels = [2][2][16]func(dst *byte, dstStride int, src *byte, srcStride int, w int, h int){\n")
	for _, avg := range []bool{false, true} {
		t.WriteString("\t{\n")
		for _, w := range []int{4, 8} {
			t.WriteString("\t\t{")
			// Table order is xFrac + 4*yFrac.
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

func run(dir string) error {
	asm, tab := generate()
	for name, body := range map[string]string{"luma_arm64.s": asm, "luma_arm64_tab.go": tab} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	dir := "mc"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	if err := run(dir); err != nil {
		fmt.Fprintln(os.Stderr, "genmc:", err)
		osExit(1)
	}
}
