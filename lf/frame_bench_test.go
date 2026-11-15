package lf

import "testing"

// BenchmarkDeblockFrame filters every edge of every macroblock of a 1920x1088
// 4:2:0 picture the way an all-intra picture does: bS 4 on the macroblock
// edges, bS 3 inside, the four vertical and the four horizontal luma edges and
// the two plus two chroma edges of each chroma plane per macroblock. The plane
// is a smooth ramp, so every line passes the thresholds of 8.7.2.2 and runs the
// whole formula (the worst case for a kernel). One iteration is one picture;
// the reported ns/MB is per macroblock, to put next to a decoder's loop-filter
// time per macroblock (docs/bench/ffmpeg-stages).
func BenchmarkDeblockFrame(b *testing.B) {
	const wMB, hMB = 120, 68
	ramp := func(stride, rows int) []byte {
		p := make([]byte, stride*rows)
		for y := 0; y < rows; y++ {
			for x := 0; x < stride; x++ {
				p[y*stride+x] = byte(100 + (x+y)>>3&3)
			}
		}
		return p
	}
	const indexA, indexB = 34, 34
	bS := [4]uint8{3, 3, 3, 3}
	luma := ramp(wMB*16, hMB*16)
	chroma := [2][]byte{ramp(wMB*8, hMB*8), ramp(wMB*8, hMB*8)}
	run := func(b *testing.B, doLuma, doChroma bool) {
		for i := 0; i < b.N; i++ {
			for my := 1; my < hMB; my++ {
				for mx := 1; mx < wMB; mx++ {
					if doLuma {
						ls := wMB * 16
						o := my*16*ls + mx*16
						DeblockLumaIntraV(luma, o, ls, indexA, indexB)
						DeblockLumaIntraH(luma, o, ls, indexA, indexB)
						for e := 4; e < 16; e += 4 {
							DeblockLumaV(luma, o+e, ls, indexA, indexB, bS)
							DeblockLumaH(luma, o+e*ls, ls, indexA, indexB, bS)
						}
					}
					if doChroma {
						cs := wMB * 8
						for _, p := range chroma {
							o := my*8*cs + mx*8
							DeblockChromaIntraV(p, o, cs, indexA, indexB)
							DeblockChromaIntraH(p, o, cs, indexA, indexB)
							DeblockChromaV(p, o+4, cs, indexA, indexB, bS)
							DeblockChromaH(p, o+4*cs, cs, indexA, indexB, bS)
						}
					}
				}
			}
		}
		b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*(wMB-1)*(hMB-1)), "ns/MB")
	}
	b.Run("luma", func(b *testing.B) { run(b, true, false) })
	b.Run("chroma", func(b *testing.B) { run(b, false, true) })
	b.Run("luma+chroma", func(b *testing.B) { run(b, true, true) })
}
