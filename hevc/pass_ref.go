package hevc

// hpassRef and vpassRef are the portable reference: the equations of the text,
// one sample at a time. They are the oracle of the kernels and the path of every
// target without one.

func hpassRef(dst []int16, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
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

func hpass4Ref(dst []int16, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
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
