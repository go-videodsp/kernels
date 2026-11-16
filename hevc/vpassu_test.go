package hevc

import (
	"errors"
	"math/rand"
	"testing"
)

// modelVU is the text's one-dimensional vertical filter of one sample in 64 bits: the taps c
// times the samples of the rows from -bef around the position, shifted by bitDepth - 8.
func modelVU(src []uint16, pos, stride int, c []int16, bef int, bd int) int32 {
	var s int64
	for k, ck := range c {
		s += int64(ck) * int64(src[pos+(k-bef)*stride])
	}
	return int32(s >> uint(bd-8))
}

func randomFilter(rng *rand.Rand, n int, iter int) []int16 {
	c := make([]int16, n)
	switch iter % 3 {
	case 0:
		if n == 8 {
			copy(c, lumaFilters[rng.Intn(3)][:])
		} else {
			copy(c, chromaFilters[rng.Intn(7)][:])
		}
	case 1: // the extremes of the sums
		for i := range c {
			c[i] = [2]int16{-90, 90}[rng.Intn(2)]
		}
	default:
		for i := range c {
			c[i] = int16(rng.Intn(181) - 90)
		}
	}
	return c
}

func TestVPassUAgainstTheText(t *testing.T) {
	rng := rand.New(rand.NewSource(61))
	for iter := 0; iter < 8000; iter++ {
		chroma := iter%2 == 1
		n, bef, aft, unit := 8, before, after, 4
		if chroma {
			n, bef, aft, unit = 4, chromaBefore, chromaAfter, 2
		}
		w, h := unit*(1+rng.Intn(64/unit)), 1+rng.Intn(64)
		bd := 8 + rng.Intn(5)
		c := randomFilter(rng, n, iter/2)
		srcStride, dstStride := w+rng.Intn(9), w+rng.Intn(9)
		rows := h + bef + aft
		src := randomU16(rng, rows*srcStride+rng.Intn(5), bd, iter%7 == 0)
		if iter%13 == 0 { // the samples up to the largest the kernels take
			for i := range src {
				src[i] = uint16(rng.Intn(32768))
			}
		}
		dst := make([]int32, h*dstStride+rng.Intn(5))
		for i := range dst {
			dst[i] = int32(sentinel16)
		}
		srcOff := bef*srcStride + rng.Intn(srcStride-w+1)
		var err error
		if chroma {
			err = TryVPassU4(dst, 0, dstStride, src, srcOff, srcStride, w, h, bd, (*[4]int16)(c))
		} else {
			err = TryVPassU8(dst, 0, dstStride, src, srcOff, srcStride, w, h, bd, (*[8]int16)(c))
		}
		if err != nil {
			t.Fatalf("iter %d: %v", iter, err)
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				want := modelVU(src, srcOff+y*srcStride+x, srcStride, c, bef, bd)
				if got := dst[y*dstStride+x]; got != want {
					t.Fatalf("iter %d: chroma %v %dx%d bit depth %d: sample (%d,%d) is %d, the text gives %d", iter, chroma, w, h, bd, x, y, got, want)
				}
			}
		}
		for i := range dst {
			if y, x := i/dstStride, i%dstStride; (x >= w || y >= h) && dst[i] != int32(sentinel16) {
				t.Fatalf("iter %d: a sample outside the block was written", iter)
			}
		}
	}
}

func TestVPassURefusals(t *testing.T) {
	dst := make([]int32, 4096)
	src := make([]uint16, 8192)
	c8 := lumaFilters[1]
	c4 := chromaFilters[3]
	for name, f := range map[string]func() error{
		"luma width not a multiple of 4": func() error { return TryVPassU8(dst, 0, 64, src, 3*64, 64, 6, 4, 8, &c8) },
		"luma width 0":                   func() error { return TryVPassU8(dst, 0, 64, src, 3*64, 64, 0, 4, 8, &c8) },
		"luma width past 64":             func() error { return TryVPassU8(dst, 0, 128, src, 3*128, 128, 68, 4, 8, &c8) },
		"luma height 0":                  func() error { return TryVPassU8(dst, 0, 64, src, 3*64, 64, 8, 0, 8, &c8) },
		"luma height 65":                 func() error { return TryVPassU8(dst, 0, 64, src, 3*64, 64, 8, 65, 8, &c8) },
		"bit depth 7":                    func() error { return TryVPassU8(dst, 0, 64, src, 3*64, 64, 8, 4, 7, &c8) },
		"bit depth 13":                   func() error { return TryVPassU8(dst, 0, 64, src, 3*64, 64, 8, 4, 13, &c8) },
		"no coefficients":                func() error { return TryVPassU8(dst, 0, 64, src, 3*64, 64, 8, 4, 8, nil) },
		"destination stride":             func() error { return TryVPassU8(dst, 0, 4, src, 3*64, 64, 8, 4, 8, &c8) },
		"source stride 0":                func() error { return TryVPassU8(dst, 0, 64, src, 3*64, 0, 8, 4, 8, &c8) },
		"destination fit":                func() error { return TryVPassU8(dst, 4090, 64, src, 3*64, 64, 8, 4, 8, &c8) },
		"source fit above":               func() error { return TryVPassU8(dst, 0, 64, src, 2*64, 64, 8, 4, 8, &c8) },
		"source fit below":               func() error { return TryVPassU8(dst, 0, 64, src, 8100, 64, 8, 4, 8, &c8) },
		"stride that wraps":              func() error { return TryVPassU8(dst, 0, 64, src, 3*80, maxInt, 8, 4, 8, &c8) },
		"chroma odd width":               func() error { return TryVPassU4(dst, 0, 64, src, 64, 64, 5, 4, 8, &c4) },
		"chroma source fit above":        func() error { return TryVPassU4(dst, 0, 64, src, 0, 64, 8, 4, 8, &c4) },
		"chroma bit depth 13":            func() error { return TryVPassU4(dst, 0, 64, src, 64, 64, 8, 4, 13, &c4) },
	} {
		for i := range dst {
			dst[i] = int32(sentinel16)
		}
		if err := f(); !errors.Is(err, ErrRefused) {
			t.Errorf("%s: %v", name, err)
		}
		for i := range dst {
			if dst[i] != int32(sentinel16) {
				t.Fatalf("%s: a refused call wrote dst[%d]", name, i)
			}
		}
	}
	// the destination sharing memory with the source footprint
	mem := make([]int32, 4096)
	if err := TryVPassU8(mem, 0, 64, unsafeUint16(mem), 3*64, 64, 8, 4, 8, &c8); !errors.Is(err, ErrRefused) {
		t.Errorf("overlap: %v", err)
	}
	mustPanic(t, "VPassU8", func() { VPassU8(dst, 0, 64, src, 3*64, 64, 6, 4, 8, &c8) })
	mustPanic(t, "VPassU4", func() { VPassU4(dst, 0, 64, src, 64, 64, 5, 4, 8, &c4) })
	VPassU8(dst, 0, 64, src, 3*64, 64, 8, 4, 8, &c8)
	VPassU4(dst, 0, 64, src, 64, 64, 8, 4, 8, &c4)
}

// The portable reference is the text's formula too: it is what a target without a kernel runs.
func TestVPassURefsAgainstTheText(t *testing.T) {
	rng := rand.New(rand.NewSource(62))
	for iter := 0; iter < 300; iter++ {
		w, h, bd := 4*(1+rng.Intn(4)), 1+rng.Intn(6), 8+rng.Intn(5)
		c8, c4 := randomFilter(rng, 8, iter), randomFilter(rng, 4, iter)
		src := randomU16(rng, (h+7)*w, bd, false)
		got := make([]int32, w*h)
		vpassU8Ref(got, 0, w, src, 3*w, w, w, h, uint(bd-8), (*[8]int16)(c8))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if want := modelVU(src, 3*w+y*w+x, w, c8, 3, bd); got[y*w+x] != want {
					t.Fatalf("8 taps: (%d,%d) is %d, the text gives %d", x, y, got[y*w+x], want)
				}
			}
		}
		vpassU4Ref(got, 0, w, src, w, w, w, h, uint(bd-8), (*[4]int16)(c4))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if want := modelVU(src, w+y*w+x, w, c4, 1, bd); got[y*w+x] != want {
					t.Fatalf("4 taps: (%d,%d) is %d, the text gives %d", x, y, got[y*w+x], want)
				}
			}
		}
	}
}
