package hevc

import (
	"errors"
	"math/rand"
	"testing"
)

// modelHW is the text's one-dimensional horizontal filter of one sample in 64 bits: the taps c
// times the samples of the columns from -bef around the position, shifted by bitDepth - 8.
func modelHW(src []uint16, pos int, c []int16, bef int, bd int) int32 {
	var s int64
	for k, ck := range c {
		s += int64(ck) * int64(src[pos+k-bef])
	}
	return int32(s >> uint(bd-8))
}

func TestHPassWAgainstTheText(t *testing.T) {
	rng := rand.New(rand.NewSource(71))
	for iter := 0; iter < 8000; iter++ {
		chroma := iter%2 == 1
		n, bef, aft, unit := 8, before, after, 4
		if chroma {
			n, bef, aft, unit = 4, chromaBefore, chromaAfter, 2
		}
		w, h := unit*(1+rng.Intn(64/unit)), 1+rng.Intn(64)
		bd := 8 + rng.Intn(5)
		c := randomFilter(rng, n, iter/2)
		srcStride, dstStride := w+bef+aft+rng.Intn(9), w+rng.Intn(9)
		src := randomU16(rng, h*srcStride+rng.Intn(5), bd, iter%7 == 0)
		if iter%13 == 0 {
			for i := range src {
				src[i] = uint16(rng.Intn(32768))
			}
		}
		dst := make([]int32, h*dstStride+rng.Intn(5))
		for i := range dst {
			dst[i] = int32(sentinel16)
		}
		srcOff := bef + rng.Intn(srcStride-w-bef-aft+1)
		var err error
		if chroma {
			err = TryHPassW4(dst, 0, dstStride, src, srcOff, srcStride, w, h, bd, (*[4]int16)(c))
		} else {
			err = TryHPassW8(dst, 0, dstStride, src, srcOff, srcStride, w, h, bd, (*[8]int16)(c))
		}
		if err != nil {
			t.Fatalf("iter %d: %v", iter, err)
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				want := modelHW(src, srcOff+y*srcStride+x, c, bef, bd)
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

func TestWidenAgainstTheText(t *testing.T) {
	rng := rand.New(rand.NewSource(72))
	for iter := 0; iter < 6000; iter++ {
		w, h := 2*(1+rng.Intn(32)), 1+rng.Intn(64)
		shift := rng.Intn(16)
		srcStride, dstStride := w+rng.Intn(9), w+rng.Intn(9)
		src := randomU16(rng, (h-1)*srcStride+w+rng.Intn(5), 12, iter%5 == 0)
		if iter%3 == 0 { // every 16-bit value, the largest included
			for i := range src {
				src[i] = uint16(rng.Intn(65536))
			}
		}
		dst := make([]int32, (h-1)*dstStride+w+rng.Intn(5))
		for i := range dst {
			dst[i] = int32(sentinel16)
		}
		if err := TryWiden(dst, 0, dstStride, src, 0, srcStride, w, h, shift); err != nil {
			t.Fatalf("iter %d: %v", iter, err)
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if want := int32(src[y*srcStride+x]) << uint(shift); dst[y*dstStride+x] != want {
					t.Fatalf("iter %d: %dx%d shift %d: sample (%d,%d) is %d, want %d", iter, w, h, shift, x, y, dst[y*dstStride+x], want)
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

func TestHPassWAndWidenRefusals(t *testing.T) {
	dst := make([]int32, 4096)
	src := make([]uint16, 8192)
	c8 := lumaFilters[1]
	c4 := chromaFilters[3]
	for name, f := range map[string]func() error{
		"luma width not a multiple of 4": func() error { return TryHPassW8(dst, 0, 64, src, 3, 80, 6, 4, 8, &c8) },
		"luma width past 64":             func() error { return TryHPassW8(dst, 0, 128, src, 3, 128, 68, 4, 8, &c8) },
		"luma height 0":                  func() error { return TryHPassW8(dst, 0, 64, src, 3, 80, 8, 0, 8, &c8) },
		"bit depth 7":                    func() error { return TryHPassW8(dst, 0, 64, src, 3, 80, 8, 4, 7, &c8) },
		"bit depth 13":                   func() error { return TryHPassW8(dst, 0, 64, src, 3, 80, 8, 4, 13, &c8) },
		"no coefficients":                func() error { return TryHPassW8(dst, 0, 64, src, 3, 80, 8, 4, 8, nil) },
		"destination stride":             func() error { return TryHPassW8(dst, 0, 4, src, 3, 80, 8, 4, 8, &c8) },
		"destination fit":                func() error { return TryHPassW8(dst, 4090, 64, src, 3, 80, 8, 4, 8, &c8) },
		"source fit before":              func() error { return TryHPassW8(dst, 0, 64, src, 2, 80, 8, 4, 8, &c8) },
		"source fit after":               func() error { return TryHPassW8(dst, 0, 64, src, 8180, 80, 8, 4, 8, &c8) },
		"stride that wraps":              func() error { return TryHPassW8(dst, 0, 64, src, 3, maxInt, 8, 4, 8, &c8) },
		"chroma odd width":               func() error { return TryHPassW4(dst, 0, 64, src, 1, 80, 5, 4, 8, &c4) },
		"chroma source fit before":       func() error { return TryHPassW4(dst, 0, 64, src, 0, 80, 8, 4, 8, &c4) },
		"widen odd width":                func() error { return TryWiden(dst, 0, 64, src, 0, 64, 5, 4, 2) },
		"widen width 0":                  func() error { return TryWiden(dst, 0, 64, src, 0, 64, 0, 4, 2) },
		"widen width past 64":            func() error { return TryWiden(dst, 0, 128, src, 0, 128, 66, 4, 2) },
		"widen height 65":                func() error { return TryWiden(dst, 0, 64, src, 0, 64, 8, 65, 2) },
		"widen shift -1":                 func() error { return TryWiden(dst, 0, 64, src, 0, 64, 8, 4, -1) },
		"widen shift 16":                 func() error { return TryWiden(dst, 0, 64, src, 0, 64, 8, 4, 16) },
		"widen destination stride":       func() error { return TryWiden(dst, 0, 4, src, 0, 64, 8, 4, 2) },
		"widen source stride":            func() error { return TryWiden(dst, 0, 64, src, 0, 4, 8, 4, 2) },
		"widen destination fit":          func() error { return TryWiden(dst, 4090, 64, src, 0, 64, 8, 4, 2) },
		"widen source fit":               func() error { return TryWiden(dst, 0, 64, src, 8190, 64, 8, 4, 2) },
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
	mem := make([]int32, 4096)
	if err := TryHPassW8(mem, 0, 64, unsafeUint16(mem), 3, 80, 8, 4, 8, &c8); !errors.Is(err, ErrRefused) {
		t.Errorf("overlap: %v", err)
	}
	if err := TryWiden(mem, 0, 64, unsafeUint16(mem), 0, 64, 8, 4, 2); !errors.Is(err, ErrRefused) {
		t.Errorf("widen overlap: %v", err)
	}
	mustPanic(t, "HPassW8", func() { HPassW8(dst, 0, 64, src, 3, 80, 6, 4, 8, &c8) })
	mustPanic(t, "HPassW4", func() { HPassW4(dst, 0, 64, src, 1, 80, 5, 4, 8, &c4) })
	mustPanic(t, "Widen", func() { Widen(dst, 0, 64, src, 0, 64, 5, 4, 2) })
	HPassW8(dst, 0, 64, src, 3, 80, 8, 4, 8, &c8)
	HPassW4(dst, 0, 64, src, 1, 80, 8, 4, 8, &c4)
	Widen(dst, 0, 64, src, 0, 64, 8, 4, 2)
}

// The portable references are the text's formulas too: they are what a target without a kernel runs.
func TestHPassWAndWidenRefsAgainstTheText(t *testing.T) {
	rng := rand.New(rand.NewSource(73))
	for iter := 0; iter < 300; iter++ {
		w, h, bd := 4*(1+rng.Intn(4)), 1+rng.Intn(6), 8+rng.Intn(5)
		c8, c4 := randomFilter(rng, 8, iter), randomFilter(rng, 4, iter)
		src := randomU16(rng, h*(w+7), bd, false)
		got := make([]int32, w*h)
		hpassW8Ref(got, 0, w, src, 3, w+7, w, h, uint(bd-8), (*[8]int16)(c8))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if want := modelHW(src, 3+y*(w+7)+x, c8, 3, bd); got[y*w+x] != want {
					t.Fatalf("8 taps: (%d,%d) is %d, the text gives %d", x, y, got[y*w+x], want)
				}
			}
		}
		hpassW4Ref(got, 0, w, src, 1, w+7, w, h, uint(bd-8), (*[4]int16)(c4))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if want := modelHW(src, 1+y*(w+7)+x, c4, 1, bd); got[y*w+x] != want {
					t.Fatalf("4 taps: (%d,%d) is %d, the text gives %d", x, y, got[y*w+x], want)
				}
			}
		}
		widenRef(got, 0, w, src, 0, w+7, w, h, uint(iter%16))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if want := int32(src[y*(w+7)+x]) << uint(iter%16); got[y*w+x] != want {
					t.Fatalf("widen: (%d,%d) is %d, want %d", x, y, got[y*w+x], want)
				}
			}
		}
	}
}
