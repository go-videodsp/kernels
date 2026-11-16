package hevc

import (
	"fmt"
	"math/rand"
	"testing"
)

// Benchmarks of the SAO kernels against the portable reference, per block size.
func BenchmarkSAO(b *testing.B) {
	rng := rand.New(rand.NewSource(13))
	off := [4]int16{-3, -1, 2, 4}
	for _, n := range []int{16, 32, 64} {
		stride := n + 2
		src := randomU16(rng, (n+2)*stride, 8, false)
		dst := make([]uint16, n*stride)
		b.Run(fmt.Sprintf("Band/%dx%d/kernel", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TrySAOBand(dst, 0, stride, src, 0, stride, n, n, 8, 9, &off)
			}
		})
		b.Run(fmt.Sprintf("Band/%dx%d/reference", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				saoBandRef(dst, 0, stride, src, 0, stride, n, n, 8, 9, &off)
			}
		})
		offA, offB := saoEdgeOffsets(2, stride)
		b.Run(fmt.Sprintf("Edge/%dx%d/kernel", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TrySAOEdge(dst, 0, stride, src, stride+1, stride, n, n, 8, 2, &off)
			}
		})
		b.Run(fmt.Sprintf("Edge/%dx%d/reference", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				saoEdgeRef(dst, 0, stride, src, stride+1, stride, n, n, 8, offA, offB, &off)
			}
		})
	}
}
