//go:build amd64 && !purego

package mc

import "github.com/go-videodsp/kernels/cpu"

// The kernels take the constants of the formula already derived, so that the
// assembly holds no branch on them: rnd is 2^(logWD-1) (0 for logWD 0) for the
// single-list form, shift the right shift, off the offset. For two lists
// wpair holds w0 in its low and w1 in its high sixteen bits (the PMADDWD
// operand), k is 2^logWD + (o << (logWD + 1)) with o = (o0 + o1 + 1) >> 1, and
// shift is logWD + 1: see internal/genweight86. A kernel handles two rows per
// iteration at widths 16 and 8 (four at width 4) and reads and writes exactly
// the bytes of its block.

var (
	weightUniImpl = weightUniRef
	weightBiImpl  = weightBiRef
)

func init() {
	if cpu.HasAVX2 {
		weightUniImpl = weightUniAVX2
		weightBiImpl = weightBiAVX2
	}
}

func weightUniDispatch(dst []byte, off, stride, w, h, weight, offset, logWD int) {
	weightUniImpl(dst, off, stride, w, h, weight, offset, logWD)
}

func weightBiDispatch(dst []byte, dstOff, dstStride int, src []byte, srcOff, srcStride, w, h, w0, w1, o0, o1, logWD int) {
	weightBiImpl(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, w0, w1, o0, o1, logWD)
}

func weightUniAVX2(dst []byte, off, stride, w, h, weight, offset, logWD int) {
	rnd := 0
	if logWD >= 1 {
		rnd = 1 << (logWD - 1)
	}
	switch w {
	case 4:
		weightUni4(&dst[off], stride, h, weight, rnd, logWD, offset)
	case 8:
		weightUni8(&dst[off], stride, h, weight, rnd, logWD, offset)
	default:
		weightUni16(&dst[off], stride, h, weight, rnd, logWD, offset)
	}
}

func weightBiAVX2(dst []byte, dstOff, dstStride int, src []byte, srcOff, srcStride, w, h, w0, w1, o0, o1, logWD int) {
	wpair := int(uint16(w0)) | w1<<16
	k := 1<<logWD + ((o0+o1+1)>>1)<<(logWD+1)
	switch w {
	case 4:
		weightBi4(&dst[dstOff], dstStride, &src[srcOff], srcStride, h, wpair, k, logWD+1)
	case 8:
		weightBi8(&dst[dstOff], dstStride, &src[srcOff], srcStride, h, wpair, k, logWD+1)
	default:
		weightBi16(&dst[dstOff], dstStride, &src[srcOff], srcStride, h, wpair, k, logWD+1)
	}
}
