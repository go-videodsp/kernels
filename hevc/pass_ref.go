package hevc

// hpassRef and vpassRef are the portable reference: the equations of the text,
// one sample at a time. They are the oracle of the kernels and the path of every
// target without one.

func hpassRef[S uint8 | uint16](dst []int16, dstOff, dstStride int, src []S, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var s int32
			for k := 0; k < taps; k++ {
				s += int32(c[k]) * int32(src[srcOff+y*srcStride+x+k-before])
			}
			dst[dstOff+y*dstStride+x] = int16(s >> shift)
		}
	}
}

func vpassRef(dst []int32, dstOff, dstStride int, src []int16, srcOff, srcStride, w, h int, c *[8]int16) {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var s int32
			for k := 0; k < taps; k++ {
				s += int32(c[k]) * int32(src[srcOff+(y+k-before)*srcStride+x])
			}
			dst[dstOff+y*dstStride+x] = s >> 6
		}
	}
}

func hpass4Ref[S uint8 | uint16](dst []int16, dstOff, dstStride int, src []S, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var s int32
			for k := 0; k < chromaTaps; k++ {
				s += int32(c[k]) * int32(src[srcOff+y*srcStride+x+k-chromaBefore])
			}
			dst[dstOff+y*dstStride+x] = int16(s >> shift)
		}
	}
}

func vpass4Ref(dst []int32, dstOff, dstStride int, src []int16, srcOff, srcStride, w, h int, c *[4]int16) {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var s int32
			for k := 0; k < chromaTaps; k++ {
				s += int32(c[k]) * int32(src[srcOff+(y+k-chromaBefore)*srcStride+x])
			}
			dst[dstOff+y*dstStride+x] = s >> 6
		}
	}
}

func vpassU8Ref[S uint8 | uint16](dst []int32, dstOff, dstStride int, src []S, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var s int32
			for k := 0; k < taps; k++ {
				s += int32(c[k]) * int32(src[srcOff+(y+k-before)*srcStride+x])
			}
			dst[dstOff+y*dstStride+x] = s >> shift
		}
	}
}

func vpassU4Ref[S uint8 | uint16](dst []int32, dstOff, dstStride int, src []S, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var s int32
			for k := 0; k < chromaTaps; k++ {
				s += int32(c[k]) * int32(src[srcOff+(y+k-chromaBefore)*srcStride+x])
			}
			dst[dstOff+y*dstStride+x] = s >> shift
		}
	}
}

func hpassW8Ref[S uint8 | uint16](dst []int32, dstOff, dstStride int, src []S, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var s int32
			for k := 0; k < taps; k++ {
				s += int32(c[k]) * int32(src[srcOff+y*srcStride+x+k-before])
			}
			dst[dstOff+y*dstStride+x] = s >> shift
		}
	}
}

func hpassW4Ref[S uint8 | uint16](dst []int32, dstOff, dstStride int, src []S, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var s int32
			for k := 0; k < chromaTaps; k++ {
				s += int32(c[k]) * int32(src[srcOff+y*srcStride+x+k-chromaBefore])
			}
			dst[dstOff+y*dstStride+x] = s >> shift
		}
	}
}

func widenRef[S uint8 | uint16](dst []int32, dstOff, dstStride int, src []S, srcOff, srcStride, w, h int, shift uint) {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst[dstOff+y*dstStride+x] = int32(src[srcOff+y*srcStride+x]) << shift
		}
	}
}

func clip16(v, hi int) int { return min(max(v, 0), hi) }

// saoBandRef is the band offset of the text, one sample at a time.
func saoBandRef(dst []uint16, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h, bitDepth, bandPos int, off *[4]int16) {
	hi := 1<<uint(bitDepth) - 1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			cur := int(src[srcOff+y*srcStride+x])
			band, d := cur>>uint(bitDepth-5), 0
			for k := 0; k < 4; k++ {
				if band == (bandPos+k)&31 {
					d = int(off[k])
				}
			}
			dst[dstOff+y*dstStride+x] = uint16(clip16(cur+d, hi))
		}
	}
}

func sign16(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// saoEdgeRef is the edge offset of the text: offA and offB are the element offsets of the
// two neighbours.
func saoEdgeRef(dst []uint16, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h, bitDepth, offA, offB int, off *[4]int16) {
	hi := 1<<uint(bitDepth) - 1
	delta := [5]int{int(off[0]), int(off[1]), 0, int(off[2]), int(off[3])}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := srcOff + y*srcStride + x
			cur := int(src[i])
			e := sign16(cur-int(src[i+offA])) + sign16(cur-int(src[i+offB]))
			dst[dstOff+y*dstStride+x] = uint16(clip16(cur+delta[e+2], hi))
		}
	}
}

// idctRef is the two stages of the inverse transform in 32 bits: mat[j*matStride + i] is the
// basis function of frequency j at position i; g is scratch of n*n entries.
func idctRef(dst, d, g, mat []int32, matStride, n, maxX, maxY int, shift uint) {
	for x := 0; x <= maxX; x++ {
		for i := 0; i < n; i++ {
			var s int32
			for j := 0; j <= maxY; j++ {
				s += mat[j*matStride+i] * d[j*n+x]
			}
			g[i*n+x] = min(max((s+64)>>7, minCoeff), maxCoeff)
		}
	}
	for y := 0; y < n; y++ {
		for i := 0; i < n; i++ {
			var s int32
			for k := 0; k <= maxX; k++ {
				s += mat[k*matStride+i] * g[y*n+k]
			}
			dst[y*n+i] = (s + 1<<(shift-1)) >> shift
		}
	}
}

// averageRef is (a + b + 1) >> 1 of the text.
func averageRef(dst []uint16, dstOff, dstStride int, a []uint16, aOff, aStride int, b []uint16, bOff, bStride, w, h int) {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst[dstOff+y*dstStride+x] = uint16((uint32(a[aOff+y*aStride+x]) + uint32(b[bOff+y*bStride+x]) + 1) >> 1)
		}
	}
}

// weightUniRef and weightBiRef are the weighted prediction of the text in 32-bit arithmetic.
func weightUniRef(dst []uint16, dstOff, dstStride int, p []int32, pOff, pStride, w, h int, log2Wd uint, q *[4]int32) {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := ((p[pOff+y*pStride+x]*q[0] + q[1]) >> log2Wd) + q[2]
			dst[dstOff+y*dstStride+x] = uint16(min(max(v, 0), q[3]))
		}
	}
}

func weightBiRef(dst []uint16, dstOff, dstStride int, p0, p1 []int32, pOff, pStride, w, h int, log2Wd uint, q *[4]int32) {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := pOff + y*pStride + x
			v := (p0[i]*q[0] + p1[i]*q[1] + q[2]) >> (log2Wd + 1)
			dst[dstOff+y*dstStride+x] = uint16(min(max(v, 0), q[3]))
		}
	}
}
