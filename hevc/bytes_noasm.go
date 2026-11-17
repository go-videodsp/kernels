//go:build (!arm64 && !amd64) || purego

package hevc

// The byte-source kernels have no assembly here: the portable reference is the path.

func hpass8BDispatch(dst []int16, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	hpassRef(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
}

func hpass4BDispatch(dst []int16, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	hpass4Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
}

func vpassU8BDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	vpassU8Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
}

func vpassU4BDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	vpassU4Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
}

func hpassW8BDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	hpassW8Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
}

func hpassW4BDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	hpassW4Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
}

func widenBDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint) {
	widenRef(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift)
}
