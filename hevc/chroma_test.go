package hevc

import (
	"errors"
	"math/rand"
	"testing"
)

// The seven chroma filters of ITU-T H.265 (V11) (01/2026), Equations 8-241 to 8-247, for the
// eighth-sample positions 1 to 7, taps at offsets -1 to 2.
var chromaFilters = [7][4]int16{
	{-2, 58, 10, -2},
	{-4, 54, 16, -2},
	{-6, 46, 28, -4},
	{-4, 36, 36, -4},
	{-4, 28, 46, -6},
	{-2, 16, 54, -4},
	{-2, 10, 58, -2},
}

// The impulse response shows the taps: the output at offset 1-k from the impulse is c[k].
func TestChromaImpulseResponseIsTheFilterTaps(t *testing.T) {
	for f, c := range chromaFilters {
		c := c
		src := make([]uint16, 16*3)
		src[16+8] = 1
		dst := make([]int16, 16*3)
		hpass4Ref(dst, 16+4, 16, src, 16+4, 16, 8, 1, 0, &c)
		for k := 0; k < 4; k++ {
			if got := dst[16+4+(8-4)+1-k]; got != c[k] {
				t.Errorf("filter %d: tap %d: impulse response %d, want %d", f, k, got, c[k])
			}
		}
		mid := make([]int16, 8*8)
		mid[3*8+2] = 64
		for r := 1; r < 5; r++ {
			o := make([]int32, 8*8)
			vpass4Ref(o, r*8, 8, mid, r*8, 8, 8, 1, &c)
			k := 3 - r + 1
			want := int32(0)
			if k >= 0 && k < 4 {
				want = int32(c[k])
			}
			if o[r*8+2] != want {
				t.Errorf("filter %d: vertical row %d: %d, want %d", f, r, o[r*8+2], want)
			}
		}
	}
}

func TestChromaPassesAgainstTheReference(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for iter := 0; iter < 8000; iter++ {
		w, h := 2*(1+rng.Intn(32)), 1+rng.Intn(64)
		bd := 8 + rng.Intn(5)
		c := chromaFilters[rng.Intn(7)]
		dStride, sStride := w+rng.Intn(9), w+3+rng.Intn(9)
		off := 1 + rng.Intn(3)
		src := randomU16(rng, off+(h-1)*sStride+w+2+rng.Intn(5), bd, iter%2 == 0)
		got := make([]int16, h*dStride+rng.Intn(5))
		want := make([]int16, len(got))
		for i := range got {
			got[i], want[i] = sentinel16, sentinel16
		}
		if err := TryHPass4(got, 0, dStride, src, off, sStride, w, h, bd, &c); err != nil {
			t.Fatalf("iter %d: %v", iter, err)
		}
		hpass4Ref(want, 0, dStride, src, off, sStride, w, h, uint(bd-8), &c)
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("iter %d: horizontal %dx%d bit depth %d: element %d is %d, the reference gives %d", iter, w, h, bd, i, got[i], want[i])
			}
		}
		// vertical, on intermediates within the first stage's bounds
		vs := w + rng.Intn(9)
		voff := vs + rng.Intn(3)
		mid := make([]int16, voff+(h+2)*vs+w+rng.Intn(5))
		for i := range mid {
			mid[i] = int16(rng.Intn(22528+6144+1) - 6144)
			if iter%3 == 0 {
				mid[i] = [2]int16{-6144, 22528}[rng.Intn(2)]
			}
		}
		g32 := make([]int32, h*dStride+rng.Intn(5))
		w32 := make([]int32, len(g32))
		for i := range g32 {
			g32[i], w32[i] = 0x5a5a5a5a, 0x5a5a5a5a
		}
		if err := TryVPass4(g32, 0, dStride, mid, voff, vs, w, h, &c); err != nil {
			t.Fatalf("iter %d: %v", iter, err)
		}
		vpass4Ref(w32, 0, dStride, mid, voff, vs, w, h, &c)
		for i := range g32 {
			if g32[i] != w32[i] {
				t.Fatalf("iter %d: vertical %dx%d: element %d is %d, the reference gives %d", iter, w, h, i, g32[i], w32[i])
			}
		}
	}
}

func TestChromaRefusals(t *testing.T) {
	c := chromaFilters[3]
	dst := make([]int16, 4096)
	src := make([]uint16, 8192)
	d32 := make([]int32, 4096)
	s16 := make([]int16, 8192)
	for name, f := range map[string]func() error{
		"odd width":                   func() error { return TryHPass4(dst, 0, 64, src, 1, 80, 5, 4, 8, &c) },
		"width 0":                     func() error { return TryHPass4(dst, 0, 64, src, 1, 80, 0, 4, 8, &c) },
		"width past 64":               func() error { return TryHPass4(dst, 0, 128, src, 1, 160, 66, 4, 8, &c) },
		"height 0":                    func() error { return TryHPass4(dst, 0, 64, src, 1, 80, 8, 0, 8, &c) },
		"bit depth 13":                func() error { return TryHPass4(dst, 0, 64, src, 1, 80, 8, 4, 13, &c) },
		"no coefficients":             func() error { return TryHPass4(dst, 0, 64, src, 1, 80, 8, 4, 8, nil) },
		"source margin before":        func() error { return TryHPass4(dst, 0, 64, src, 0, 80, 8, 4, 8, &c) },
		"source margin after":         func() error { return TryHPass4(dst, 0, 64, src, 8192-8-1, 80, 8, 1, 8, &c) },
		"stride that wraps":           func() error { return TryHPass4(dst, 0, 64, src, 1, maxInt, 8, 4, 8, &c) },
		"vertical: odd width":         func() error { return TryVPass4(d32, 0, 64, s16, 80, 80, 7, 4, &c) },
		"vertical: source margin":     func() error { return TryVPass4(d32, 0, 64, s16, 0, 80, 8, 4, &c) },
		"vertical: stride that wraps": func() error { return TryVPass4(d32, 0, 64, s16, 80, maxInt, 8, 4, &c) },
		"vertical: no coefficients":   func() error { return TryVPass4(d32, 0, 64, s16, 80, 80, 8, 4, nil) },
	} {
		if err := f(); !errors.Is(err, ErrRefused) {
			t.Errorf("%s: %v", name, err)
		}
	}
	mustPanic(t, "HPass4", func() { HPass4(dst, 0, 64, src, 0, 80, 8, 4, 8, &c) })
	mustPanic(t, "VPass4", func() { VPass4(d32, 0, 64, s16, 0, 80, 8, 4, &c) })
	HPass4(dst, 0, 64, src, 1, 80, 8, 4, 8, &c)
	VPass4(d32, 0, 64, s16, 80, 80, 8, 4, &c)
	// shared memory
	mem := make([]uint16, 4096)
	if err := TryHPass4(unsafeInt16(mem)[:2048], 0, 64, mem, 1, 64+4, 8, 4, 8, &c); !errors.Is(err, ErrRefused) {
		t.Errorf("overlap: %v", err)
	}
}

// Beyond the bound the chroma pass wraps like the reference too (see the luma test).
func TestChromaBeyondTheBoundTheResultWrapsLikeTheReference(t *testing.T) {
	rng := rand.New(rand.NewSource(8))
	for iter := 0; iter < 2000; iter++ {
		w, h := 2*(1+rng.Intn(32)), 1+rng.Intn(6)
		bd := 8 + rng.Intn(5)
		c := chromaFilters[rng.Intn(7)]
		stride := w + 3
		src := make([]uint16, (h-1)*stride+w+3)
		for i := range src {
			src[i] = uint16(rng.Intn(32768))
		}
		got, want := make([]int16, h*w), make([]int16, h*w)
		if err := TryHPass4(got, 0, w, src, 1, stride, w, h, bd, &c); err != nil {
			t.Fatal(err)
		}
		hpass4Ref(want, 0, w, src, 1, stride, w, h, uint(bd-8), &c)
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("iter %d: element %d is %d, the reference gives %d", iter, i, got[i], want[i])
			}
		}
	}
}
