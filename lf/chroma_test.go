package lf

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
)

// The chroma oracle transcribes ITU-T H.264 (V16) (06/2026) clauses 8.7.2.2
// to 8.7.2.4 with chromaStyleFilteringFlag = 1 (Equation 8-450 for 4:2:0),
// reusing the table transcriptions of deblock_test.go: tC = tC0 + 1 (8-466),
// Δ (8-467), p'0 and q'0 (8-468, 8-469), p'1 = p1 and q'1 = q1 (8-471,
// 8-473); for bS 4 p'0 (8-480) and q'0 (8-487).
func filterSetChroma(p, q [2]int, bS, indexA, indexB int) (p0n, q0n int) {
	p0n, q0n = p[0], q[0]
	alpha, beta := half(oracleAlpha, indexA), half(oracleBeta, indexB)
	if !(bS != 0 && absInt(p[0]-q[0]) < alpha && absInt(p[1]-p[0]) < beta && absInt(q[1]-q[0]) < beta) {
		return
	}
	if bS < 4 {
		tc := half(oracleTC0[bS-1], indexA) + 1
		delta := oclip3(-tc, tc, (((q[0]-p[0])<<2)+(p[1]-q[1])+4)>>3)
		return oclip3(0, 255, p[0]+delta), oclip3(0, 255, q[0]-delta)
	}
	return (2*p[1] + p[0] + q[1] + 2) >> 2, (2*q[1] + q[0] + p[1] + 2) >> 2
}

// oracleChromaEdge filters 8 lines in place; bS is the strength of each
// group of two lines.
func oracleChromaEdge(pix []byte, off, stride int, vertical bool, indexA, indexB int, bS [4]int) {
	for k := 0; k < 8; k++ {
		at := func(i int) int { // i >= 0: q_i; i < 0: p_(-i-1)
			if vertical {
				return off + k*stride + i
			}
			return off + k + i*stride
		}
		p := [2]int{int(pix[at(-1)]), int(pix[at(-2)])}
		q := [2]int{int(pix[at(0)]), int(pix[at(1)])}
		p0, q0 := filterSetChroma(p, q, bS[k/2], indexA, indexB)
		pix[at(-1)], pix[at(0)] = byte(p0), byte(q0)
	}
}

func runChroma(t *testing.T, r *rand.Rand, vertical bool, indexA, indexB int, bS [4]int, strong bool) {
	t.Helper()
	pl := make([]byte, planeStride*planeRows)
	fillEdges(r, pl)
	off := edgeOffset(vertical, r.Intn(4))
	want := append([]byte(nil), pl...)
	oracleChromaEdge(want, off, planeStride, vertical, indexA, indexB, bS)
	got := append([]byte(nil), pl...)
	ref := append([]byte(nil), pl...)
	var u [4]uint8
	for i := range u {
		u[i] = uint8(bS[i])
	}
	switch {
	case strong && vertical:
		DeblockChromaIntraV(got, off, planeStride, indexA, indexB)
		deblockChromaStrongRef(ref, off, planeStride, true, indexA, indexB)
	case strong:
		DeblockChromaIntraH(got, off, planeStride, indexA, indexB)
		deblockChromaStrongRef(ref, off, planeStride, false, indexA, indexB)
	case vertical:
		DeblockChromaV(got, off, planeStride, indexA, indexB, u)
		deblockChromaNormalRef(ref, off, planeStride, true, indexA, indexB, u)
	default:
		DeblockChromaH(got, off, planeStride, indexA, indexB, u)
		deblockChromaNormalRef(ref, off, planeStride, false, indexA, indexB, u)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("vertical=%v strong=%v indexA %d indexB %d bS %v: first difference at %d", vertical, strong, indexA, indexB, bS, firstDiff(got, want))
	}
	if !bytes.Equal(ref, want) {
		t.Fatalf("portable vertical=%v strong=%v indexA %d indexB %d bS %v: first difference at %d", vertical, strong, indexA, indexB, bS, firstDiff(ref, want))
	}
}

func TestChromaExhaustive(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	patterns := [][4]int{
		{1, 1, 1, 1}, {2, 2, 2, 2}, {3, 3, 3, 3}, {0, 0, 0, 0},
		{0, 1, 2, 3}, {3, 2, 1, 0}, {1, 0, 3, 2}, {2, 3, 0, 1},
	}
	for _, vertical := range []bool{true, false} {
		t.Run(fmt.Sprintf("vertical=%v", vertical), func(t *testing.T) {
			for a := 0; a < 52; a++ {
				for b := 0; b < 52; b++ {
					for _, p := range patterns {
						runChroma(t, r, vertical, a, b, p, false)
					}
					for i := 0; i < 4; i++ {
						runChroma(t, r, vertical, a, b, [4]int{r.Intn(4), r.Intn(4), r.Intn(4), r.Intn(4)}, false)
					}
					for i := 0; i < 3; i++ {
						runChroma(t, r, vertical, a, b, [4]int{4, 4, 4, 4}, true)
					}
				}
			}
		})
	}
}

func TestChromaRefused(t *testing.T) {
	pl := make([]byte, 64*64)
	var z [4]uint8
	cases := map[string]func(){
		"V stride 7":     func() { DeblockChromaV(pl, 4*64+8, 7, 30, 30, z) },
		"H stride 7":     func() { DeblockChromaH(pl, 4*64+8, 7, 30, 30, z) },
		"V indexA 52":    func() { DeblockChromaV(pl, 4*64+8, 64, 52, 30, z) },
		"H indexB -1":    func() { DeblockChromaH(pl, 4*64+8, 64, 30, -1, z) },
		"V bS 4":         func() { DeblockChromaV(pl, 4*64+8, 64, 30, 30, [4]uint8{0, 0, 4, 0}) },
		"H bS 9":         func() { DeblockChromaH(pl, 4*64+8, 64, 30, 30, [4]uint8{9, 0, 0, 0}) },
		"V left":         func() { DeblockChromaV(pl, 1, 64, 30, 30, z) },
		"V past end":     func() { DeblockChromaV(pl, 64*64-7*64-1, 64, 30, 30, z) },
		"H above":        func() { DeblockChromaH(pl, 64+8, 64, 30, 30, z) },
		"H past end":     func() { DeblockChromaH(pl, 64*64-64-7, 64, 30, 30, z) },
		"intra V stride": func() { DeblockChromaIntraV(pl, 4*64+8, 4, 30, 30) },
		"intra H index":  func() { DeblockChromaIntraH(pl, 4*64+8, 64, 99, 30) },
		"intra V past":   func() { DeblockChromaIntraV(pl, 64*64-7*64-1, 64, 30, 30) },
		"intra H above":  func() { DeblockChromaIntraH(pl, 64+8, 64, 30, 30) },
	}
	for name, f := range cases {
		mustPanic(t, name, f)
	}
	DeblockChromaV(pl, 2, 64, 30, 30, z)
	DeblockChromaV(pl, 64*64-7*64-2, 64, 30, 30, z)
	DeblockChromaH(pl, 2*64, 64, 30, 30, z)
	DeblockChromaH(pl, 64*64-2*64, 64, 30, 30, z)
}

func FuzzChroma(f *testing.F) {
	r := rand.New(rand.NewSource(8))
	pl := make([]byte, planeStride*planeRows)
	fillEdges(r, pl)
	f.Add(pl, uint16(edgeOffset(true, 0)), uint16(planeStride), uint8(30), uint8(30), uint8(0x1b), false, true)
	f.Add(pl, uint16(edgeOffset(false, 0)), uint16(planeStride), uint8(51), uint8(51), uint8(0xff), true, false)
	f.Add(pl[:200], uint16(100), uint16(8), uint8(10), uint8(60), uint8(0x55), false, false)

	f.Fuzz(func(t *testing.T, plane []byte, offIn, strideIn uint16, ia, ib, bsBits uint8, strong, vertical bool) {
		off, stride := int(offIn), int(strideIn)
		indexA, indexB := int(ia), int(ib)
		var bS [4]int
		var u [4]uint8
		for i := range bS {
			bS[i] = int(bsBits>>(2*i)) & 3
			u[i] = uint8(bS[i])
		}
		var first, last int
		if vertical {
			first, last = off-2, off+7*stride+1
		} else {
			first, last = off-2*stride, off+stride+7
		}
		ok := indexA <= 51 && indexB <= 51 && stride >= 8 && first >= 0 && last < len(plane)

		got := append([]byte(nil), plane...)
		var panicked bool
		func() {
			defer func() { panicked = recover() != nil }()
			switch {
			case strong && vertical:
				DeblockChromaIntraV(got, off, stride, indexA, indexB)
			case strong:
				DeblockChromaIntraH(got, off, stride, indexA, indexB)
			case vertical:
				DeblockChromaV(got, off, stride, indexA, indexB, u)
			default:
				DeblockChromaH(got, off, stride, indexA, indexB, u)
			}
		}()
		if panicked == ok {
			t.Fatalf("ok=%v panicked=%v", ok, panicked)
		}
		if panicked {
			if !bytes.Equal(got, plane) {
				t.Fatal("a refused call wrote to the plane")
			}
			return
		}
		want := append([]byte(nil), plane...)
		if strong {
			bS = [4]int{4, 4, 4, 4}
		}
		oracleChromaEdge(want, off, stride, vertical, indexA, indexB, bS)
		if !bytes.Equal(got, want) {
			t.Fatalf("strong=%v vertical=%v: first difference at %d", strong, vertical, firstDiff(got, want))
		}
	})
}

func BenchmarkChroma(b *testing.B) {
	pl := make([]byte, planeStride*planeRows)
	for i := range pl {
		pl[i] = 100
	}
	bS := [4]uint8{3, 2, 1, 3}
	for _, impl := range []string{"dispatch", "portable"} {
		for _, vertical := range []bool{true, false} {
			off := edgeOffset(vertical, 0)
			dir := map[bool]string{true: "V", false: "H"}[vertical]
			b.Run(fmt.Sprintf("%s/normal%s", impl, dir), func(b *testing.B) {
				b.SetBytes(8)
				for i := 0; i < b.N; i++ {
					if impl == "portable" {
						deblockChromaNormalRef(pl, off, planeStride, vertical, 40, 30, bS)
					} else if vertical {
						DeblockChromaV(pl, off, planeStride, 40, 30, bS)
					} else {
						DeblockChromaH(pl, off, planeStride, 40, 30, bS)
					}
				}
			})
			b.Run(fmt.Sprintf("%s/strong%s", impl, dir), func(b *testing.B) {
				b.SetBytes(8)
				for i := 0; i < b.N; i++ {
					if impl == "portable" {
						deblockChromaStrongRef(pl, off, planeStride, vertical, 40, 30)
					} else if vertical {
						DeblockChromaIntraV(pl, off, planeStride, 40, 30)
					} else {
						DeblockChromaIntraH(pl, off, planeStride, 40, 30)
					}
				}
			})
		}
	}
}
