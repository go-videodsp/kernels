package hevc

import (
	"fmt"
	"math/rand"
	"testing"
)

// Benchmarks of the two passes against the portable reference, per block size: the
// reference is what every target without a kernel runs.
func BenchmarkPasses(b *testing.B) {
	rng := rand.New(rand.NewSource(5))
	c := lumaFilters[1]
	for _, n := range []int{8, 16, 32, 64} {
		stride := n + 16
		src := randomU16(rng, (n+7)*stride, 8, false)
		mid := make([]int16, (n+7)*stride)
		for i := range mid {
			mid[i] = int16(rng.Intn(20000))
		}
		dst16 := make([]int16, n*stride)
		dst32 := make([]int32, n*stride)
		b.Run(fmt.Sprintf("HPass8/%dx%d/kernel", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryHPass8(dst16, 0, stride, src, 3, stride, n, n, 8, &c)
			}
		})
		b.Run(fmt.Sprintf("HPass8/%dx%d/reference", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				hpassRef(dst16, 0, stride, src, 3, stride, n, n, 0, &c)
			}
		})
		b.Run(fmt.Sprintf("VPass8/%dx%d/kernel", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryVPass8(dst32, 0, stride, mid, 3*stride, stride, n, n, &c)
			}
		})
		b.Run(fmt.Sprintf("VPass8/%dx%d/reference", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				vpassRef(dst32, 0, stride, mid, 3*stride, stride, n, n, &c)
			}
		})
	}
}
