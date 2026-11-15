package lf

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
)

// The oracle is a per-line transcription of ITU-T H.264 (V16) (06/2026)
// clauses 8.7.1 (Equations 8-442 to 8-449, the p and q sample positions),
// 8.7.2 (8-451, 8-452), 8.7.2.2 (8-453 to 8-460, Table 8-16), 8.7.2.3 (8-461
// to 8-475, Table 8-17) and 8.7.2.4 (8-476 to 8-489). It computes the whole
// output p'0..2, q'0..2 for every line and writes them back, instead of
// writing only the samples that change.

// Table 8-16, in the two rows of 26 the standard prints (indexA 0..25, then
// 26..51), α′ then β′.
var oracleAlpha = [2][26]int{
	{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 4, 4, 5, 6, 7, 8, 9, 10, 12, 13},
	{15, 17, 20, 22, 25, 28, 32, 36, 40, 45, 50, 56, 63, 71, 80, 90, 101, 113, 127, 144, 162, 182, 203, 226, 255, 255},
}
var oracleBeta = [2][26]int{
	{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4},
	{6, 6, 7, 7, 8, 8, 9, 9, 10, 10, 11, 11, 12, 12, 13, 13, 14, 14, 15, 15, 16, 16, 17, 17, 18, 18},
}

// Table 8-17, rows bS = 1, 2, 3, each in the two halves.
var oracleTC0 = [3][2][26]int{
	{ // bS = 1
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1},
		{1, 1, 1, 1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 4, 4, 4, 5, 6, 6, 7, 8, 9, 10, 11, 13},
	},
	{ // bS = 2
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1},
		{1, 1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 4, 4, 5, 5, 6, 7, 8, 8, 10, 11, 12, 13, 15, 17},
	},
	{ // bS = 3
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		{1, 2, 2, 2, 2, 3, 3, 3, 4, 4, 4, 5, 6, 6, 7, 8, 9, 10, 11, 13, 14, 16, 18, 20, 23, 25},
	},
}

func half(t [2][26]int, i int) int { return t[i/26][i%26] }

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func oclip3(lo, hi, v int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// filterSet is clause 8.7.2 on one set of eight samples across the edge: it
// returns p'0..2 and q'0..2. bS 0 means no filtering.
func filterSet(p, q [4]int, bS, indexA, indexB int) (pp, qq [3]int) {
	for i := 0; i < 3; i++ { // 8-451, 8-452 by default
		pp[i], qq[i] = p[i], q[i]
	}
	alpha, beta := half(oracleAlpha, indexA), half(oracleBeta, indexB)
	if !(bS != 0 && absInt(p[0]-q[0]) < alpha && absInt(p[1]-p[0]) < beta && absInt(q[1]-q[0]) < beta) { // 8-460
		return
	}
	ap, aq := absInt(p[2]-p[0]), absInt(q[2]-q[0])
	if bS < 4 {
		tc0 := half(oracleTC0[bS-1], indexA)
		tc := tc0
		if ap < beta {
			tc++
		}
		if aq < beta {
			tc++
		}
		delta := oclip3(-tc, tc, (((q[0]-p[0])<<2)+(p[1]-q[1])+4)>>3)
		pp[0] = oclip3(0, 255, p[0]+delta)
		qq[0] = oclip3(0, 255, q[0]-delta)
		if ap < beta {
			pp[1] = p[1] + oclip3(-tc0, tc0, (p[2]+((p[0]+q[0]+1)>>1)-(p[1]<<1))>>1)
		}
		if aq < beta {
			qq[1] = q[1] + oclip3(-tc0, tc0, (q[2]+((p[0]+q[0]+1)>>1)-(q[1]<<1))>>1)
		}
		return
	}
	if ap < beta && absInt(p[0]-q[0]) < ((alpha>>2)+2) {
		pp[0] = (p[2] + 2*p[1] + 2*p[0] + 2*q[0] + q[1] + 4) >> 3
		pp[1] = (p[2] + p[1] + p[0] + q[0] + 2) >> 2
		pp[2] = (2*p[3] + 3*p[2] + p[1] + p[0] + q[0] + 4) >> 3
	} else {
		pp[0] = (2*p[1] + p[0] + q[1] + 2) >> 2
	}
	if aq < beta && absInt(p[0]-q[0]) < ((alpha>>2)+2) {
		qq[0] = (p[1] + 2*p[0] + 2*q[0] + 2*q[1] + q[2] + 4) >> 3
		qq[1] = (p[0] + q[0] + q[1] + q[2] + 2) >> 2
		qq[2] = (2*q[3] + 3*q[2] + q[1] + q[0] + p[0] + 4) >> 3
	} else {
		qq[0] = (2*q[1] + q[0] + p[1] + 2) >> 2
	}
	return
}

// oracleEdge filters 16 lines in place; bS gives the strength of each group
// of four lines (the strength of every line when bS4).
func oracleEdge(pix []byte, off, stride int, vertical bool, indexA, indexB int, bS [4]int) {
	for k := 0; k < 16; k++ {
		// (x, y) of the sample i across the edge: q_i is i steps from q0, p_i
		// is i+1 steps before it (8-442..8-445).
		at := func(i int) int { // i >= 0: q_i; i < 0: p_(-i-1)
			if vertical {
				return off + k*stride + i
			}
			return off + k + i*stride
		}
		var p, q [4]int
		for i := 0; i < 4; i++ {
			q[i] = int(pix[at(i)])
			p[i] = int(pix[at(-i-1)])
		}
		pp, qq := filterSet(p, q, bS[k/4], indexA, indexB)
		for i := 0; i < 3; i++ { // 8-446..8-449
			pix[at(i)] = byte(qq[i])
			pix[at(-i-1)] = byte(pp[i])
		}
	}
}

func TestTables(t *testing.T) {
	for i := 0; i < 52; i++ {
		if Alpha(i) != half(oracleAlpha, i) || Beta(i) != half(oracleBeta, i) {
			t.Errorf("index %d: alpha %d beta %d", i, Alpha(i), Beta(i))
		}
		for bS := 1; bS <= 3; bS++ {
			if TC0(bS, i) != half(oracleTC0[bS-1], i) {
				t.Errorf("tc0 bS %d index %d: %d", bS, i, TC0(bS, i))
			}
		}
	}
	for name, f := range map[string]func(){
		"alpha -1": func() { Alpha(-1) }, "alpha 52": func() { Alpha(52) },
		"beta -1": func() { Beta(-1) }, "beta 52": func() { Beta(52) },
		"tc0 bS 0": func() { TC0(0, 10) }, "tc0 bS 4": func() { TC0(4, 10) }, "tc0 index 52": func() { TC0(1, 52) },
	} {
		mustPanic(t, name, f)
	}
}

func mustPanic(t *testing.T, what string, f func()) {
	t.Helper()
	defer func() {
		t.Helper()
		if recover() == nil {
			t.Errorf("%s: no panic", what)
		}
	}()
	f()
}

const (
	planeStride = 40
	planeRows   = 32
)

// fillEdges draws a plane whose samples across the edges are close enough
// for the filters to fire: per call a noise amplitude, and a step between the
// two sides.
func fillEdges(r *rand.Rand, pl []byte) {
	amps := []int{0, 1, 2, 4, 8, 16, 40, 255}
	noise := amps[r.Intn(len(amps))]
	step := amps[r.Intn(len(amps))]
	for y := 0; y < planeRows; y++ {
		base := r.Intn(256)
		for x := 0; x < planeStride; x++ {
			side := 0
			if x >= 16 { // the vertical edge is at x = 16; the horizontal at y = 16
				side += step
			}
			if y >= 16 {
				side += step
			}
			v := base + side + r.Intn(2*noise+1) - noise
			if r.Intn(40) == 0 {
				v = 255 * r.Intn(2) // extremes
			}
			pl[y*planeStride+x] = byte(oclip3(0, 255, v))
		}
	}
}

func edgeOffset(vertical bool, rowOff int) int {
	if vertical {
		return (4+rowOff)*planeStride + 16 // rows 4+.., edge between columns 15 and 16
	}
	return 16*planeStride + 4 + rowOff // edge between rows 15 and 16, columns 4+..
}

func runEdge(t *testing.T, r *rand.Rand, vertical bool, indexA, indexB int, bS [4]int, strong bool) {
	t.Helper()
	pl := make([]byte, planeStride*planeRows)
	fillEdges(r, pl)
	off := edgeOffset(vertical, r.Intn(4))
	want := append([]byte(nil), pl...)
	oracleEdge(want, off, planeStride, vertical, indexA, indexB, bS)

	got := append([]byte(nil), pl...)
	ref := append([]byte(nil), pl...)
	var u [4]uint8
	for i := range u {
		u[i] = uint8(bS[i])
	}
	switch {
	case strong && vertical:
		DeblockLumaIntraV(got, off, planeStride, indexA, indexB)
		deblockStrongRef(ref, off, planeStride, true, indexA, indexB)
	case strong:
		DeblockLumaIntraH(got, off, planeStride, indexA, indexB)
		deblockStrongRef(ref, off, planeStride, false, indexA, indexB)
	case vertical:
		DeblockLumaV(got, off, planeStride, indexA, indexB, u)
		deblockNormalRef(ref, off, planeStride, true, indexA, indexB, u)
	default:
		DeblockLumaH(got, off, planeStride, indexA, indexB, u)
		deblockNormalRef(ref, off, planeStride, false, indexA, indexB, u)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("vertical=%v strong=%v indexA %d indexB %d bS %v: first difference at %d (got %d want %d)", vertical, strong, indexA, indexB, bS, firstDiff(got, want), got[firstDiff(got, want)], want[firstDiff(got, want)])
	}
	if !bytes.Equal(ref, want) {
		t.Fatalf("portable vertical=%v strong=%v indexA %d indexB %d bS %v: first difference at %d", vertical, strong, indexA, indexB, bS, firstDiff(ref, want))
	}
}

func firstDiff(a, b []byte) int {
	for i := range a {
		if a[i] != b[i] {
			return i
		}
	}
	return -1
}

// Every indexA and indexB, both directions, and strength patterns covering
// every value in every group.
func TestDeblockExhaustive(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	patterns := [][4]int{
		{1, 1, 1, 1}, {2, 2, 2, 2}, {3, 3, 3, 3}, {0, 0, 0, 0},
		{0, 1, 2, 3}, {3, 2, 1, 0}, {1, 0, 3, 2}, {2, 3, 0, 1},
	}
	for _, vertical := range []bool{true, false} {
		t.Run(fmt.Sprintf("vertical=%v", vertical), func(t *testing.T) {
			for a := 0; a < 52; a++ {
				for b := 0; b < 52; b++ {
					for _, p := range patterns {
						runEdge(t, r, vertical, a, b, p, false)
					}
					for i := 0; i < 4; i++ {
						runEdge(t, r, vertical, a, b, [4]int{r.Intn(4), r.Intn(4), r.Intn(4), r.Intn(4)}, false)
					}
					for i := 0; i < 3; i++ {
						runEdge(t, r, vertical, a, b, [4]int{4, 4, 4, 4}, true)
					}
				}
			}
		})
	}
}

func TestDeblockRefused(t *testing.T) {
	pl := make([]byte, 64*64)
	var z [4]uint8
	type call func()
	cases := map[string]call{
		"V stride 15":     func() { DeblockLumaV(pl, 4*64+8, 15, 30, 30, z) },
		"H stride 15":     func() { DeblockLumaH(pl, 4*64+8, 15, 30, 30, z) },
		"V indexA 52":     func() { DeblockLumaV(pl, 4*64+8, 64, 52, 30, z) },
		"V indexB -1":     func() { DeblockLumaV(pl, 4*64+8, 64, 30, -1, z) },
		"V bS 4":          func() { DeblockLumaV(pl, 4*64+8, 64, 30, 30, [4]uint8{0, 0, 4, 0}) },
		"H bS 5":          func() { DeblockLumaH(pl, 4*64+8, 64, 30, 30, [4]uint8{5, 0, 0, 0}) },
		"V left of plane": func() { DeblockLumaV(pl, 3, 64, 30, 30, z) },
		"V past end":      func() { DeblockLumaV(pl, 64*64-15*64-3, 64, 30, 30, z) },
		"H above plane":   func() { DeblockLumaH(pl, 3*64+8, 64, 30, 30, z) },
		"H past end":      func() { DeblockLumaH(pl, 64*64-3*64-15, 64, 30, 30, z) },
		"intra V stride":  func() { DeblockLumaIntraV(pl, 4*64+8, 8, 30, 30) },
		"intra H index":   func() { DeblockLumaIntraH(pl, 4*64+8, 64, 99, 30) },
		"intra V past":    func() { DeblockLumaIntraV(pl, 64*64-15*64-3, 64, 30, 30) },
		"intra H above":   func() { DeblockLumaIntraH(pl, 3*64+8, 64, 30, 30) },
		"empty":           func() { DeblockLumaV(nil, 0, 16, 0, 0, z) },
	}
	for name, f := range cases {
		mustPanic(t, name, f)
	}
	// Exactly fitting edges are fine.
	DeblockLumaV(pl, 4, 64, 30, 30, z)
	DeblockLumaV(pl, 64*64-15*64-4, 64, 30, 30, z)
	DeblockLumaH(pl, 4*64, 64, 30, 30, z)
	DeblockLumaH(pl, 64*64-4*64, 64, 30, 30, z)
}

// Fuzz the edge position and parameters: the call panics exactly when the
// footprint, stride, index or strength is out of range, and otherwise equals
// the oracle with nothing outside the edge touched.
func FuzzDeblock(f *testing.F) {
	r := rand.New(rand.NewSource(2))
	pl := make([]byte, planeStride*planeRows)
	fillEdges(r, pl)
	f.Add(pl, uint16(edgeOffset(true, 0)), uint16(planeStride), uint8(30), uint8(30), uint8(0x1b), false, true)
	f.Add(pl, uint16(edgeOffset(false, 0)), uint16(planeStride), uint8(51), uint8(51), uint8(0xff), true, false)
	f.Add(pl, uint16(edgeOffset(true, 1)), uint16(planeStride), uint8(40), uint8(20), uint8(0), true, true)
	f.Add(pl[:300], uint16(200), uint16(16), uint8(10), uint8(60), uint8(0x55), false, false)

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
			first, last = off-4, off+15*stride+3
		} else {
			first, last = off-4*stride, off+3*stride+15
		}
		ok := indexA <= 51 && indexB <= 51 && stride >= 16 && first >= 0 && last < len(plane)

		got := append([]byte(nil), plane...)
		var panicked bool
		func() {
			defer func() { panicked = recover() != nil }()
			switch {
			case strong && vertical:
				DeblockLumaIntraV(got, off, stride, indexA, indexB)
			case strong:
				DeblockLumaIntraH(got, off, stride, indexA, indexB)
			case vertical:
				DeblockLumaV(got, off, stride, indexA, indexB, u)
			default:
				DeblockLumaH(got, off, stride, indexA, indexB, u)
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
		oracleEdge(want, off, stride, vertical, indexA, indexB, bS)
		if !bytes.Equal(got, want) {
			t.Fatalf("strong=%v vertical=%v: first difference at %d", strong, vertical, firstDiff(got, want))
		}
	})
}

func BenchmarkDeblock(b *testing.B) {
	// A flat plane is a fixed point of every filter and makes each line run
	// the whole formula, so repeated calls measure the worst case.
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
				b.SetBytes(16)
				for i := 0; i < b.N; i++ {
					if impl == "portable" {
						deblockNormalRef(pl, off, planeStride, vertical, 40, 30, bS)
					} else if vertical {
						DeblockLumaV(pl, off, planeStride, 40, 30, bS)
					} else {
						DeblockLumaH(pl, off, planeStride, 40, 30, bS)
					}
				}
			})
			b.Run(fmt.Sprintf("%s/strong%s", impl, dir), func(b *testing.B) {
				b.SetBytes(16)
				for i := 0; i < b.N; i++ {
					if impl == "portable" {
						deblockStrongRef(pl, off, planeStride, vertical, 40, 30)
					} else if vertical {
						DeblockLumaIntraV(pl, off, planeStride, 40, 30)
					} else {
						DeblockLumaIntraH(pl, off, planeStride, 40, 30)
					}
				}
			})
		}
	}
}
