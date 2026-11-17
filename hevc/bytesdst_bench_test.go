package hevc

import (
	"fmt"
	"math/rand"
	"testing"
)

// Benchmarks of the byte-destination kernels against the same kernel over uint16 samples (what an
// 8-bit picture used before, with a narrowing copy of the block after it that is not counted here)
// and against the portable reference, per block size.
func BenchmarkBytesDst(b *testing.B) {
	rng := rand.New(rand.NewSource(23))
	off := [4]int16{3, -2, 5, -1}
	for _, n := range []int{8, 16, 32, 64} {
		sS := n + 2
		a8, b8 := randomBytes(rng, (n+2)*sS, false), randomBytes(rng, (n+2)*sS, false)
		a16, b16 := make([]uint16, len(a8)), make([]uint16, len(b8))
		for i := range a8 {
			a16[i], b16[i] = uint16(a8[i]), uint16(b8[i])
		}
		p0, p1 := make([]int32, n*n), make([]int32, n*n)
		for i := range p0 {
			p0[i], p1[i] = int32(rng.Intn(256)<<6), int32(rng.Intn(256)<<6)
		}
		d8, d16 := make([]uint8, n*n), make([]uint16, n*n)
		b.Run(fmt.Sprintf("Average/%dx%d/bytes", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryAverageBytes(d8, 0, n, a8, 0, sS, b8, 0, sS, n, n)
			}
		})
		b.Run(fmt.Sprintf("Average/%dx%d/uint16", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryAverage(d16, 0, n, a16, 0, sS, b16, 0, sS, n, n)
			}
		})
		b.Run(fmt.Sprintf("Average/%dx%d/reference", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				averageRef(d8, 0, n, a8, 0, sS, b8, 0, sS, n, n)
			}
		})
		b.Run(fmt.Sprintf("WeightUni/%dx%d/bytes", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryWeightUniBytes(d8, 0, n, p0, 0, n, n, n, 6, 1, 0)
			}
		})
		b.Run(fmt.Sprintf("WeightUni/%dx%d/uint16", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryWeightUni(d16, 0, n, p0, 0, n, n, n, 8, 6, 1, 0)
			}
		})
		b.Run(fmt.Sprintf("WeightBi/%dx%d/bytes", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryWeightBiBytes(d8, 0, n, p0, p1, 0, n, n, n, 6, 1, 1, 0, 0)
			}
		})
		b.Run(fmt.Sprintf("WeightBi/%dx%d/uint16", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryWeightBi(d16, 0, n, p0, p1, 0, n, n, n, 8, 6, 1, 1, 0, 0)
			}
		})
		b.Run(fmt.Sprintf("WeightBi/%dx%d/reference", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				weightBiRef(d8, 0, n, p0, p1, 0, n, n, n, 6, &[4]int32{1, 1, 1 << 6, 255})
			}
		})
		b.Run(fmt.Sprintf("SAOBand/%dx%d/bytes", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TrySAOBandBytes(d8, 0, n, a8, 0, sS, n, n, 3, &off)
			}
		})
		b.Run(fmt.Sprintf("SAOBand/%dx%d/uint16", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TrySAOBand(d16, 0, n, a16, 0, sS, n, n, 8, 3, &off)
			}
		})
		b.Run(fmt.Sprintf("SAOEdge/%dx%d/bytes", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TrySAOEdgeBytes(d8, 0, n, a8, sS+1, sS, n, n, 2, &off)
			}
		})
		b.Run(fmt.Sprintf("SAOEdge/%dx%d/uint16", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TrySAOEdge(d16, 0, n, a16, sS+1, sS, n, n, 8, 2, &off)
			}
		})
	}
}
