package hevc

import (
	"errors"
	"math/rand"
	"testing"
)

// The text's formulas in 64 bits (Equations 8-275 and 8-277 with the Clip3 of the sample range).
func modelUni(p, w, o int64, log2Wd uint, bd int) uint16 {
	v := ((p*w + 1<<(log2Wd-1)) >> log2Wd) + o
	return uint16(min(max(v, 0), 1<<uint(bd)-1))
}

func modelBi(p0, p1, w0, w1, o0, o1 int64, log2Wd uint, bd int) uint16 {
	v := (p0*w0 + p1*w1 + (o0+o1+1)<<log2Wd) >> (log2Wd + 1)
	return uint16(min(max(v, 0), 1<<uint(bd)-1))
}

func randomIntermediates(rng *rand.Rand, n int, kind int) []int32 {
	p := make([]int32, n)
	for i := range p {
		switch kind {
		case 0: // the range of interpolation of 8-bit video
			p[i] = int32(rng.Intn(33200+6144) - 6144)
		case 1:
			p[i] = [2]int32{-65536, 65536}[rng.Intn(2)]
		default:
			p[i] = int32(rng.Intn(131073) - 65536)
		}
	}
	return p
}

func TestWeightAgainstTheText(t *testing.T) {
	rng := rand.New(rand.NewSource(31))
	var uni, bi, deflt int
	for iter := 0; iter < 8000; iter++ {
		w, h := 2*(1+rng.Intn(32)), 1+rng.Intn(64)
		bd := 8 + rng.Intn(5)
		log2Wd := minLog2Wd + rng.Intn(maxLog2Wd-minLog2Wd+1)
		wt0, wt1 := int32(rng.Intn(383)-128), int32(rng.Intn(383)-128)
		o0, o1 := int32(rng.Intn(2049)-1024), int32(rng.Intn(2049)-1024)
		if iter%5 == 0 { // the default process
			log2Wd, wt0, wt1, o0, o1 = max(2, 14-bd), 1, 1, 0, 0
			deflt++
		}
		pStride, dStride := w+rng.Intn(9), w+rng.Intn(9)
		n := (h-1)*pStride + w + rng.Intn(5)
		p0, p1 := randomIntermediates(rng, n, iter%3), randomIntermediates(rng, n, (iter+1)%3)
		dst := make([]uint16, (h-1)*dStride+w+rng.Intn(5))
		for i := range dst {
			dst[i] = sentinel16
		}
		if iter%2 == 0 {
			uni++
			if err := TryWeightUni(dst, 0, dStride, p0, 0, pStride, w, h, bd, log2Wd, wt0, o0); err != nil {
				t.Fatalf("iter %d: %v", iter, err)
			}
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					if want := modelUni(int64(p0[y*pStride+x]), int64(wt0), int64(o0), uint(log2Wd), bd); dst[y*dStride+x] != want {
						t.Fatalf("iter %d: uni %dx%d bit depth %d log2Wd %d: sample (%d,%d) is %d, the text gives %d", iter, w, h, bd, log2Wd, x, y, dst[y*dStride+x], want)
					}
				}
			}
		} else {
			bi++
			if err := TryWeightBi(dst, 0, dStride, p0, p1, 0, pStride, w, h, bd, log2Wd, wt0, wt1, o0, o1); err != nil {
				t.Fatalf("iter %d: %v", iter, err)
			}
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					i := y*pStride + x
					if want := modelBi(int64(p0[i]), int64(p1[i]), int64(wt0), int64(wt1), int64(o0), int64(o1), uint(log2Wd), bd); dst[y*dStride+x] != want {
						t.Fatalf("iter %d: bi %dx%d bit depth %d log2Wd %d: sample (%d,%d) is %d, the text gives %d", iter, w, h, bd, log2Wd, x, y, dst[y*dStride+x], want)
					}
				}
			}
		}
		for i := range dst {
			if y, x := i/dStride, i%dStride; (x >= w || y >= h) && dst[i] != sentinel16 {
				t.Fatalf("iter %d: a sample outside the block was written", iter)
			}
		}
	}
	if uni == 0 || bi == 0 || deflt == 0 {
		t.Error("a case was never exercised")
	}
}

func TestWeightRefusals(t *testing.T) {
	dst := make([]uint16, 4096)
	p := make([]int32, 4096)
	q := make([]int32, 4096)
	for name, f := range map[string]func() error{
		"odd width":              func() error { return TryWeightUni(dst, 0, 64, p, 0, 64, 5, 4, 8, 6, 1, 0) },
		"width 0":                func() error { return TryWeightUni(dst, 0, 64, p, 0, 64, 0, 4, 8, 6, 1, 0) },
		"width past 64":          func() error { return TryWeightUni(dst, 0, 128, p, 0, 128, 66, 4, 8, 6, 1, 0) },
		"height 0":               func() error { return TryWeightUni(dst, 0, 64, p, 0, 64, 8, 0, 8, 6, 1, 0) },
		"bit depth 13":           func() error { return TryWeightUni(dst, 0, 64, p, 0, 64, 8, 4, 13, 6, 1, 0) },
		"log2Wd 1":               func() error { return TryWeightUni(dst, 0, 64, p, 0, 64, 8, 4, 8, 1, 1, 0) },
		"log2Wd 14":              func() error { return TryWeightUni(dst, 0, 64, p, 0, 64, 8, 4, 8, 14, 1, 0) },
		"weight too large":       func() error { return TryWeightUni(dst, 0, 64, p, 0, 64, 8, 4, 8, 6, 513, 0) },
		"offset too large":       func() error { return TryWeightUni(dst, 0, 64, p, 0, 64, 8, 4, 8, 6, 1, 1<<17) },
		"destination stride":     func() error { return TryWeightUni(dst, 0, 4, p, 0, 64, 8, 4, 8, 6, 1, 0) },
		"source stride":          func() error { return TryWeightUni(dst, 0, 64, p, 0, 4, 8, 4, 8, 6, 1, 0) },
		"destination fit":        func() error { return TryWeightUni(dst, 4090, 64, p, 0, 64, 8, 4, 8, 6, 1, 0) },
		"source fit":             func() error { return TryWeightUni(dst, 0, 64, p, 4090, 64, 8, 4, 8, 6, 1, 0) },
		"stride that wraps":      func() error { return TryWeightUni(dst, 0, 64, p, 0, maxInt, 8, 4, 8, 6, 1, 0) },
		"bi: second source fit":  func() error { return TryWeightBi(dst, 0, 64, p, q[:100], 0, 64, 8, 4, 8, 6, 1, 1, 0, 0) },
		"bi: weight too large":   func() error { return TryWeightBi(dst, 0, 64, p, q, 0, 64, 8, 4, 8, 6, 1, 600, 0, 0) },
		"bi: offset too large":   func() error { return TryWeightBi(dst, 0, 64, p, q, 0, 64, 8, 4, 8, 6, 1, 1, 0, -(1 << 17)) },
		"bi: bit depth 7":        func() error { return TryWeightBi(dst, 0, 64, p, q, 0, 64, 8, 4, 7, 6, 1, 1, 0, 0) },
		"bi: destination stride": func() error { return TryWeightBi(dst, 0, 4, p, q, 0, 64, 8, 4, 8, 6, 1, 1, 0, 0) },
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
	// the destination and an intermediate array sharing memory
	mem := make([]int32, 2048)
	if err := TryWeightUni(unsafeUint16(mem), 0, 64, mem, 0, 64, 8, 4, 8, 6, 1, 0); !errors.Is(err, ErrRefused) {
		t.Errorf("overlap: %v", err)
	}
	if err := TryWeightBi(unsafeUint16(mem), 0, 64, p, mem, 0, 64, 8, 4, 8, 6, 1, 1, 0, 0); !errors.Is(err, ErrRefused) {
		t.Errorf("bi overlap: %v", err)
	}
	mustPanic(t, "WeightUni", func() { WeightUni(dst, 0, 64, p, 0, 64, 5, 4, 8, 6, 1, 0) })
	mustPanic(t, "WeightBi", func() { WeightBi(dst, 0, 64, p, q, 0, 64, 5, 4, 8, 6, 1, 1, 0, 0) })
	WeightUni(dst, 0, 64, p, 0, 64, 8, 4, 8, 6, 1, 0)
	WeightBi(dst, 0, 64, p, q, 0, 64, 8, 4, 8, 6, 1, 1, 0, 0)
}

// The portable reference is the text's formulas too: it is what a target without a kernel runs.
func TestWeightReferenceAgainstTheText(t *testing.T) {
	rng := rand.New(rand.NewSource(33))
	for iter := 0; iter < 400; iter++ {
		w, h := 2*(1+rng.Intn(8)), 1+rng.Intn(8)
		bd := 8 + rng.Intn(5)
		log2Wd := uint(minLog2Wd + rng.Intn(maxLog2Wd-minLog2Wd+1))
		w0, w1, o0, o1 := int32(rng.Intn(383)-128), int32(rng.Intn(383)-128), int32(rng.Intn(2049)-1024), int32(rng.Intn(2049)-1024)
		p0, p1 := randomIntermediates(rng, w*h, iter%3), randomIntermediates(rng, w*h, (iter+1)%3)
		got := make([]uint16, w*h)
		hi := int32(1)<<uint(bd) - 1
		weightUniRef(got, 0, w, p0, 0, w, w, h, log2Wd, &[4]int32{w0, 1 << (log2Wd - 1), o0, hi})
		for i := range got {
			if want := modelUni(int64(p0[i]), int64(w0), int64(o0), log2Wd, bd); got[i] != want {
				t.Fatalf("uni: element %d is %d, the text gives %d", i, got[i], want)
			}
		}
		weightBiRef(got, 0, w, p0, p1, 0, w, w, h, log2Wd, &[4]int32{w0, w1, (o0 + o1 + 1) << log2Wd, hi})
		for i := range got {
			if want := modelBi(int64(p0[i]), int64(p1[i]), int64(w0), int64(w1), int64(o0), int64(o1), log2Wd, bd); got[i] != want {
				t.Fatalf("bi: element %d is %d, the text gives %d", i, got[i], want)
			}
		}
	}
}
