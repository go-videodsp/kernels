package main

import (
	"fmt"
	"strings"
)

// Chroma deblocking (4:2:0, one edge of eight lines per call): clause 8.7.2.3
// with chromaStyleFilteringFlag 1 (tC = tC0 + 1, only p0 and q0 change) and
// clause 8.7.2.4 for chroma (8-480, 8-487). One line per sixteen-bit lane of
// an XMM register. tc is passed as tC0 + 1 per group of two lines.
//
// A horizontal edge reads each of its four rows with one VPMOVZXBW of eight
// bytes. A vertical edge loads eight rows of four bytes, transposes them to
// four rows of eight lines with a PSHUFB (4x4 transposes) and two dword
// unpacks, and transposes back through byte and word unpacks.

type chroma struct {
	b strings.Builder
}

func (c *chroma) f(format string, a ...any) { fmt.Fprintf(&c.b, "\t"+format+"\n", a...) }

// absLess writes into X(dst) the lanes of |X(a) - X(b)| < X(t), through X(tmp).
func (c *chroma) absLess(dst, a, b, t, tmp int) {
	c.f("VPSUBW X%d, X%d, X%d", b, a, tmp)
	c.f("VPABSW X%d, X%d", tmp, tmp)
	c.f("VPCMPGTW X%d, X%d, X%d", tmp, t, dst)
}

// thresholds leaves in X8 the lanes where p0, q0 may change (8-460); p1 X0,
// p0 X1, q0 X2, q1 X3, alpha X4, beta X5.
func (c *chroma) thresholds() {
	c.absLess(8, 1, 2, 4, 12)
	c.absLess(9, 0, 1, 5, 12)
	c.f("VPAND X9, X8, X8")
	c.absLess(9, 3, 2, 5, 12)
	c.f("VPAND X9, X8, X8")
}

// normalCore leaves p0' in X1 and q0' in X2 (words). tc (tC0 + 1) is X6, the
// bS != 0 lanes X7.
func (c *chroma) normalCore() {
	c.thresholds()
	c.f("VPAND X7, X8, X8")
	c.f("VPSUBW X1, X2, X10") // q0 - p0
	c.f("VPSLLW $2, X10, X10")
	c.f("VPSUBW X3, X0, X11") // p1 - q1
	c.f("VPADDW X11, X10, X10")
	c.f("VPADDW lfc4<>(SB), X10, X10")
	c.f("VPSRAW $3, X10, X10")
	c.f("VPXOR X13, X13, X13")
	c.f("VPSUBW X6, X13, X13") // -tc
	c.f("VPMINSW X6, X10, X10")
	c.f("VPMAXSW X13, X10, X10")
	c.f("VPAND X8, X10, X10")
	c.f("VPADDW X10, X1, X1")
	c.f("VPSUBW X10, X2, X2")
}

// strongCore leaves p0' in X1 and q0' in X2 (words): (2*p1 + p0 + q1 + 2) >> 2
// and (2*q1 + q0 + p1 + 2) >> 2 where 8-460 holds.
func (c *chroma) strongCore() {
	c.thresholds()
	c.f("VPADDW X0, X0, X10")
	c.f("VPADDW X1, X10, X10")
	c.f("VPADDW X3, X10, X10")
	c.f("VPADDW lfc2<>(SB), X10, X10")
	c.f("VPSRAW $2, X10, X10")
	c.f("VPADDW X3, X3, X11")
	c.f("VPADDW X2, X11, X11")
	c.f("VPADDW X0, X11, X11")
	c.f("VPADDW lfc2<>(SB), X11, X11")
	c.f("VPSRAW $2, X11, X11")
	c.f("VPBLENDVB X8, X10, X1, X1")
	c.f("VPBLENDVB X8, X11, X2, X2")
}

func (c *chroma) bcast(arg string, n int) {
	c.f("MOVQ %s, AX", arg)
	c.f("VMOVD AX, X%d", n)
	c.f("VPBROADCASTW X%d, X%d", n, n)
}

func (c *chroma) prologueConstants(strong bool) {
	c.bcast("alpha+16(FP)", 4)
	c.bcast("beta+24(FP)", 5)
	if strong {
		return
	}
	c.f("MOVL tc0+32(FP), AX")
	c.f("VMOVD AX, X6")
	c.f("VPSHUFB lfshuf2<>(SB), X6, X6")
	c.f("VPMOVZXBW X6, X6")
	c.f("MOVL mask+36(FP), AX")
	c.f("VMOVD AX, X7")
	c.f("VPSHUFB lfshuf2<>(SB), X7, X7")
	c.f("VPMOVSXBW X7, X7")
}

func (c *chroma) kernel(name string, vertical, strong bool) string {
	c.b.Reset()
	sig, args := "pix *byte, stride int, alpha, beta int, tc0, mask uint32", 40
	if strong {
		sig, args = "pix *byte, stride int, alpha, beta int", 32
	}
	fmt.Fprintf(&c.b, "// func %s(%s)\n", name, sig)
	fmt.Fprintf(&c.b, "TEXT ·%s(SB), NOSPLIT, $0-%d\n", name, args)
	c.f("MOVQ pix+0(FP), DI")
	c.f("MOVQ stride+8(FP), DX")
	if vertical {
		c.f("LEAQ (DX)(DX*2), R13")
		c.f("LEAQ -2(DI), R8")
		c.f("LEAQ (R8)(DX*4), R9")
		rows := func(base string) [4]string {
			return [4]string{"(" + base + ")", "(" + base + ")(DX*1)", "(" + base + ")(DX*2)", "(" + base + ")(R13*1)"}
		}
		for n, base := range []string{"R8", "R9"} {
			r := rows(base)
			c.f("VMOVD %s, X%d", r[0], n)
			for k := 1; k < 4; k++ {
				c.f("VPINSRD $%d, %s, X%d, X%d", k, r[k], n, n)
			}
			c.f("VPSHUFB lfT4<>(SB), X%d, X%d", n, n)
		}
		c.f("VPUNPCKLDQ X1, X0, X8") // columns p1 (low), p0 (high)
		c.f("VPUNPCKHDQ X1, X0, X9") // columns q0 (low), q1 (high)
		c.f("VPMOVZXBW X8, X0")
		c.f("VPSHUFD $0xEE, X8, X8")
		c.f("VPMOVZXBW X8, X1")
		c.f("VPMOVZXBW X9, X2")
		c.f("VPSHUFD $0xEE, X9, X9")
		c.f("VPMOVZXBW X9, X3")
	} else {
		c.f("MOVQ DX, R9")
		c.f("NEGQ R9")
		c.f("LEAQ (R9)(R9*1), R10")
		c.f("VPMOVZXBW (DI)(R10*1), X0")
		c.f("VPMOVZXBW (DI)(R9*1), X1")
		c.f("VPMOVZXBW (DI), X2")
		c.f("VPMOVZXBW (DI)(DX*1), X3")
	}
	c.prologueConstants(strong)
	if strong {
		c.strongCore()
	} else {
		c.normalCore()
	}
	if vertical {
		for _, n := range []int{0, 1, 2, 3} {
			c.f("VPACKUSWB X%d, X%d, X%d", n, n, n)
		}
		c.f("VPUNPCKLBW X1, X0, X8")
		c.f("VPUNPCKLBW X3, X2, X9")
		c.f("VPUNPCKLWD X9, X8, X10") // lines 0-3
		c.f("VPUNPCKHWD X9, X8, X11") // lines 4-7
		for n, base := range []string{"R8", "R9"} {
			r := [4]string{"(" + base + ")", "(" + base + ")(DX*1)", "(" + base + ")(DX*2)", "(" + base + ")(R13*1)"}
			c.f("VMOVD X%d, %s", 10+n, r[0])
			for k := 1; k < 4; k++ {
				c.f("VPEXTRD $%d, X%d, %s", k, 10+n, r[k])
			}
		}
	} else {
		c.f("VPACKUSWB X1, X1, X1")
		c.f("VPACKUSWB X2, X2, X2")
		c.f("VMOVQ X1, (DI)(R9*1)")
		c.f("VMOVQ X2, (DI)")
	}
	c.f("RET")
	c.b.WriteString("\n")
	return c.b.String()
}

func generateChroma() (string, string) {
	var s, t strings.Builder
	s.WriteString(header)
	s.WriteString("//go:build amd64 && !purego\n\n#include \"textflag.h\"\n\n")
	// Expansion of the four group bytes (one per two lines) to eight lines, and
	// the 4x4 byte transpose of the vertical edges.
	s.WriteString("DATA lfshuf2<>+0(SB)/8, $0x0303020201010000\n")
	s.WriteString("DATA lfshuf2<>+8(SB)/8, $0x0000000000000000\n")
	s.WriteString("GLOBL lfshuf2<>(SB), RODATA|NOPTR, $16\n\n")
	s.WriteString("DATA lfT4<>+0(SB)/8, $0x0d0905010c080400\n")
	s.WriteString("DATA lfT4<>+8(SB)/8, $0x0f0b07030e0a0602\n")
	s.WriteString("GLOBL lfT4<>(SB), RODATA|NOPTR, $16\n\n")
	for _, k := range []struct {
		name string
		v    uint64
	}{{"lfc2", 0x0002000200020002}, {"lfc4", 0x0004000400040004}} {
		for o := 0; o < 16; o += 8 {
			fmt.Fprintf(&s, "DATA %s<>+%d(SB)/8, $0x%016x\n", k.name, o, k.v)
		}
		fmt.Fprintf(&s, "GLOBL %s<>(SB), RODATA|NOPTR, $16\n\n", k.name)
	}
	t.WriteString(header)
	t.WriteString("//go:build amd64 && !purego\n\npackage lf\n\n")
	c := &chroma{}
	for _, k := range []struct {
		name             string
		vertical, strong bool
	}{
		{"deblockChromaNormalH", false, false}, {"deblockChromaNormalV", true, false},
		{"deblockChromaStrongH", false, true}, {"deblockChromaStrongV", true, true},
	} {
		s.WriteString(c.kernel(k.name, k.vertical, k.strong))
		sig := "pix *byte, stride int, alpha, beta int, tc0, mask uint32"
		if k.strong {
			sig = "pix *byte, stride int, alpha, beta int"
		}
		fmt.Fprintf(&t, "//go:noescape\nfunc %s(%s)\n\n", k.name, sig)
	}
	return s.String(), strings.TrimRight(t.String(), "\n") + "\n"
}
