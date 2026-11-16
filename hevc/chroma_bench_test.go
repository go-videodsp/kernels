package hevc

import (
	"fmt"
	"math/rand"
	"testing"
)

// Benchmarks of the four-tap passes against the portable reference.
func BenchmarkChromaPasses(b *testing.B) {
	rng := rand.New(rand.NewSource(11))
	c := chromaFilters[3]
	for _, n := range []int{4, 8, 16, 32} {
		stride := n + 16
		src := randomU16(rng, (n+3)*stride, 8, false)
		mid := make([]int16, (n+3)*stride)
		for i := range mid {
			mid[i] = int16(rng.Intn(20000))
		}
		dst16 := make([]int16, n*stride)
		dst32 := make([]int32, n*stride)
		b.Run(fmt.Sprintf("HPass4/%dx%d/kernel", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryHPass4(dst16, 0, stride, src, 1, stride, n, n, 8, &c)
			}
		})
		b.Run(fmt.Sprintf("HPass4/%dx%d/reference", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				hpass4Ref(dst16, 0, stride, src, 1, stride, n, n, 0, &c)
			}
		})
		b.Run(fmt.Sprintf("VPass4/%dx%d/kernel", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryVPass4(dst32, 0, stride, mid, stride, stride, n, n, &c)
			}
		})
		b.Run(fmt.Sprintf("VPass4/%dx%d/reference", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				vpass4Ref(dst32, 0, stride, mid, stride, stride, n, n, &c)
			}
		})
	}
}
