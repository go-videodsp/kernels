//go:build arm64 && !purego

package mc

// The kernels take the constants of the formula already derived, so that the
// assembly holds no branch on them: rnd is 2^(logWD-1) (0 for logWD 0) for
// the single-list form and 2^logWD for the two-list one, shift is minus the
// right shift (the SSHL count), off the offset ((o0+o1+1)>>1 for two lists).
// The width-4 forms read and write 4 bytes per row, the others strips of 8.

//go:noescape
func weightUni4(dst *byte, stride, w, h, weight, rnd, shift, off int)

//go:noescape
func weightUni8(dst *byte, stride, w, h, weight, rnd, shift, off int)

//go:noescape
func weightBi4(dst *byte, dstStride int, src *byte, srcStride, w, h, w0, w1, rnd, shift, off int)

//go:noescape
func weightBi8(dst *byte, dstStride int, src *byte, srcStride, w, h, w0, w1, rnd, shift, off int)

func weightUniDispatch(dst []byte, off, stride, w, h, weight, offset, logWD int) {
	rnd := 0
	if logWD >= 1 {
		rnd = 1 << (logWD - 1)
	}
	f := weightUni8
	if w == 4 {
		f = weightUni4
	}
	f(&dst[off], stride, w, h, weight, rnd, -logWD, offset)
}

func weightBiDispatch(dst []byte, dstOff, dstStride int, src []byte, srcOff, srcStride, w, h, w0, w1, o0, o1, logWD int) {
	f := weightBi8
	if w == 4 {
		f = weightBi4
	}
	f(&dst[dstOff], dstStride, &src[srcOff], srcStride, w, h, w0, w1, 1<<logWD, -(logWD + 1), (o0+o1+1)>>1)
}
