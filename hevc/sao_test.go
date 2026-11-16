package hevc

import (
	"errors"
	"math/rand"
	"testing"
)

// The model of the text: Equations 8-411 and 8-412 (edge) and 8-414 (band), written as they
// read, with SaoOffsetVal indexed 1..4 and edgeIdx remapped as the text does.
func modelEdge(cur, a, b int, off [4]int16, bd int) int {
	sgn := func(v int) int {
		if v > 0 {
			return 1
		}
		if v < 0 {
			return -1
		}
		return 0
	}
	edgeIdx := 2 + sgn(cur-a) + sgn(cur-b) // (8-411)
	if edgeIdx == 0 || edgeIdx == 1 || edgeIdx == 2 {
		if edgeIdx == 2 {
			edgeIdx = 0
		} else {
			edgeIdx++ // (8-412)
		}
	}
	v := cur
	if edgeIdx != 0 {
		v += int(off[edgeIdx-1])
	}
	return min(max(v, 0), 1<<uint(bd)-1)
}

func modelBand(cur, bandPos int, off [4]int16, bd int) int {
	var table [32]int
	for k := 0; k < 4; k++ {
		table[(k+bandPos)&31] = k + 1 // (8-414)
	}
	idx := table[cur>>uint(bd-5)]
	v := cur
	if idx != 0 {
		v += int(off[idx-1])
	}
	return min(max(v, 0), 1<<uint(bd)-1)
}

func randomSAOOffsets(rng *rand.Rand) [4]int16 {
	var o [4]int16
	for i := range o {
		o[i] = int16(rng.Intn(249) - 124)
	}
	return o
}

func TestSAOBandAgainstTheText(t *testing.T) {
	rng := rand.New(rand.NewSource(21))
	for iter := 0; iter < 6000; iter++ {
		w, h := 1+rng.Intn(64), 1+rng.Intn(64)
		bd := 8 + rng.Intn(5)
		bandPos := rng.Intn(32)
		off := randomSAOOffsets(rng)
		dStride, sStride := w+rng.Intn(9), w+rng.Intn(9)
		src := make([]uint16, (h-1)*sStride+w+rng.Intn(5))
		for i := range src {
			switch iter % 3 {
			case 0:
				src[i] = uint16(rng.Intn(1 << uint(bd)))
			case 1: // the four selected bands only, so the offsets apply often
				band := (bandPos + rng.Intn(4)) & 31
				src[i] = uint16(band<<uint(bd-5) | rng.Intn(1<<uint(bd-5)))
			default:
				src[i] = [2]uint16{0, uint16(1<<uint(bd) - 1)}[rng.Intn(2)]
			}
		}
		dst := make([]uint16, (h-1)*dStride+w+rng.Intn(5))
		for i := range dst {
			dst[i] = sentinel16
		}
		if err := TrySAOBand(dst, 0, dStride, src, 0, sStride, w, h, bd, bandPos, &off); err != nil {
			t.Fatalf("iter %d: %v", iter, err)
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if want := modelBand(int(src[y*sStride+x]), bandPos, off, bd); int(dst[y*dStride+x]) != want {
					t.Fatalf("iter %d: %dx%d bit depth %d band %d: sample (%d,%d) is %d, the text gives %d", iter, w, h, bd, bandPos, x, y, dst[y*dStride+x], want)
				}
			}
		}
		for i := range dst { // outside the block nothing changes
			y, x := i/dStride, i%dStride
			if (x >= w || y >= h) && dst[i] != sentinel16 {
				t.Fatalf("iter %d: a sample outside the block was written", iter)
			}
		}
	}
}

func TestSAOEdgeAgainstTheText(t *testing.T) {
	rng := rand.New(rand.NewSource(22))
	counts := map[int]int{}
	for iter := 0; iter < 8000; iter++ {
		w, h := 1+rng.Intn(64), 1+rng.Intn(64)
		bd := 8 + rng.Intn(5)
		class := rng.Intn(4)
		off := randomSAOOffsets(rng)
		rows, cols := saoMargins(class)
		sStride := w + 2*cols + rng.Intn(5)
		dStride := w + rng.Intn(5)
		srcOff := rows*sStride + cols
		src := make([]uint16, srcOff+(h-1+rows)*sStride+w+cols+rng.Intn(3))
		span := 1 << uint(bd)
		if iter%2 == 0 {
			span = 4 // few values, so the neighbours tie and the edge values -2..2 all occur
		}
		for i := range src {
			src[i] = uint16(rng.Intn(span)) + uint16(1<<uint(bd)-span)*uint16(rng.Intn(2))
		}
		dst := make([]uint16, (h-1)*dStride+w+rng.Intn(5))
		for i := range dst {
			dst[i] = sentinel16
		}
		if err := TrySAOEdge(dst, 0, dStride, src, srcOff, sStride, w, h, bd, class, &off); err != nil {
			t.Fatalf("iter %d: %v", iter, err)
		}
		hPos := [4][2]int{{-1, 1}, {0, 0}, {-1, 1}, {1, -1}}
		vPos := [4][2]int{{0, 0}, {-1, 1}, {-1, 1}, {-1, 1}}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				i := srcOff + y*sStride + x
				a := int(src[i+vPos[class][0]*sStride+hPos[class][0]])
				b := int(src[i+vPos[class][1]*sStride+hPos[class][1]])
				cur := int(src[i])
				want := modelEdge(cur, a, b, off, bd)
				counts[2+bool2(cur > a)-bool2(cur < a)+bool2(cur > b)-bool2(cur < b)]++
				if int(dst[y*dStride+x]) != want {
					t.Fatalf("iter %d: %dx%d bit depth %d class %d: sample (%d,%d) is %d, the text gives %d", iter, w, h, bd, class, x, y, dst[y*dStride+x], want)
				}
			}
		}
	}
	for e := 0; e <= 4; e++ {
		if counts[e] == 0 {
			t.Errorf("edge value %d never occurred", e-2)
		}
	}
}

func bool2(b bool) int {
	if b {
		return 1
	}
	return 0
}

func TestSAORefusals(t *testing.T) {
	off := [4]int16{1, 2, 3, 4}
	dst := make([]uint16, 4096)
	src := make([]uint16, 8192)
	for name, f := range map[string]func() error{
		"band: width 0":            func() error { return TrySAOBand(dst, 0, 64, src, 0, 80, 0, 4, 8, 3, &off) },
		"band: width past 64":      func() error { return TrySAOBand(dst, 0, 128, src, 0, 160, 65, 4, 8, 3, &off) },
		"band: height 0":           func() error { return TrySAOBand(dst, 0, 64, src, 0, 80, 8, 0, 8, 3, &off) },
		"band: bit depth 13":       func() error { return TrySAOBand(dst, 0, 64, src, 0, 80, 8, 4, 13, 3, &off) },
		"band: band position 32":   func() error { return TrySAOBand(dst, 0, 64, src, 0, 80, 8, 4, 8, 32, &off) },
		"band: band position -1":   func() error { return TrySAOBand(dst, 0, 64, src, 0, 80, 8, 4, 8, -1, &off) },
		"band: no offsets":         func() error { return TrySAOBand(dst, 0, 64, src, 0, 80, 8, 4, 8, 3, nil) },
		"band: offset too large":   func() error { return TrySAOBand(dst, 0, 64, src, 0, 80, 8, 4, 8, 3, &[4]int16{0, 0, 0, 5000}) },
		"band: destination stride": func() error { return TrySAOBand(dst, 0, 4, src, 0, 80, 8, 4, 8, 3, &off) },
		"band: source stride 0":    func() error { return TrySAOBand(dst, 0, 64, src, 0, 0, 8, 4, 8, 3, &off) },
		"band: destination fit":    func() error { return TrySAOBand(dst, 4090, 64, src, 0, 80, 8, 4, 8, 3, &off) },
		"band: source fit":         func() error { return TrySAOBand(dst, 0, 64, src, 8190, 80, 8, 4, 8, 3, &off) },
		"band: stride that wraps":  func() error { return TrySAOBand(dst, 0, 64, src, 0, maxInt, 8, 4, 8, 3, &off) },
		"edge: class 4":            func() error { return TrySAOEdge(dst, 0, 64, src, 81, 80, 8, 4, 8, 4, &off) },
		"edge: class -1":           func() error { return TrySAOEdge(dst, 0, 64, src, 81, 80, 8, 4, 8, -1, &off) },
		"edge: margin before":      func() error { return TrySAOEdge(dst, 0, 64, src, 80, 80, 8, 4, 8, 2, &off) },
		"edge: margin after":       func() error { return TrySAOEdge(dst, 0, 64, src, 8192-8-80, 80, 8, 1, 8, 2, &off) },
		"edge: bit depth 7":        func() error { return TrySAOEdge(dst, 0, 64, src, 81, 80, 8, 4, 7, 2, &off) },
	} {
		for i := range dst {
			dst[i] = sentinel16
		}
		if err := f(); !errors.Is(err, ErrRefused) {
			t.Errorf("%s: %v", name, err)
		}
		for i := range dst {
			if dst[i] != sentinel16 {
				t.Fatalf("%s: a refused call wrote dst[%d]", name, i)
			}
		}
	}
	mem := make([]uint16, 4096)
	if err := TrySAOBand(mem, 0, 64, mem, 32, 64, 8, 4, 8, 3, &off); !errors.Is(err, ErrRefused) {
		t.Errorf("overlap: %v", err)
	}
	mustPanic(t, "SAOBand", func() { SAOBand(dst, 0, 64, src, 0, 80, 0, 4, 8, 3, &off) })
	mustPanic(t, "SAOEdge", func() { SAOEdge(dst, 0, 64, src, 0, 80, 8, 4, 8, 7, &off) })
	SAOBand(dst, 0, 64, src, 0, 80, 8, 4, 8, 3, &off)
	SAOEdge(dst, 0, 64, src, 81, 80, 8, 4, 8, 2, &off)
}
