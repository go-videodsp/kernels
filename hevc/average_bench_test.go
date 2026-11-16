package hevc

import (
	"fmt"
	"math/rand"
	"testing"
)

// Benchmarks of the averaging kernel against the portable reference, per block size.
func BenchmarkAverage(b *testing.B) {
	rng := rand.New(rand.NewSource(17))
	for _, n := range []int{8, 16, 32, 64} {
		x, y := randomSamples(rng, n*n, 0), randomSamples(rng, n*n, 0)
		dst := make([]uint16, n*n)
		b.Run(fmt.Sprintf("%dx%d/kernel", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryAverage(dst, 0, n, x, 0, n, y, 0, n, n, n)
			}
		})
		b.Run(fmt.Sprintf("%dx%d/reference", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				averageRef(dst, 0, n, x, 0, n, y, 0, n, n, n)
			}
		})
	}
}
