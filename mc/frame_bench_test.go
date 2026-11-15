package mc

import (
	"fmt"
	"testing"
)

// The frame-sweep benchmarks run a kernel over every 16x16 macroblock of a
// 1920x1088 picture, reading a reference plane and writing another one, so
// that the working set (about 6 MB) and the call overhead are those of a
// decoder. The L1-resident benchmarks above measure the kernel alone; these
// are the figures to put next to a decoder's per-macroblock time (see
// docs/bench/ffmpeg-stages, which times FFmpeg on the same shape). Each
// iteration is one picture; the reported ns/MB is per macroblock.
const (
	frameWMB    = 120
	frameHMB    = 68
	frameStride = frameWMB*16 + 16 // a margin beside the picture for the filter taps
)

func framePlane(rows int) []byte {
	p := make([]byte, frameStride*rows)
	seed := uint32(7)
	for i := range p {
		seed = seed*1664525 + 1013904223
		p[i] = byte(seed >> 24)
	}
	return p
}

// BenchmarkLumaFrame is PutLuma and AvgLuma for one position over a picture.
func BenchmarkLumaFrame(b *testing.B) {
	src := framePlane(frameHMB*16 + 8)
	dst := framePlane(frameHMB * 16)
	for _, avg := range []bool{false, true} {
		op := map[bool]string{false: "put", true: "avg"}[avg]
		for fy := 0; fy < 4; fy++ {
			for fx := 0; fx < 4; fx++ {
				b.Run(fmt.Sprintf("%s/%c", op, table812[fx][fy]), func(b *testing.B) {
					f := PutLuma
					if avg {
						f = AvgLuma
					}
					for i := 0; i < b.N; i++ {
						for my := 0; my < frameHMB; my++ {
							for mx := 0; mx < frameWMB; mx++ {
								f(dst, my*16*frameStride+mx*16, frameStride, src, (my*16+4)*frameStride+mx*16+4, frameStride, 16, 16, fx, fy)
							}
						}
					}
					b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*frameWMB*frameHMB), "ns/MB")
				})
			}
		}
	}
}

// BenchmarkWeightFrame is WeightUni (the explicit single-list weights of the
// FFmpeg case: denominator 5, weight 40, offset 3) and WeightBi over a
// picture.
func BenchmarkWeightFrame(b *testing.B) {
	dst := framePlane(frameHMB * 16)
	src := framePlane(frameHMB * 16)
	b.Run("uni", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for my := 0; my < frameHMB; my++ {
				for mx := 0; mx < frameWMB; mx++ {
					WeightUni(dst, my*16*frameStride+mx*16, frameStride, 16, 16, 40, 3, 5)
				}
			}
		}
		b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*frameWMB*frameHMB), "ns/MB")
	})
	b.Run("bi", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for my := 0; my < frameHMB; my++ {
				for mx := 0; mx < frameWMB; mx++ {
					o := my*16*frameStride + mx*16
					WeightBi(dst, o, frameStride, src, o, frameStride, 16, 16, 24, 40, 1, 2, 5)
				}
			}
		}
		b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*frameWMB*frameHMB), "ns/MB")
	})
}
