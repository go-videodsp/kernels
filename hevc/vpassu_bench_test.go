package hevc

import (
	"fmt"
	"math/rand"
	"testing"
)

// Benchmarks of the vertical filter over picture samples against the portable reference, per
// block size, luma and chroma.
func BenchmarkVPassU(b *testing.B) {
	rng := rand.New(rand.NewSource(21))
	for _, n := range []int{8, 16, 32, 64} {
		src := randomU16(rng, (n+7)*n, 8, false)
		dst := make([]int32, n*n)
		b.Run(fmt.Sprintf("Luma/%dx%d/kernel", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryVPassU8(dst, 0, n, src, 3*n, n, n, n, 8, &lumaFilters[1])
			}
		})
		b.Run(fmt.Sprintf("Luma/%dx%d/reference", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				vpassU8Ref(dst, 0, n, src, 3*n, n, n, n, 0, &lumaFilters[1])
			}
		})
		b.Run(fmt.Sprintf("Chroma/%dx%d/kernel", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryVPassU4(dst, 0, n, src, n, n, n, n, 8, &chromaFilters[3])
			}
		})
		b.Run(fmt.Sprintf("Chroma/%dx%d/reference", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				vpassU4Ref(dst, 0, n, src, n, n, n, n, 0, &chromaFilters[3])
			}
		})
	}
}
