//go:build amd64 && !purego

package hevc

// The dispatch of the byte-source kernels (8-bit pictures), as pass_amd64.go's for uint16:
// the portable reference where AVX2 is missing.

//go:noescape
func hpass8BAVX2(dst *int16, dstStride int, src *uint8, srcStride, w, h int, c *int16, shift int)

//go:noescape
func hpass4BAVX2(dst *int16, dstStride int, src *uint8, srcStride, w, h int, c *int16, shift int)

//go:noescape
func vpassU8BAVX2(dst *int32, dstStride int, src *uint8, srcStride, w, h int, c *int16, shift int)

//go:noescape
func vpassU4BAVX2(dst *int32, dstStride int, src *uint8, srcStride, w, h int, c *int16, shift int)

//go:noescape
func hpassW8BAVX2(dst *int32, dstStride int, src *uint8, srcStride, w, h int, c *int16, shift int)

//go:noescape
func hpassW4BAVX2(dst *int32, dstStride int, src *uint8, srcStride, w, h int, c *int16, shift int)

//go:noescape
func widenBAVX2(dst *int32, dstStride int, src *uint8, srcStride, w, h, shift int)

func hpass8BDispatch(dst []int16, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	if !useAVX2 {
		hpassRef(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
		return
	}
	hpass8BAVX2(&dst[dstOff], dstStride, &src[srcOff-before], srcStride, w, h, &c[0], int(shift))
}

func hpass4BDispatch(dst []int16, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	if !useAVX2 {
		hpass4Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
		return
	}
	hpass4BAVX2(&dst[dstOff], dstStride, &src[srcOff-chromaBefore], srcStride, w, h, &c[0], int(shift))
}

func vpassU8BDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	if !useAVX2 {
		vpassU8Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
		return
	}
	vpassU8BAVX2(&dst[dstOff], dstStride, &src[srcOff-before*srcStride], srcStride, w, h, &c[0], int(shift))
}

func vpassU4BDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	if !useAVX2 {
		vpassU4Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
		return
	}
	vpassU4BAVX2(&dst[dstOff], dstStride, &src[srcOff-chromaBefore*srcStride], srcStride, w, h, &c[0], int(shift))
}

func hpassW8BDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	if !useAVX2 {
		hpassW8Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
		return
	}
	hpassW8BAVX2(&dst[dstOff], dstStride, &src[srcOff-before], srcStride, w, h, &c[0], int(shift))
}

func hpassW4BDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	if !useAVX2 {
		hpassW4Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
		return
	}
	hpassW4BAVX2(&dst[dstOff], dstStride, &src[srcOff-chromaBefore], srcStride, w, h, &c[0], int(shift))
}

func widenBDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint) {
	if !useAVX2 {
		widenRef(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift)
		return
	}
	widenBAVX2(&dst[dstOff], dstStride, &src[srcOff], srcStride, w, h, int(shift))
}
