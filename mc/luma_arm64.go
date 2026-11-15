//go:build arm64 && !purego

package mc

// lumaDispatch runs the NEON kernel for the position. The caller has proved
// the footprint (checkLuma), so the two pointers and the loops inside the
// assembly stay within dst and src.
func lumaDispatch(dst []byte, dstOff, dstStride int, src []byte, srcOff, srcStride, w, h, xFrac, yFrac int, avg bool) {
	a, wide := 0, 0
	if avg {
		a = 1
	}
	if w != 4 {
		wide = 1
	}
	lumaKernels[a][wide][xFrac+4*yFrac](&dst[dstOff], dstStride, &src[srcOff], srcStride, w, h)
}
