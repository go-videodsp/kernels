//go:build !arm64 || purego

package mc

func weightUniDispatch(dst []byte, off, stride, w, h, weight, offset, logWD int) {
	weightUniRef(dst, off, stride, w, h, weight, offset, logWD)
}

func weightBiDispatch(dst []byte, dstOff, dstStride int, src []byte, srcOff, srcStride, w, h, w0, w1, o0, o1, logWD int) {
	weightBiRef(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, w0, w1, o0, o1, logWD)
}
