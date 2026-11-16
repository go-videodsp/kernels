//go:build amd64 && !purego

package hevc

import (
	"math/rand"
	"testing"
)

// Without AVX2 the passes run the portable reference: the same results, by the same
// entry points.
func TestPortablePathWithoutAVX2(t *testing.T) {
	old := useAVX2
	defer func() { useAVX2 = old }()
	rng := rand.New(rand.NewSource(4))
	c := lumaFilters[0]
	for _, avx := range []bool{false, true} {
		useAVX2 = avx && old
		src := randomU16(rng, 40*20, 10, false)
		got, want := make([]int16, 16*8), make([]int16, 16*8)
		if err := TryHPass8(got, 0, 16, src, 3, 40, 16, 8, 10, &c); err != nil {
			t.Fatal(err)
		}
		hpassRef(want, 0, 16, src, 3, 40, 16, 8, 2, &c)
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("avx2=%v: element %d is %d, want %d", useAVX2, i, got[i], want[i])
			}
		}
		mid := make([]int16, 40*20)
		for i := range mid {
			mid[i] = int16(rng.Intn(20000))
		}
		g32, w32 := make([]int32, 16*8), make([]int32, 16*8)
		if err := TryVPass8(g32, 0, 16, mid, 3*40, 40, 16, 8, &c); err != nil {
			t.Fatal(err)
		}
		vpassRef(w32, 0, 16, mid, 3*40, 40, 16, 8, &c)
		for i := range g32 {
			if g32[i] != w32[i] {
				t.Fatalf("avx2=%v: vertical element %d is %d, want %d", useAVX2, i, g32[i], w32[i])
			}
		}
	}
}

// The same for the chroma passes.
func TestChromaPortablePathWithoutAVX2(t *testing.T) {
	old := useAVX2
	defer func() { useAVX2 = old }()
	rng := rand.New(rand.NewSource(10))
	c := chromaFilters[3]
	for _, avx := range []bool{false, true} {
		useAVX2 = avx && old
		src := randomU16(rng, 40*20, 10, false)
		got, want := make([]int16, 6*8), make([]int16, 6*8)
		if err := TryHPass4(got, 0, 6, src, 1, 40, 6, 8, 10, &c); err != nil {
			t.Fatal(err)
		}
		hpass4Ref(want, 0, 6, src, 1, 40, 6, 8, 2, &c)
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("avx2=%v: element %d is %d, want %d", useAVX2, i, got[i], want[i])
			}
		}
		mid := make([]int16, 40*20)
		for i := range mid {
			mid[i] = int16(rng.Intn(20000))
		}
		g32, w32 := make([]int32, 6*8), make([]int32, 6*8)
		if err := TryVPass4(g32, 0, 6, mid, 40, 40, 6, 8, &c); err != nil {
			t.Fatal(err)
		}
		vpass4Ref(w32, 0, 6, mid, 40, 40, 6, 8, &c)
		for i := range g32 {
			if g32[i] != w32[i] {
				t.Fatalf("avx2=%v: vertical element %d is %d, want %d", useAVX2, i, g32[i], w32[i])
			}
		}
	}
}

// Without AVX2 the SAO kernels run the portable reference.
func TestSAOPortablePathWithoutAVX2(t *testing.T) {
	old := useAVX2
	defer func() { useAVX2 = old }()
	rng := rand.New(rand.NewSource(12))
	off := [4]int16{-4, -1, 2, 6}
	for _, avx := range []bool{false, true} {
		useAVX2 = avx && old
		src := randomU16(rng, 24*20, 10, false)
		got, want := make([]uint16, 16*8), make([]uint16, 16*8)
		if err := TrySAOBand(got, 0, 16, src, 0, 24, 16, 8, 10, 5, &off); err != nil {
			t.Fatal(err)
		}
		saoBandRef(want, 0, 16, src, 0, 24, 16, 8, 10, 5, &off)
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("avx2=%v band: element %d is %d, want %d", useAVX2, i, got[i], want[i])
			}
		}
		for i := range got {
			got[i], want[i] = 0, 0
		}
		if err := TrySAOEdge(got, 0, 16, src, 25, 24, 16, 8, 10, 2, &off); err != nil {
			t.Fatal(err)
		}
		a, b := saoEdgeOffsets(2, 24)
		saoEdgeRef(want, 0, 16, src, 25, 24, 16, 8, 10, a, b, &off)
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("avx2=%v edge: element %d is %d, want %d", useAVX2, i, got[i], want[i])
			}
		}
	}
}

// Without AVX2 the weighting kernels run the portable reference.
func TestWeightPortablePathWithoutAVX2(t *testing.T) {
	old := useAVX2
	defer func() { useAVX2 = old }()
	rng := rand.New(rand.NewSource(14))
	for _, avx := range []bool{false, true} {
		useAVX2 = avx && old
		p0, p1 := randomIntermediates(rng, 16*8, 0), randomIntermediates(rng, 16*8, 0)
		got := make([]uint16, 16*8)
		if err := TryWeightUni(got, 0, 16, p0, 0, 16, 16, 8, 10, 4, 3, 2); err != nil {
			t.Fatal(err)
		}
		for i := range got {
			if want := modelUni(int64(p0[i]), 3, 2, 4, 10); got[i] != want {
				t.Fatalf("avx2=%v uni: element %d is %d, want %d", useAVX2, i, got[i], want)
			}
		}
		if err := TryWeightBi(got, 0, 16, p0, p1, 0, 16, 16, 8, 10, 4, 3, 5, 2, -1); err != nil {
			t.Fatal(err)
		}
		for i := range got {
			if want := modelBi(int64(p0[i]), int64(p1[i]), 3, 5, 2, -1, 4, 10); got[i] != want {
				t.Fatalf("avx2=%v bi: element %d is %d, want %d", useAVX2, i, got[i], want)
			}
		}
	}
}

// Without AVX2 the averaging and the transform kernels run the portable reference.
func TestAverageAndIDCTPortablePathWithoutAVX2(t *testing.T) {
	old := useAVX2
	defer func() { useAVX2 = old }()
	rng := rand.New(rand.NewSource(24))
	for _, avx := range []bool{false, true} {
		useAVX2 = avx && old
		a, b := randomSamples(rng, 16*8, 2), randomSamples(rng, 16*8, 2)
		got := make([]uint16, 16*8)
		if err := TryAverage(got, 0, 16, a, 0, 16, b, 0, 16, 16, 8); err != nil {
			t.Fatal(err)
		}
		for i := range got {
			if want := modelAverage(a[i], b[i]); got[i] != want {
				t.Fatalf("avx2=%v average: element %d is %d, want %d", useAVX2, i, got[i], want)
			}
		}
		for _, n := range []int{4, 8} { // the two AVX2 kernels, 4 lanes and 8
			d := randomCoefficients(rng, n, 1)
			out := make([]int32, n*n)
			if err := TryIDCT(out, d, map[int]int{4: 2, 8: 3}[n], 12); err != nil {
				t.Fatal(err)
			}
			for i, w := range modelTransform(d, n, 12, dctEntry(n)) {
				if out[i] != w {
					t.Fatalf("avx2=%v idct %dx%d: element %d is %d, want %d", useAVX2, n, n, i, out[i], w)
				}
			}
		}
	}
}

// Without AVX2 the vertical filters over picture samples run the portable reference.
func TestVPassUPortablePathWithoutAVX2(t *testing.T) {
	old := useAVX2
	defer func() { useAVX2 = old }()
	rng := rand.New(rand.NewSource(25))
	for _, avx := range []bool{false, true} {
		useAVX2 = avx && old
		src := randomU16(rng, 16*16, 10, false)
		got := make([]int32, 16*8)
		if err := TryVPassU8(got, 0, 16, src, 3*16, 16, 16, 8, 10, &lumaFilters[1]); err != nil {
			t.Fatal(err)
		}
		for i := range got {
			if want := modelVU(src, 3*16+i, 16, lumaFilters[1][:], 3, 10); got[i] != want {
				t.Fatalf("avx2=%v 8 taps: element %d is %d, want %d", useAVX2, i, got[i], want)
			}
		}
		if err := TryVPassU4(got, 0, 16, src, 16, 16, 16, 8, 10, &chromaFilters[3]); err != nil {
			t.Fatal(err)
		}
		for i := range got {
			if want := modelVU(src, 16+i, 16, chromaFilters[3][:], 1, 10); got[i] != want {
				t.Fatalf("avx2=%v 4 taps: element %d is %d, want %d", useAVX2, i, got[i], want)
			}
		}
	}
}

// Without AVX2 the horizontal filters into int32 and the widening copy run the portable reference.
func TestHPassWAndWidenPortablePathWithoutAVX2(t *testing.T) {
	old := useAVX2
	defer func() { useAVX2 = old }()
	rng := rand.New(rand.NewSource(26))
	for _, avx := range []bool{false, true} {
		useAVX2 = avx && old
		src := randomU16(rng, 8*32, 10, false)
		got := make([]int32, 16*8)
		if err := TryHPassW8(got, 0, 16, src, 3, 32, 16, 8, 10, &lumaFilters[1]); err != nil {
			t.Fatal(err)
		}
		for y := 0; y < 8; y++ {
			for x := 0; x < 16; x++ {
				if want := modelHW(src, 3+y*32+x, lumaFilters[1][:], 3, 10); got[y*16+x] != want {
					t.Fatalf("avx2=%v 8 taps: (%d,%d) is %d, want %d", useAVX2, x, y, got[y*16+x], want)
				}
			}
		}
		if err := TryHPassW4(got, 0, 16, src, 1, 32, 16, 8, 10, &chromaFilters[3]); err != nil {
			t.Fatal(err)
		}
		for y := 0; y < 8; y++ {
			for x := 0; x < 16; x++ {
				if want := modelHW(src, 1+y*32+x, chromaFilters[3][:], 1, 10); got[y*16+x] != want {
					t.Fatalf("avx2=%v 4 taps: (%d,%d) is %d, want %d", useAVX2, x, y, got[y*16+x], want)
				}
			}
		}
		if err := TryWiden(got, 0, 16, src, 0, 32, 16, 8, 4); err != nil {
			t.Fatal(err)
		}
		for y := 0; y < 8; y++ {
			for x := 0; x < 16; x++ {
				if want := int32(src[y*32+x]) << 4; got[y*16+x] != want {
					t.Fatalf("avx2=%v widen: (%d,%d) is %d, want %d", useAVX2, x, y, got[y*16+x], want)
				}
			}
		}
	}
}
