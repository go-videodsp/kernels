package hevc

import (
	"fmt"
	"math/rand"
	"testing"
)

// Benchmarks of the transform kernels against the portable reference: a dense block and a
// sparse one (the low-frequency corner, as a decoder's blocks are), per size.
func BenchmarkIDCT(b *testing.B) {
	rng := rand.New(rand.NewSource(19))
	for log2 := 2; log2 <= 5; log2++ {
		n := 1 << uint(log2)
		for _, kind := range []struct {
			name string
			k    int
		}{{"dense", 1}, {"sparse", 0}} {
			d := randomCoefficients(rng, n, kind.k)
			maxX, maxY := -1, -1
			for y := 0; y < n; y++ {
				for x := 0; x < n; x++ {
					if d[y*n+x] != 0 {
						maxX, maxY = max(maxX, x), y
					}
				}
			}
			if maxX < 1 { // not the DC-only block, which never reaches the kernel
				d[n+1], maxX, maxY = 5, max(maxX, 1), max(maxY, 1)
			}
			dst, g := make([]int32, n*n), make([]int32, n*n)
			b.Run(fmt.Sprintf("%dx%d/%s/kernel", n, n, kind.name), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					_ = TryIDCT(dst, d, log2, 12)
				}
			})
			b.Run(fmt.Sprintf("%dx%d/%s/reference", n, n, kind.name), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					idctRef(dst, d, g, dctMatrix[:], 32*(32/n), n, maxX, maxY, 12)
				}
			})
		}
	}
}
