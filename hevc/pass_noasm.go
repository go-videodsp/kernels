//go:build (!arm64 && !amd64) || purego

package hevc

// The portable path: every target without a kernel, and any build with the purego tag.

func hpassDispatch(dst []int16, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	hpassRef(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
}

func vpassDispatch(dst []int32, dstOff, dstStride int, src []int16, srcOff, srcStride, w, h int, c *[8]int16) {
	vpassRef(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, c)
}

func hpass4Dispatch(dst []int16, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	hpass4Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
}

func vpass4Dispatch(dst []int32, dstOff, dstStride int, src []int16, srcOff, srcStride, w, h int, c *[4]int16) {
	vpass4Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, c)
}
