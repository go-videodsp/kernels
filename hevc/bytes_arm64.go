//go:build arm64 && !purego

package hevc

// The dispatch of the byte-source kernels (8-bit pictures): the declarations are generated
// (hpassb_arm64.go, vpassu_arm64.go); the callers have proved the footprints, as for the
// uint16 forms.

func hpass8BDispatch(dst []int16, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	hpass8BNEON[shift](&dst[dstOff], dstStride, &src[srcOff-before], srcStride, w, h, &c[0])
}

func hpass4BDispatch(dst []int16, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	hpass4BNEON[shift](&dst[dstOff], dstStride, &src[srcOff-chromaBefore], srcStride, w, h, &c[0])
}

func vpassU8BDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	vpassU8BNEON[shift](&dst[dstOff], dstStride, &src[srcOff-before*srcStride], srcStride, w, h, &c[0])
}

func vpassU4BDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	vpassU4BNEON[shift](&dst[dstOff], dstStride, &src[srcOff-chromaBefore*srcStride], srcStride, w, h, &c[0])
}

func hpassW8BDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	hpassW8BNEON[shift](&dst[dstOff], dstStride, &src[srcOff-before], srcStride, w, h, &c[0])
}

func hpassW4BDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	hpassW4BNEON[shift](&dst[dstOff], dstStride, &src[srcOff-chromaBefore], srcStride, w, h, &c[0])
}

func widenBDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint) {
	widenBNEON(&dst[dstOff], dstStride, &src[srcOff], srcStride, w, h, int(shift))
}
