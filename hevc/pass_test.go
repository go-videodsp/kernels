package hevc

import (
	"errors"
	"math/rand"
	"testing"
)

// The three luma filters of ITU-T H.265 (V11) (01/2026), Equations 8-224 to 8-226, in the
// order of their taps at offsets -3 to 4.
var lumaFilters = [3][8]int16{
	{-1, 4, -10, 58, 17, -5, 1, 0},
	{-1, 4, -11, 40, 40, -11, 4, -1},
	{0, 1, -5, 17, 58, -10, 4, -1},
}

const sentinel16 = 0x5a5a

// maxInt is the largest int on this target: a stride whose product with the rows wraps.
const maxInt = int(^uint(0) >> 1)

// An impulse (one sample 1, the others 0) shows the filter's own taps: the output at
// offset 3-k from the impulse is c[k] (Equation 8-224 and its siblings), whatever the
// implementation. This ties the reference to the text.
func TestImpulseResponseIsTheFilterTaps(t *testing.T) {
	for f, c := range lumaFilters {
		c := c
		src := make([]uint16, 32*9)
		src[4*32+16] = 1
		dst := make([]int16, 32*9)
		hpassRef(dst, 4*32+8, 32, src, 4*32+8, 32, 16, 1, 0, &c)
		for k := 0; k < 8; k++ {
			x := 16 - 8 + 3 - k // output column of block starting at column 8
			if got := dst[4*32+8+x]; got != c[k] {
				t.Errorf("filter %d: tap %d: impulse response %d, want %d", f, k, got, c[k])
			}
		}
	}
	// the vertical pass likewise
	for f, c := range lumaFilters {
		c := c
		w, rows := 8, 16
		mid := make([]int16, w*rows)
		mid[8*w+2] = 64
		out := make([]int32, w*rows)
		vpassRef(out, 8*w, w, mid, 8*w, w, w, 1, &c)
		_ = out
		// one output row y=8 (offset 8*w): taps rows 5..12 around the impulse at row 8:
		// the output of row r has the impulse at tap k = 8 - r + 3 when 0 <= k < 8
		for r := 4; r < 12; r++ {
			o := make([]int32, w*rows)
			vpassRef(o, r*w, w, mid, r*w, w, w, 1, &c)
			k := 8 - r + 3
			want := int32(0)
			if k >= 0 && k < 8 {
				want = int32(c[k]) * 64 >> 6
			}
			if o[r*w+2] != want {
				t.Errorf("filter %d: vertical, row %d: %d, want %d", f, r, o[r*w+2], want)
			}
		}
	}
}

func randomU16(rng *rand.Rand, n, bd int, extreme bool) []uint16 {
	s := make([]uint16, n)
	for i := range s {
		if extreme {
			s[i] = uint16(rng.Intn(2)) * uint16(1<<uint(bd)-1)
		} else {
			s[i] = uint16(rng.Intn(1 << uint(bd)))
		}
	}
	return s
}

func TestHPass8AgainstTheReference(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for iter := 0; iter < 6000; iter++ {
		w, h := 4*(1+rng.Intn(16)), 1+rng.Intn(64)
		bd := 8 + rng.Intn(5)
		c := lumaFilters[rng.Intn(3)]
		dstStride, srcStride := w+rng.Intn(9), w+7+rng.Intn(9)
		dst := make([]int16, h*dstStride+rng.Intn(5))
		want := make([]int16, len(dst))
		for i := range dst {
			dst[i], want[i] = sentinel16, sentinel16
		}
		srcOff := 3 + rng.Intn(3)
		src := randomU16(rng, srcOff+(h-1)*srcStride+w+4+rng.Intn(5), bd, iter%2 == 0)
		if err := TryHPass8(dst, 0, dstStride, src, srcOff, srcStride, w, h, bd, &c); err != nil {
			t.Fatalf("iter %d: %v", iter, err)
		}
		hpassRef(want, 0, dstStride, src, srcOff, srcStride, w, h, uint(bd-8), &c)
		for i := range dst {
			if dst[i] != want[i] {
				t.Fatalf("iter %d: %dx%d bit depth %d: element %d is %d, the reference gives %d", iter, w, h, bd, i, dst[i], want[i])
			}
		}
	}
}

func TestVPass8AgainstTheReference(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	for iter := 0; iter < 6000; iter++ {
		w, h := 4*(1+rng.Intn(16)), 1+rng.Intn(64)
		c := lumaFilters[rng.Intn(3)]
		dstStride, srcStride := w+rng.Intn(9), w+rng.Intn(9)
		dst := make([]int32, h*dstStride+rng.Intn(5))
		want := make([]int32, len(dst))
		for i := range dst {
			dst[i], want[i] = 0x5a5a5a5a, 0x5a5a5a5a
		}
		srcOff := 3*srcStride + rng.Intn(3)
		src := make([]int16, srcOff+(h+3)*srcStride+w+rng.Intn(5))
		for i := range src { // the bounds of the first stage, -6144 .. 22528, and its extremes
			switch {
			case iter%3 == 0:
				src[i] = [2]int16{-6144, 22528}[rng.Intn(2)]
			default:
				src[i] = int16(rng.Intn(22528+6144+1) - 6144)
			}
		}
		if err := TryVPass8(dst, 0, dstStride, src, srcOff, srcStride, w, h, &c); err != nil {
			t.Fatalf("iter %d: %v", iter, err)
		}
		vpassRef(want, 0, dstStride, src, srcOff, srcStride, w, h, &c)
		for i := range dst {
			if dst[i] != want[i] {
				t.Fatalf("iter %d: %dx%d: element %d is %d, the reference gives %d", iter, w, h, i, dst[i], want[i])
			}
		}
	}
}

func TestRefusals(t *testing.T) {
	c := lumaFilters[1]
	dst := make([]int16, 4096)
	src := make([]uint16, 8192)
	d32 := make([]int32, 4096)
	s16 := make([]int16, 8192)
	for name, f := range map[string]func() error{
		"width not a multiple of 4":   func() error { return TryHPass8(dst, 0, 64, src, 3, 80, 6, 4, 8, &c) },
		"width past 64":               func() error { return TryHPass8(dst, 0, 128, src, 3, 160, 68, 4, 8, &c) },
		"width 0":                     func() error { return TryHPass8(dst, 0, 64, src, 3, 80, 0, 4, 8, &c) },
		"height 0":                    func() error { return TryHPass8(dst, 0, 64, src, 3, 80, 8, 0, 8, &c) },
		"height past 64":              func() error { return TryHPass8(dst, 0, 64, src, 3, 80, 8, 65, 8, &c) },
		"bit depth 13":                func() error { return TryHPass8(dst, 0, 64, src, 3, 80, 8, 4, 13, &c) },
		"bit depth 7":                 func() error { return TryHPass8(dst, 0, 64, src, 3, 80, 8, 4, 7, &c) },
		"no coefficients":             func() error { return TryHPass8(dst, 0, 64, src, 3, 80, 8, 4, 8, nil) },
		"destination stride < w":      func() error { return TryHPass8(dst, 0, 4, src, 3, 80, 8, 4, 8, &c) },
		"source stride 0":             func() error { return TryHPass8(dst, 0, 64, src, 3, 0, 8, 4, 8, &c) },
		"destination does not fit":    func() error { return TryHPass8(dst, 4090, 64, src, 3, 80, 8, 4, 8, &c) },
		"source margin before":        func() error { return TryHPass8(dst, 0, 64, src, 2, 80, 8, 4, 8, &c) },
		"source margin after":         func() error { return TryHPass8(dst, 0, 64, src, 8192-8-3, 80, 8, 1, 8, &c) },
		"stride that wraps":           func() error { return TryHPass8(dst, 0, 64, src, 3, maxInt, 8, 4, 8, &c) },
		"empty slices":                func() error { return TryHPass8(nil, 0, 64, nil, 3, 80, 8, 4, 8, &c) },
		"vertical: width":             func() error { return TryVPass8(d32, 0, 64, s16, 3*80, 80, 5, 4, &c) },
		"vertical: source margin":     func() error { return TryVPass8(d32, 0, 64, s16, 2*80, 80, 8, 4, &c) },
		"vertical: destination":       func() error { return TryVPass8(d32, 4090, 64, s16, 3*80, 80, 8, 4, &c) },
		"vertical: stride that wraps": func() error { return TryVPass8(d32, 0, 64, s16, 3*80, maxInt, 8, 4, &c) },
		"vertical: source stride 0":   func() error { return TryVPass8(d32, 0, 64, s16, 3*80, 0, 8, 4, &c) },
		"vertical: no coefficients":   func() error { return TryVPass8(d32, 0, 64, s16, 3*80, 80, 8, 4, nil) },
	} {
		for i := range dst {
			dst[i] = sentinel16
		}
		err := f()
		if !errors.Is(err, ErrRefused) {
			t.Errorf("%s: %v", name, err)
		}
		for i := range dst {
			if dst[i] != sentinel16 {
				t.Fatalf("%s: a refused call wrote dst[%d]", name, i)
			}
		}
	}
	// the shared memory of the two sides
	mem := make([]uint16, 4096)
	d16 := unsafeInt16(mem)
	if err := TryHPass8(d16[:2048], 0, 64, mem[:], 0+3, 64+8, 8, 4, 8, &c); !errors.Is(err, ErrRefused) {
		t.Errorf("overlap: %v", err)
	}
	mid := make([]int16, 4096)
	d := unsafeInt32(mid)
	if err := TryVPass8(d[:1024], 0, 64, mid, 3*64, 64, 8, 4, &c); !errors.Is(err, ErrRefused) {
		t.Errorf("vertical overlap: %v", err)
	}
	// the panicking forms
	mustPanic(t, "HPass8", func() { HPass8(dst, 0, 64, src, 0, 80, 8, 4, 8, &c) })
	mustPanic(t, "VPass8", func() { VPass8(d32, 0, 64, s16, 0, 80, 8, 4, &c) })
	HPass8(dst, 0, 64, src, 3, 80, 8, 4, 8, &c)
	VPass8(d32, 0, 64, s16, 3*80, 80, 8, 4, &c)
}

func mustPanic(t *testing.T, what string, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s did not panic", what)
		}
	}()
	f()
}

// Beyond the stated bound the three implementations still agree: a sum that does not fit
// int16 keeps its low 16 bits (the reference's conversion, NEON's XTN, and the AVX2 byte
// shuffle: not a saturating pack, which gave 32767 where the reference gave 29016 on a
// seed of the fuzz corpus). Samples up to 32767 are in the domain whatever the bit depth.
func TestBeyondTheBoundTheResultWrapsLikeTheReference(t *testing.T) {
	rng := rand.New(rand.NewSource(6))
	for iter := 0; iter < 3000; iter++ {
		w, h := 4*(1+rng.Intn(16)), 1+rng.Intn(8)
		bd := 8 + rng.Intn(5)
		c := lumaFilters[rng.Intn(3)]
		stride := w + 7
		src := make([]uint16, (h-1)*stride+w+7)
		for i := range src {
			src[i] = uint16(rng.Intn(32768))
			if iter%4 == 0 {
				src[i] = 32767
			}
		}
		got, want := make([]int16, h*w), make([]int16, h*w)
		if err := TryHPass8(got, 0, w, src, 3, stride, w, h, bd, &c); err != nil {
			t.Fatal(err)
		}
		hpassRef(want, 0, w, src, 3, stride, w, h, uint(bd-8), &c)
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("iter %d: bit depth %d: element %d is %d, the reference gives %d", iter, bd, i, got[i], want[i])
			}
		}
	}
	// the seed that failed on amd64: eight bits, samples up to 2040
	c := lumaFilters[0]
	src := make([]uint16, 4*14+4)
	for i := range src {
		src[i] = uint16(i*37%256) << 3
	}
	got, want := make([]int16, 32), make([]int16, 32)
	if err := TryHPass8(got, 0, 8, src, 3, 14, 8, 4, 8, &c); err != nil {
		t.Fatal(err)
	}
	hpassRef(want, 0, 8, src, 3, 14, 8, 4, 0, &c)
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("seed: element %d is %d, the reference gives %d", i, got[i], want[i])
		}
	}
}
