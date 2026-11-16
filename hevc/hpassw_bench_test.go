package hevc

import (
	"fmt"
	"math/rand"
	"testing"
)

// Benchmarks of the horizontal filters into int32 and the widening copy against the portable
// reference (and, for the filters, the HPass8 plus widening loop they replace), per block size.
func BenchmarkHPassW(b *testing.B) {
	rng := rand.New(rand.NewSource(23))
	for _, n := range []int{8, 16, 32, 64} {
		src := randomU16(rng, n*(n+7), 8, false)
		dst := make([]int32, n*n)
		mid := make([]int16, n*n)
		b.Run(fmt.Sprintf("Luma/%dx%d/kernel", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryHPassW8(dst, 0, n, src, 3, n+7, n, n, 8, &lumaFilters[1])
			}
		})
		b.Run(fmt.Sprintf("Luma/%dx%d/int16-then-widen", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryHPass8(mid, 0, n, src, 3, n+7, n, n, 8, &lumaFilters[1])
				for j, v := range mid {
					dst[j] = int32(v)
				}
			}
		})
		b.Run(fmt.Sprintf("Luma/%dx%d/reference", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				hpassW8Ref(dst, 0, n, src, 3, n+7, n, n, 0, &lumaFilters[1])
			}
		})
		b.Run(fmt.Sprintf("Widen/%dx%d/kernel", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryWiden(dst, 0, n, src, 0, n+7, n, n, 6)
			}
		})
		b.Run(fmt.Sprintf("Widen/%dx%d/reference", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				widenRef(dst, 0, n, src, 0, n+7, n, n, 6)
			}
		})
	}
}
