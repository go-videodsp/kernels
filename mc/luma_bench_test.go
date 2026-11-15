package mc

import (
	"fmt"
	"testing"
)

// BenchmarkLuma times PutLuma (the NEON kernel on arm64, the portable code
// elsewhere, or with -tags purego) against the portable reference, for every
// block size and fractional position, put and average forms. Planes are
// small enough to stay in L1, so the table measures the kernel, not memory.
func BenchmarkLuma(b *testing.B) {
	const stride = 64
	src := make([]byte, stride*40)
	for i := range src {
		src[i] = byte(i*131 + i>>5)
	}
	dst := make([]byte, stride*24)
	for _, impl := range []string{"dispatch", "portable"} {
		for _, avg := range []bool{false, true} {
			op := "put"
			if avg {
				op = "avg"
			}
			for _, sz := range sizes {
				w, h := sz[0], sz[1]
				for fy := 0; fy < 4; fy++ {
					for fx := 0; fx < 4; fx++ {
						name := fmt.Sprintf("%s/%s/%dx%d/%c", impl, op, w, h, table812[fx][fy])
						b.Run(name, func(b *testing.B) {
							b.SetBytes(int64(w * h))
							const off = 4*stride + 4
							for i := 0; i < b.N; i++ {
								switch {
								case impl == "portable":
									lumaRef(dst, 0, stride, src, off, stride, w, h, fx, fy, avg)
								case avg:
									AvgLuma(dst, 0, stride, src, off, stride, w, h, fx, fy)
								default:
									PutLuma(dst, 0, stride, src, off, stride, w, h, fx, fy)
								}
							}
						})
					}
				}
			}
		}
	}
}
