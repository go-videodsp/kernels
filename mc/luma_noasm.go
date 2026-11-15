//go:build !arm64 || purego

package mc

// lumaDispatch is the portable path: every target without a kernel, and any
// build with the purego tag.
func lumaDispatch(dst []byte, dstOff, dstStride int, src []byte, srcOff, srcStride, w, h, xFrac, yFrac int, avg bool) {
	lumaRef(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, xFrac, yFrac, avg)
}
