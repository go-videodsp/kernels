package hevc

import (
	"errors"
	"math/rand"
	"testing"
	"unsafe"
)

// The kernels that write the bytes of an 8-bit picture, against the text's formulas in 64 bits.

const sentinel8 = 0x5a

func fillBytes(n int, v uint8) []uint8 {
	s := make([]uint8, n)
	for i := range s {
		s[i] = v
	}
	return s
}

func clip8(v int64) uint8 { return uint8(min(max(v, 0), 255)) }

// modelWeightUni and modelWeightBi are (8-275) and (8-277) at bit depth 8.
func modelWeightUni(p int32, log2Wd int, wt, o int32) uint8 {
	return clip8(((int64(p)*int64(wt) + 1<<uint(log2Wd-1)) >> uint(log2Wd)) + int64(o))
}

func modelWeightBi(p0, p1 int32, log2Wd int, w0, w1, o0, o1 int32) uint8 {
	return clip8((int64(p0)*int64(w0) + int64(p1)*int64(w1) + (int64(o0)+int64(o1)+1)<<uint(log2Wd)) >> uint(log2Wd+1))
}

// modelSAOBand is (8-414) at bit depth 8.
func modelSAOBand(cur uint8, bandPos int, off *[4]int16) uint8 {
	band, d := int(cur)>>3, int64(0)
	for k := 0; k < 4; k++ {
		if band == (bandPos+k)&31 {
			d = int64(off[k])
		}
	}
	return clip8(int64(cur) + d)
}

// modelSAOEdge is (8-411) and (8-412).
func modelSAOEdge(cur, a, b uint8, off *[4]int16) uint8 {
	sg := func(x, y uint8) int {
		switch {
		case x > y:
			return 1
		case x < y:
			return -1
		}
		return 0
	}
	delta := [5]int64{int64(off[0]), int64(off[1]), 0, int64(off[2]), int64(off[3])}
	return clip8(int64(cur) + delta[sg(cur, a)+sg(cur, b)+2])
}

func randomByteSAOOffsets(rng *rand.Rand, iter int) *[4]int16 {
	var off [4]int16
	for i := range off {
		switch iter % 3 {
		case 0:
			off[i] = int16(rng.Intn(33) - 16)
		case 1:
			off[i] = [2]int16{-300, 300}[rng.Intn(2)]
		default:
			off[i] = int16(rng.Intn(2*saoMaxOffset+1) - saoMaxOffset)
		}
	}
	return &off
}

func TestAverageBytes(t *testing.T) {
	rng := rand.New(rand.NewSource(81))
	for iter := 0; iter < 4000; iter++ {
		w, h := 2*(1+rng.Intn(32)), 1+rng.Intn(64)
		aS, bS, dS := w+rng.Intn(9), w+rng.Intn(9), w+rng.Intn(9)
		a := randomBytes(rng, (h-1)*aS+w+rng.Intn(5), iter%7 == 0)
		b := randomBytes(rng, (h-1)*bS+w+rng.Intn(5), iter%11 == 0)
		if iter%5 == 0 {
			b = a // the same block twice is allowed
			bS = aS
		}
		dst := fillBytes((h-1)*dS+w+rng.Intn(5), sentinel8)
		if err := TryAverageBytes(dst, 0, dS, a, 0, aS, b, 0, bS, w, h); err != nil {
			t.Fatalf("iter %d: %v", iter, err)
		}
		for i := range dst {
			y, x := i/dS, i%dS
			want := uint8(sentinel8)
			if x < w && y < h {
				want = uint8((int(a[y*aS+x]) + int(b[y*bS+x]) + 1) >> 1)
			}
			if dst[i] != want {
				t.Fatalf("iter %d %dx%d: element %d is %d, want %d", iter, w, h, i, dst[i], want)
			}
		}
	}
}

func TestWeightBytes(t *testing.T) {
	rng := rand.New(rand.NewSource(82))
	for iter := 0; iter < 8000; iter++ {
		bi := iter%2 == 1
		w, h := 2*(1+rng.Intn(32)), 1+rng.Intn(64)
		log2Wd := minLog2Wd + rng.Intn(maxLog2Wd-minLog2Wd+1)
		pS, dS := w+rng.Intn(9), w+rng.Intn(9)
		n := (h-1)*pS + w + rng.Intn(5)
		p0, p1 := make([]int32, n), make([]int32, n)
		for i := range p0 {
			p0[i], p1[i] = int32(rng.Intn(1<<17)-1<<16), int32(rng.Intn(1<<17)-1<<16)
			if iter%9 == 0 { // the intermediates of 8-bit video: 0 .. 255 << 6, then past it
				p0[i], p1[i] = int32(rng.Intn(256)<<6), int32(rng.Intn(256)<<6)
			}
		}
		w0, w1 := int32(rng.Intn(257)-128), int32(rng.Intn(257)-128)
		o0, o1 := int32(rng.Intn(257)-128), int32(rng.Intn(257)-128)
		switch iter % 5 {
		case 0: // the default process
			w0, w1, o0, o1 = 1, 1, 0, 0
			log2Wd = 6
		case 1: // the limits
			w0, w1 = [2]int32{-maxWeight, maxWeight}[rng.Intn(2)], [2]int32{-maxWeight, maxWeight}[rng.Intn(2)]
			o0, o1 = [2]int32{-maxWOff, maxWOff}[rng.Intn(2)], [2]int32{-maxWOff, maxWOff}[rng.Intn(2)]
		}
		dst := fillBytes((h-1)*dS+w+rng.Intn(5), sentinel8)
		var err error
		if bi {
			err = TryWeightBiBytes(dst, 0, dS, p0, p1, 0, pS, w, h, log2Wd, w0, w1, o0, o1)
		} else {
			err = TryWeightUniBytes(dst, 0, dS, p0, 0, pS, w, h, log2Wd, w0, o0)
		}
		if err != nil {
			t.Fatalf("iter %d: %v", iter, err)
		}
		for i := range dst {
			y, x := i/dS, i%dS
			want := uint8(sentinel8)
			if x < w && y < h {
				if bi {
					want = modelWeightBi(p0[y*pS+x], p1[y*pS+x], log2Wd, w0, w1, o0, o1)
				} else {
					want = modelWeightUni(p0[y*pS+x], log2Wd, w0, o0)
				}
			}
			if dst[i] != want {
				t.Fatalf("iter %d bi %v %dx%d log2Wd %d: element %d is %d, the text gives %d", iter, bi, w, h, log2Wd, i, dst[i], want)
			}
		}
	}
}

func TestSAOBytes(t *testing.T) {
	rng := rand.New(rand.NewSource(83))
	for iter := 0; iter < 8000; iter++ {
		edge := iter%2 == 1
		w, h := 1+rng.Intn(64), 1+rng.Intn(64)
		off := randomByteSAOOffsets(rng, iter/2)
		sS, dS := w+2+rng.Intn(9), w+rng.Intn(9)
		src := randomBytes(rng, (h+2)*sS+rng.Intn(5), iter%7 == 0)
		if iter%13 == 0 { // flat: no edge
			src = fillBytes(len(src), uint8(rng.Intn(256)))
		}
		dst := fillBytes((h-1)*dS+w+rng.Intn(5), sentinel8)
		srcOff := sS + 1 // one row and one column of neighbours before the block
		class, bandPos := rng.Intn(4), rng.Intn(32)
		var err error
		if edge {
			err = TrySAOEdgeBytes(dst, 0, dS, src, srcOff, sS, w, h, class, off)
		} else {
			err = TrySAOBandBytes(dst, 0, dS, src, srcOff, sS, w, h, bandPos, off)
		}
		if err != nil {
			t.Fatalf("iter %d: %v", iter, err)
		}
		offA, offB := saoEdgeOffsets(class, sS)
		for i := range dst {
			y, x := i/dS, i%dS
			want := uint8(sentinel8)
			if x < w && y < h {
				pos := srcOff + y*sS + x
				if edge {
					want = modelSAOEdge(src[pos], src[pos+offA], src[pos+offB], off)
				} else {
					want = modelSAOBand(src[pos], bandPos, off)
				}
			}
			if dst[i] != want {
				t.Fatalf("iter %d edge %v %dx%d: element %d is %d, the text gives %d", iter, edge, w, h, i, dst[i], want)
			}
		}
	}
}

func TestBytesDstRefusals(t *testing.T) {
	d := fillBytes(4096, sentinel8)
	a := make([]uint8, 8192)
	p := make([]int32, 4096)
	off := [4]int16{1, -2, 3, -4}
	bigOff := [4]int16{saoMaxOffset + 1, 0, 0, 0}
	mem := make([]int32, 4096)
	mem8 := unsafe.Slice((*uint8)(unsafe.Pointer(&mem[0])), 4*len(mem))
	for name, f := range map[string]func() error{
		"average odd width":             func() error { return TryAverageBytes(d, 0, 64, a, 0, 64, a, 0, 64, 5, 4) },
		"average width 0":               func() error { return TryAverageBytes(d, 0, 64, a, 0, 64, a, 0, 64, 0, 4) },
		"average width 66":              func() error { return TryAverageBytes(d, 0, 128, a, 0, 128, a, 0, 128, 66, 4) },
		"average height 65":             func() error { return TryAverageBytes(d, 0, 64, a, 0, 64, a, 0, 64, 8, 65) },
		"average stride":                func() error { return TryAverageBytes(d, 0, 4, a, 0, 64, a, 0, 64, 8, 4) },
		"average source stride":         func() error { return TryAverageBytes(d, 0, 64, a, 0, 64, a, 0, 4, 8, 4) },
		"average destination fit":       func() error { return TryAverageBytes(d, 4090, 64, a, 0, 64, a, 0, 64, 8, 4) },
		"average source fit":            func() error { return TryAverageBytes(d, 0, 64, a, 8190, 64, a, 0, 64, 8, 4) },
		"average overlap":               func() error { return TryAverageBytes(mem8, 0, 64, a, 0, 64, mem8, 0, 64, 8, 4) },
		"weight odd width":              func() error { return TryWeightUniBytes(d, 0, 64, p, 0, 64, 5, 4, 6, 1, 0) },
		"weight log2Wd 1":               func() error { return TryWeightUniBytes(d, 0, 64, p, 0, 64, 8, 4, 1, 1, 0) },
		"weight log2Wd 14":              func() error { return TryWeightBiBytes(d, 0, 64, p, p, 0, 64, 8, 4, 14, 1, 1, 0, 0) },
		"weight too heavy":              func() error { return TryWeightUniBytes(d, 0, 64, p, 0, 64, 8, 4, 6, maxWeight+1, 0) },
		"weight offset":                 func() error { return TryWeightBiBytes(d, 0, 64, p, p, 0, 64, 8, 4, 6, 1, 1, maxWOff+1, 0) },
		"weight stride":                 func() error { return TryWeightUniBytes(d, 0, 4, p, 0, 64, 8, 4, 6, 1, 0) },
		"weight intermediate stride":    func() error { return TryWeightUniBytes(d, 0, 64, p, 0, 4, 8, 4, 6, 1, 0) },
		"weight destination fit":        func() error { return TryWeightUniBytes(d, 4090, 64, p, 0, 64, 8, 4, 6, 1, 0) },
		"weight intermediate fit":       func() error { return TryWeightBiBytes(d, 0, 64, p, p, 4090, 64, 8, 4, 6, 1, 1, 0, 0) },
		"weight overlap":                func() error { return TryWeightUniBytes(mem8, 0, 64, mem, 0, 64, 8, 4, 6, 1, 0) },
		"band position":                 func() error { return TrySAOBandBytes(d, 0, 64, a, 0, 64, 8, 4, 32, &off) },
		"band no offsets":               func() error { return TrySAOBandBytes(d, 0, 64, a, 0, 64, 8, 4, 0, nil) },
		"band offset too big":           func() error { return TrySAOBandBytes(d, 0, 64, a, 0, 64, 8, 4, 0, &bigOff) },
		"band width 65":                 func() error { return TrySAOBandBytes(d, 0, 128, a, 0, 128, 65, 4, 0, &off) },
		"band height 0":                 func() error { return TrySAOBandBytes(d, 0, 64, a, 0, 64, 8, 0, 0, &off) },
		"band destination stride":       func() error { return TrySAOBandBytes(d, 0, 4, a, 0, 64, 8, 4, 0, &off) },
		"band source stride":            func() error { return TrySAOBandBytes(d, 0, 64, a, 0, 0, 8, 4, 0, &off) },
		"band destination fit":          func() error { return TrySAOBandBytes(d, 4090, 64, a, 0, 64, 8, 4, 0, &off) },
		"band source fit":               func() error { return TrySAOBandBytes(d, 0, 64, a, 8190, 64, 8, 4, 0, &off) },
		"band overlap":                  func() error { return TrySAOBandBytes(mem8, 0, 64, mem8, 0, 64, 8, 4, 0, &off) },
		"edge class":                    func() error { return TrySAOEdgeBytes(d, 0, 64, a, 65, 64, 8, 4, 4, &off) },
		"edge no offsets":               func() error { return TrySAOEdgeBytes(d, 0, 64, a, 65, 64, 8, 4, 0, nil) },
		"edge source fit before":        func() error { return TrySAOEdgeBytes(d, 0, 64, a, 0, 64, 8, 4, 1, &off) },
		"edge source fit after":         func() error { return TrySAOEdgeBytes(d, 0, 64, a, 8190, 64, 8, 4, 1, &off) },
		"edge destination fit":          func() error { return TrySAOEdgeBytes(d, 4090, 64, a, 65, 64, 8, 4, 0, &off) },
		"edge overlap":                  func() error { return TrySAOEdgeBytes(mem8, 0, 64, mem8, 65, 64, 8, 4, 0, &off) },
		"edge destination stride":       func() error { return TrySAOEdgeBytes(d, 0, 4, a, 65, 64, 8, 4, 0, &off) },
		"edge block past 64":            func() error { return TrySAOEdgeBytes(d, 0, 128, a, 129, 128, 65, 4, 0, &off) },
		"edge source stride that wraps": func() error { return TrySAOEdgeBytes(d, 0, 64, a, 65, maxInt, 8, 4, 1, &off) },
		"average stride that wraps":     func() error { return TryAverageBytes(d, 0, 64, a, 0, maxInt, a, 0, 64, 8, 4) },
		"weight stride that wraps":      func() error { return TryWeightUniBytes(d, 0, 64, p, 0, maxInt, 8, 4, 6, 1, 0) },
	} {
		for i := range d {
			d[i] = sentinel8
		}
		if err := f(); !errors.Is(err, ErrRefused) {
			t.Errorf("%s: %v", name, err)
		}
		for i := range d {
			if d[i] != sentinel8 {
				t.Fatalf("%s: a refused call wrote dst[%d]", name, i)
			}
		}
	}
}

// The portable references are the text's formulas over bytes too: they are what a target
// without a kernel runs, and what finishes the columns past the last group of eight.
func TestBytesDstRefsAgainstTheText(t *testing.T) {
	rng := rand.New(rand.NewSource(84))
	for iter := 0; iter < 300; iter++ {
		w, h := 2*(1+rng.Intn(5)), 1+rng.Intn(6)
		a, b := randomBytes(rng, w*h, false), randomBytes(rng, w*h, iter%3 == 0)
		d := make([]uint8, w*h)
		averageRef(d, 0, w, a, 0, w, b, 0, w, w, h)
		for i := range d {
			if want := uint8((int(a[i]) + int(b[i]) + 1) >> 1); d[i] != want {
				t.Fatalf("average: element %d is %d, want %d", i, d[i], want)
			}
		}
		p0, p1 := make([]int32, w*h), make([]int32, w*h)
		for i := range p0 {
			p0[i], p1[i] = int32(rng.Intn(1<<17)-1<<16), int32(rng.Intn(1<<17)-1<<16)
		}
		log2Wd := minLog2Wd + rng.Intn(maxLog2Wd-minLog2Wd+1)
		wt, o := int32(rng.Intn(257)-128), int32(rng.Intn(257)-128)
		weightUniRef(d, 0, w, p0, 0, w, w, h, uint(log2Wd), &[4]int32{wt, 1 << uint(log2Wd-1), o, 255})
		for i := range d {
			if want := modelWeightUni(p0[i], log2Wd, wt, o); d[i] != want {
				t.Fatalf("uni: element %d is %d, want %d", i, d[i], want)
			}
		}
		weightBiRef(d, 0, w, p0, p1, 0, w, w, h, uint(log2Wd), &[4]int32{wt, wt + 1, (o + o + 1) << uint(log2Wd), 255})
		for i := range d {
			if want := modelWeightBi(p0[i], p1[i], log2Wd, wt, wt+1, o, o); d[i] != want {
				t.Fatalf("bi: element %d is %d, want %d", i, d[i], want)
			}
		}
	}
}
