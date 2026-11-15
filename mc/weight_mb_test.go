package mc

import (
	"bytes"
	"math"
	"math/rand"
	"testing"

	"github.com/go-videodsp/kernels/internal/boundstest"
)

// rect is a partition's position and size inside the macroblock.
type rect struct{ x, y, w, h int }

// mbLayout draws one of the partitionings of 7.4.5 and 7.4.5.2: 16x16,
// 16x8, 8x16, or four 8x8 each kept or split into 8x4, 4x8 or 4x4.
func mbLayout(r *rand.Rand) []rect {
	switch r.Intn(4) {
	case 0:
		return []rect{{0, 0, 16, 16}}
	case 1:
		return []rect{{0, 0, 16, 8}, {0, 8, 16, 8}}
	case 2:
		return []rect{{0, 0, 8, 16}, {8, 0, 8, 16}}
	}
	var out []rect
	for q := 0; q < 4; q++ {
		x, y := q%2*8, q/2*8
		switch r.Intn(4) {
		case 0:
			out = append(out, rect{x, y, 8, 8})
		case 1:
			out = append(out, rect{x, y, 8, 4}, rect{x, y + 4, 8, 4})
		case 2:
			out = append(out, rect{x, y, 4, 8}, rect{x + 4, y, 4, 8})
		default:
			out = append(out, rect{x, y, 4, 4}, rect{x + 4, y, 4, 4}, rect{x, y + 4, 4, 4}, rect{x + 4, y + 4, 4, 4})
		}
	}
	return out
}

func uniParts(r *rand.Rand, rs []rect) []UniPart {
	var ps []UniPart
	for _, q := range rs {
		ps = append(ps, UniPart{q.x, q.y, q.w, q.h, r.Intn(257) - 128, r.Intn(256) - 128})
	}
	return ps
}

func biParts(r *rand.Rand, rs []rect) []BiPart {
	var ps []BiPart
	for _, q := range rs {
		w := r.Intn(257) - 128
		if r.Intn(4) == 0 {
			w = 32 // with logWD 5 and zero offsets: the plain average path
		}
		ps = append(ps, BiPart{q.x, q.y, q.w, q.h, w, []int{w, r.Intn(257) - 128}[r.Intn(2)], r.Intn(256) - 128, r.Intn(256) - 128})
	}
	return ps
}

// Every layout, random weights, offsets, strides and plane contents: the
// macroblock call must equal the spec formula applied per sample (the oracle of
// weight_test.go), and leave every sample outside the partitions alone.
func TestWeightUniMBEqualsTheFormula(t *testing.T) {
	r := rand.New(rand.NewSource(31))
	for i := 0; i < 3000; i++ {
		logWD := r.Intn(8)
		stride := 16 + r.Intn(40)
		off := r.Intn(3 * stride)
		n := off + 15*stride + 16 + r.Intn(10)
		pl := make([]byte, n)
		fill(r, pl, fillKind(r.Intn(int(fillCount))), stride)
		want := append([]byte(nil), pl...)
		parts := uniParts(r, mbLayout(r))
		for _, p := range parts {
			for y := 0; y < p.H; y++ {
				for x := 0; x < p.W; x++ {
					j := off + (p.Y+y)*stride + p.X + x
					want[j] = weightedUni(int(want[j]), p.Weight, p.Offset, logWD)
				}
			}
		}
		WeightUniMB(pl, off, stride, parts, logWD)
		if !bytes.Equal(pl, want) {
			t.Fatalf("round %d logWD %d stride %d: first difference at %d", i, logWD, stride, firstDiff(pl, want))
		}
	}
}

func TestWeightBiMBEqualsTheFormula(t *testing.T) {
	r := rand.New(rand.NewSource(32))
	for i := 0; i < 3000; i++ {
		logWD := []int{5, r.Intn(8)}[r.Intn(2)]
		ds, ss := 16+r.Intn(40), 16+r.Intn(40)
		dOff, sOff := r.Intn(3*ds), r.Intn(3*ss)
		dst := make([]byte, dOff+15*ds+16+r.Intn(10))
		src := make([]byte, sOff+15*ss+16+r.Intn(10))
		fill(r, dst, fillKind(r.Intn(int(fillCount))), ds)
		fill(r, src, fillKind(r.Intn(int(fillCount))), ss)
		want := append([]byte(nil), dst...)
		parts := biParts(r, mbLayout(r))
		for _, p := range parts {
			for y := 0; y < p.H; y++ {
				for x := 0; x < p.W; x++ {
					j, k := dOff+(p.Y+y)*ds+p.X+x, sOff+(p.Y+y)*ss+p.X+x
					want[j] = weightedBi(int(want[j]), int(src[k]), p.W0, p.W1, p.O0, p.O1, logWD)
				}
			}
		}
		WeightBiMB(dst, dOff, ds, src, sOff, ss, parts, logWD)
		if !bytes.Equal(dst, want) {
			t.Fatalf("round %d logWD %d: first difference at %d", i, logWD, firstDiff(dst, want))
		}
	}
}

// What must be refused, by assertion, with the destination untouched.
func TestWeightMBRefusals(t *testing.T) {
	good := []UniPart{{0, 0, 16, 16, 3, 1}}
	goodBi := []BiPart{{0, 0, 16, 16, 3, 5, 1, 1}}
	base := func() []byte { return bytes.Repeat([]byte{7}, 16*16+8) }
	untouched := func(name string, f func(p []byte)) {
		t.Helper()
		p := base()
		if !refused(func() { f(p) }) {
			t.Errorf("%s: accepted", name)
		}
		if !bytes.Equal(p, base()) {
			t.Errorf("%s: a refused call wrote to the destination", name)
		}
	}
	uni := func(name string, off, stride int, parts []UniPart, logWD int) {
		untouched(name, func(p []byte) { WeightUniMB(p, off, stride, parts, logWD) })
	}
	uni("stride 15", 0, 15, good, 2)
	uni("short plane", 9, 16, good, 2)
	uni("negative offset", -1, 16, good, 2)
	uni("logWD 8", 0, 16, good, 8)
	uni("logWD -1", 0, 16, good, -1)
	uni("17 partitions", 0, 16, make([]UniPart, 17), 2)
	for name, p := range map[string]UniPart{
		"width 12":       {0, 0, 12, 8, 1, 0},
		"16x4":           {0, 0, 16, 4, 1, 0},
		"x outside":      {9, 0, 8, 8, 1, 0},
		"y outside":      {0, 9, 8, 8, 1, 0},
		"negative x":     {-1, 0, 8, 8, 1, 0},
		"huge x":         {math.MaxInt, 0, 8, 8, 1, 0},
		"weight 129":     {0, 0, 8, 8, 129, 0},
		"weight -129":    {0, 0, 8, 8, -129, 0},
		"weight MaxInt":  {0, 0, 8, 8, math.MaxInt, 0},
		"offset 128":     {0, 0, 8, 8, 1, 128},
		"offset -129":    {0, 0, 8, 8, 1, -129},
		"offset MinInt":  {0, 0, 8, 8, 1, math.MinInt},
		"zero size":      {0, 0, 0, 0, 1, 0},
		"negative width": {0, 0, -4, 8, 1, 0},
	} {
		uni(name, 0, 16, []UniPart{p}, 2)
		uni(name+" after a good one", 0, 16, []UniPart{{8, 8, 8, 8, 3, 1}, p}, 2)
	}
	uni("overlap", 0, 16, []UniPart{{0, 0, 8, 8, 1, 0}, {4, 4, 8, 8, 1, 0}}, 2)
	uni("same partition twice", 0, 16, []UniPart{{0, 0, 4, 4, 1, 0}, {0, 0, 4, 4, 1, 0}}, 2)

	bi := func(name string, doff, ds, soff, ss int, parts []BiPart, logWD int) {
		untouched(name, func(p []byte) { WeightBiMB(p, doff, ds, base(), soff, ss, parts, logWD) })
	}
	bi("dst stride 15", 0, 15, 0, 16, goodBi, 2)
	bi("src stride 15", 0, 16, 0, 15, goodBi, 2)
	bi("src short", 0, 16, 9, 16, goodBi, 2)
	bi("logWD 9", 0, 16, 0, 16, goodBi, 9)
	bi("17 partitions", 0, 16, 0, 16, make([]BiPart, 17), 2)
	bi("w1 129", 0, 16, 0, 16, []BiPart{{0, 0, 16, 16, 1, 129, 0, 0}}, 2)
	bi("o1 -129", 0, 16, 0, 16, []BiPart{{0, 0, 16, 16, 1, 1, 0, -129}}, 2)
	bi("overlap", 0, 16, 0, 16, []BiPart{{0, 0, 8, 8, 1, 1, 0, 0}, {0, 4, 8, 8, 1, 1, 0, 0}}, 2)
	// Shared memory: the same array, overlapping macroblock windows.
	shared := make([]byte, 16*16+16*4)
	if !refused(func() { WeightBiMB(shared, 0, 16, shared, 16*4, 16, goodBi, 2) }) {
		t.Error("overlapping destination and source macroblocks accepted")
	}
	// Disjoint windows of one array are fine.
	arr := make([]byte, 2*16*16)
	if refused(func() { WeightBiMB(arr[:16*16], 0, 16, arr[16*16:], 0, 16, goodBi, 2) }) {
		t.Error("disjoint macroblocks of one array refused")
	}
	// The empty macroblock (no partitions) is legal and changes nothing.
	p := base()
	WeightUniMB(p, 0, 16, nil, 2)
	if !bytes.Equal(p, base()) {
		t.Error("an empty partition list wrote to the destination")
	}
}

// The wrap of the security fix, on the macroblock entries: hostile strides and
// offsets of MaxInt, MaxInt/2+1 (1<<62 on 64-bit), -1 and MinInt are refused.
func TestWeightMBHostileStridesAndOffsets(t *testing.T) {
	buf := func() []byte { return make([]byte, 64) }
	for _, v := range hostile {
		for name, f := range map[string]func(){
			"uni stride": func() { WeightUniMB(buf(), 0, v, []UniPart{{0, 0, 16, 16, 1, 0}}, 2) },
			"uni offset": func() { WeightUniMB(buf(), v, 16, []UniPart{{0, 0, 16, 16, 1, 0}}, 2) },
			"bi dst":     func() { WeightBiMB(buf(), 0, v, buf(), 0, 16, []BiPart{{0, 0, 16, 16, 1, 1, 0, 0}}, 2) },
			"bi src":     func() { WeightBiMB(buf(), 0, 16, buf(), 0, v, []BiPart{{0, 0, 16, 16, 1, 1, 0, 0}}, 2) },
			"bi offsets": func() { WeightBiMB(buf(), v, 16, buf(), v, 16, []BiPart{{0, 0, 16, 16, 1, 1, 0, 0}}, 2) },
		} {
			if !refused(f) {
				t.Errorf("%s %d accepted", name, v)
			}
		}
	}
}

// brute is the partition rule written as the plain statement of it: every
// partition a supported size, inside the macroblock, no sample claimed twice.
func brute(rs []rect) bool {
	if len(rs) > 16 {
		return false
	}
	var grid [16][16]bool
	for _, q := range rs {
		if !validSize(q.w, q.h) || q.x < 0 || q.y < 0 || q.x > 16-q.w || q.y > 16-q.h {
			return false
		}
		for y := q.y; y < q.y+q.h; y++ {
			for x := q.x; x < q.x+q.w; x++ {
				if grid[y][x] {
					return false
				}
				grid[y][x] = true
			}
		}
	}
	return true
}

// FuzzWeightMBRefusal throws unconstrained integers (positions, sizes, strides,
// offsets, weights) at both macroblock entries and compares with the exact
// oracle: the macroblock window by math/big, the partitions by brute force.
func FuzzWeightMBRefusal(f *testing.F) {
	f.Add(300, 0, 16, 0, 0, 16, 16, 3, 1, 2)
	f.Add(300, 0, math.MaxInt/2+1, 0, 0, 16, 16, 3, 1, 2)
	f.Add(300, 0, 16, 4, 4, 8, 8, 129, 0, 2)
	f.Fuzz(func(t *testing.T, n, off, stride, x, y, w, h, weight, offset, logWD int) {
		if n < 0 || n > 4096 {
			return
		}
		dst, src := make([]byte, n), make([]byte, n)
		parts := []UniPart{{x, y, w, h, weight, offset}, {0, 0, 4, 4, 1, 0}}
		rs := []rect{{x, y, w, h}, {0, 0, 4, 4}}
		want := boundstest.Fits(n, off, stride, 0, 15, 0, 15) && stride >= 16 && brute(rs) &&
			weightOK(logWD, weight, offset)
		if accepted := !refused(func() { WeightUniMB(dst, off, stride, parts, logWD) }); accepted != want {
			t.Fatalf("WeightUniMB accepted=%v, exact oracle %v: n=%d off=%d stride=%d part=%v logWD=%d", accepted, want, n, off, stride, rs[0], logWD)
		}
		bparts := []BiPart{{x, y, w, h, weight, weight, offset, offset}, {0, 0, 4, 4, 1, 1, 0, 0}}
		bwant := want && boundstest.Fits(n, off, stride, 0, 15, 0, 15)
		if accepted := !refused(func() { WeightBiMB(dst, off, stride, src, off, stride, bparts, logWD) }); accepted != bwant {
			t.Fatalf("WeightBiMB accepted=%v, exact oracle %v", accepted, bwant)
		}
	})
}
