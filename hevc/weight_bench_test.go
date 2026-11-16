package hevc

import (
	"fmt"
	"math/rand"
	"testing"
)

// Benchmarks of the weighting kernels against the portable reference, per block size.
func BenchmarkWeight(b *testing.B) {
	rng := rand.New(rand.NewSource(15))
	for _, n := range []int{8, 16, 32, 64} {
		p0, p1 := randomIntermediates(rng, n*n, 0), randomIntermediates(rng, n*n, 0)
		dst := make([]uint16, n*n)
		q := [4]int32{3, 1 << 5, 2, 255}
		qb := [4]int32{3, 5, 1 << 5, 255}
		b.Run(fmt.Sprintf("Uni/%dx%d/kernel", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryWeightUni(dst, 0, n, p0, 0, n, n, n, 8, 6, 3, 2)
			}
		})
		b.Run(fmt.Sprintf("Uni/%dx%d/reference", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				weightUniRef(dst, 0, n, p0, 0, n, n, n, 6, &q)
			}
		})
		b.Run(fmt.Sprintf("Bi/%dx%d/kernel", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryWeightBi(dst, 0, n, p0, p1, 0, n, n, n, 8, 6, 3, 5, 2, 0)
			}
		})
		b.Run(fmt.Sprintf("Bi/%dx%d/reference", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				weightBiRef(dst, 0, n, p0, p1, 0, n, n, n, 6, &qb)
			}
		})
	}
}
