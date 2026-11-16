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
