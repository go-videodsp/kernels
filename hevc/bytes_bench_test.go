package hevc

import (
	"fmt"
	"math/rand"
	"testing"
)

// Benchmarks of the byte-source kernels against the same filter over uint16 samples (the
// kernel an 8-bit picture used before, after widening its window) and against the portable
// reference, per block size, luma and chroma. The widening the byte kernels remove is the
// "widen+uint16" row: a Widen-style copy of the footprint, then the uint16 kernel.
func BenchmarkBytes(b *testing.B) {
	rng := rand.New(rand.NewSource(22))
	for _, n := range []int{8, 16, 32, 64} {
		sS := n + 8
		src8 := randomBytes(rng, (n+8)*sS, false)
		src16 := make([]uint16, len(src8))
		for i, v := range src8 {
			src16[i] = uint16(v)
		}
		d16, d32 := make([]int16, n*n), make([]int32, n*n)
		cl, cc := &lumaFilters[1], &chromaFilters[3]
		offH, offV := 3, 3*sS
		b.Run(fmt.Sprintf("HPass8/%dx%d/bytes", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryHPass8Bytes(d16, 0, n, src8, offH, sS, n, n, cl)
			}
		})
		b.Run(fmt.Sprintf("HPass8/%dx%d/uint16", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryHPass8(d16, 0, n, src16, offH, sS, n, n, 8, cl)
			}
		})
		b.Run(fmt.Sprintf("HPass8/%dx%d/reference", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				hpassRef(d16, 0, n, src8, offH, sS, n, n, 0, cl)
			}
		})
		b.Run(fmt.Sprintf("HPass4/%dx%d/bytes", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryHPass4Bytes(d16, 0, n, src8, 1, sS, n, n, cc)
			}
		})
		b.Run(fmt.Sprintf("HPass4/%dx%d/uint16", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryHPass4(d16, 0, n, src16, 1, sS, n, n, 8, cc)
			}
		})
		b.Run(fmt.Sprintf("VPassU8/%dx%d/bytes", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryVPassU8Bytes(d32, 0, n, src8, offV, sS, n, n, cl)
			}
		})
		b.Run(fmt.Sprintf("VPassU8/%dx%d/uint16", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryVPassU8(d32, 0, n, src16, offV, sS, n, n, 8, cl)
			}
		})
		b.Run(fmt.Sprintf("VPassU4/%dx%d/bytes", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryVPassU4Bytes(d32, 0, n, src8, sS, sS, n, n, cc)
			}
		})
		b.Run(fmt.Sprintf("VPassU4/%dx%d/uint16", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryVPassU4(d32, 0, n, src16, sS, sS, n, n, 8, cc)
			}
		})
		b.Run(fmt.Sprintf("HPassW8/%dx%d/bytes", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryHPassW8Bytes(d32, 0, n, src8, offH, sS, n, n, cl)
			}
		})
		b.Run(fmt.Sprintf("HPassW8/%dx%d/uint16", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryHPassW8(d32, 0, n, src16, offH, sS, n, n, 8, cl)
			}
		})
		b.Run(fmt.Sprintf("HPassW4/%dx%d/bytes", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryHPassW4Bytes(d32, 0, n, src8, 1, sS, n, n, cc)
			}
		})
		b.Run(fmt.Sprintf("HPassW4/%dx%d/uint16", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryHPassW4(d32, 0, n, src16, 1, sS, n, n, 8, cc)
			}
		})
		b.Run(fmt.Sprintf("Widen/%dx%d/bytes", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryWidenBytes(d32, 0, n, src8, 0, sS, n, n, 6)
			}
		})
		b.Run(fmt.Sprintf("Widen/%dx%d/uint16", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = TryWiden(d32, 0, n, src16, 0, sS, n, n, 6)
			}
		})
		b.Run(fmt.Sprintf("Widen/%dx%d/reference", n, n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				widenRef(d32, 0, n, src8, 0, sS, n, n, 6)
			}
		})
	}
}
