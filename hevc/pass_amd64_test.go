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
