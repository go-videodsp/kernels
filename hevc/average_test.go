package hevc

import (
	"errors"
	"math/rand"
	"testing"
)

// The text's (a + b + 1) >> 1 in 64 bits.
func modelAverage(a, b uint16) uint16 { return uint16((int64(a) + int64(b) + 1) >> 1) }

func randomSamples(rng *rand.Rand, n, kind int) []uint16 {
	s := make([]uint16, n)
	for i := range s {
		switch kind {
		case 0: // 8-bit video
			s[i] = uint16(rng.Intn(256))
		case 1: // the extremes, where the rounding and the carry out of 16 bits are
			s[i] = [4]uint16{0, 1, 65534, 65535}[rng.Intn(4)]
		default:
			s[i] = uint16(rng.Intn(65536))
		}
	}
	return s
}

func TestAverageAgainstTheText(t *testing.T) {
	rng := rand.New(rand.NewSource(41))
	for iter := 0; iter < 6000; iter++ {
		w, h := 2*(1+rng.Intn(32)), 1+rng.Intn(64)
		aStride, bStride, dStride := w+rng.Intn(9), w+rng.Intn(9), w+rng.Intn(9)
		a := randomSamples(rng, (h-1)*aStride+w+rng.Intn(5), iter%3)
		b := randomSamples(rng, (h-1)*bStride+w+rng.Intn(5), (iter+1)%3)
		if iter%7 == 0 { // the same block twice
			b, bStride = a, aStride
		}
		dst := make([]uint16, (h-1)*dStride+w+rng.Intn(5))
		for i := range dst {
			dst[i] = sentinel16
		}
		if err := TryAverage(dst, 0, dStride, a, 0, aStride, b, 0, bStride, w, h); err != nil {
			t.Fatalf("iter %d: %v", iter, err)
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if want := modelAverage(a[y*aStride+x], b[y*bStride+x]); dst[y*dStride+x] != want {
					t.Fatalf("iter %d: %dx%d: sample (%d,%d) is %d, the text gives %d", iter, w, h, x, y, dst[y*dStride+x], want)
				}
			}
		}
		for i := range dst {
			if y, x := i/dStride, i%dStride; (x >= w || y >= h) && dst[i] != sentinel16 {
				t.Fatalf("iter %d: a sample outside the block was written", iter)
			}
		}
	}
}

func TestAverageRefusals(t *testing.T) {
	dst := make([]uint16, 4096)
	a := make([]uint16, 4096)
	b := make([]uint16, 4096)
	for name, f := range map[string]func() error{
		"odd width":          func() error { return TryAverage(dst, 0, 64, a, 0, 64, b, 0, 64, 5, 4) },
		"width 0":            func() error { return TryAverage(dst, 0, 64, a, 0, 64, b, 0, 64, 0, 4) },
		"width past 64":      func() error { return TryAverage(dst, 0, 128, a, 0, 128, b, 0, 128, 66, 4) },
		"height 0":           func() error { return TryAverage(dst, 0, 64, a, 0, 64, b, 0, 64, 8, 0) },
		"height past 64":     func() error { return TryAverage(dst, 0, 64, a, 0, 64, b, 0, 64, 8, 65) },
		"destination stride": func() error { return TryAverage(dst, 0, 4, a, 0, 64, b, 0, 64, 8, 4) },
		"a stride":           func() error { return TryAverage(dst, 0, 64, a, 0, 4, b, 0, 64, 8, 4) },
		"b stride":           func() error { return TryAverage(dst, 0, 64, a, 0, 64, b, 0, 4, 8, 4) },
		"destination fit":    func() error { return TryAverage(dst, 4090, 64, a, 0, 64, b, 0, 64, 8, 4) },
		"a fit":              func() error { return TryAverage(dst, 0, 64, a, 4090, 64, b, 0, 64, 8, 4) },
		"b fit":              func() error { return TryAverage(dst, 0, 64, a, 0, 64, b, 4090, 64, 8, 4) },
		"stride that wraps":  func() error { return TryAverage(dst, 0, 64, a, 0, maxInt, b, 0, 64, 8, 4) },
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
	// the destination sharing memory with either source
	mem := make([]uint16, 2048)
	if err := TryAverage(mem, 0, 64, mem, 0, 64, b, 0, 64, 8, 4); !errors.Is(err, ErrRefused) {
		t.Errorf("overlap with a: %v", err)
	}
	if err := TryAverage(mem, 0, 64, a, 0, 64, mem, 0, 64, 8, 4); !errors.Is(err, ErrRefused) {
		t.Errorf("overlap with b: %v", err)
	}
	mustPanic(t, "Average", func() { Average(dst, 0, 64, a, 0, 64, b, 0, 64, 5, 4) })
	Average(dst, 0, 64, a, 0, 64, b, 0, 64, 8, 4)
}

// The portable reference is the text's formula too: it is what a target without a kernel runs.
func TestAverageReferenceAgainstTheText(t *testing.T) {
	rng := rand.New(rand.NewSource(43))
	for iter := 0; iter < 200; iter++ {
		w, h := 2*(1+rng.Intn(8)), 1+rng.Intn(8)
		a, b := randomSamples(rng, w*h, iter%3), randomSamples(rng, w*h, (iter+1)%3)
		got := make([]uint16, w*h)
		averageRef(got, 0, w, a, 0, w, b, 0, w, w, h)
		for i := range got {
			if want := modelAverage(a[i], b[i]); got[i] != want {
				t.Fatalf("element %d is %d, the text gives %d", i, got[i], want)
			}
		}
	}
}
