//go:build amd64 && !purego

package hevc

import (
	"math/rand"
	"testing"
)

// Without AVX2 the byte-destination kernels run the portable reference: the same results, by the
// same entry points.
func TestBytesDstPortablePathWithoutAVX2(t *testing.T) {
	old := useAVX2
	defer func() { useAVX2 = old }()
	rng := rand.New(rand.NewSource(86))
	off := [4]int16{3, -2, 5, -1}
	for _, avx := range []bool{false, true} {
		useAVX2 = avx && old
		a, b := randomBytes(rng, 40*20, false), randomBytes(rng, 40*20, false)
		got, want := make([]uint8, 16*8), make([]uint8, 16*8)
		if err := TryAverageBytes(got, 0, 16, a, 3, 40, b, 5, 40, 16, 8); err != nil {
			t.Fatal(err)
		}
		averageRef(want, 0, 16, a, 3, 40, b, 5, 40, 16, 8)
		p := make([]int32, 16*8)
		for i := range p {
			p[i] = int32(rng.Intn(256) << 6)
		}
		g2, w2 := make([]uint8, 16*8), make([]uint8, 16*8)
		if err := TryWeightUniBytes(g2, 0, 16, p, 0, 16, 16, 8, 6, 1, 0); err != nil {
			t.Fatal(err)
		}
		weightUniRef(w2, 0, 16, p, 0, 16, 16, 8, 6, &[4]int32{1, 32, 0, 255})
		g3, w3 := make([]uint8, 16*8), make([]uint8, 16*8)
		if err := TryWeightBiBytes(g3, 0, 16, p, p, 0, 16, 16, 8, 6, 1, 1, 0, 0); err != nil {
			t.Fatal(err)
		}
		weightBiRef(w3, 0, 16, p, p, 0, 16, 16, 8, 6, &[4]int32{1, 1, 1 << 6, 255})
		g4, w4 := make([]uint8, 16*8), make([]uint8, 16*8)
		if err := TrySAOBandBytes(g4, 0, 16, a, 0, 40, 16, 8, 3, &off); err != nil {
			t.Fatal(err)
		}
		saoBandRef(w4, 0, 16, a, 0, 40, 16, 8, 8, 3, &off)
		g5, w5 := make([]uint8, 16*8), make([]uint8, 16*8)
		if err := TrySAOEdgeBytes(g5, 0, 16, a, 41, 40, 16, 8, 1, &off); err != nil {
			t.Fatal(err)
		}
		offA, offB := saoEdgeOffsets(1, 40)
		saoEdgeRef(w5, 0, 16, a, 41, 40, 16, 8, 8, offA, offB, &off)
		for i := range got {
			if got[i] != want[i] || g2[i] != w2[i] || g3[i] != w3[i] || g4[i] != w4[i] || g5[i] != w5[i] {
				t.Fatalf("avx2=%v: element %d differs from the reference", useAVX2, i)
			}
		}
	}
}
