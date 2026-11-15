package mc

import (
	"fmt"

	"github.com/go-videodsp/kernels/internal/bounds"
)

//go:generate go run ../internal/genchroma ..

func validChromaSize(w, h int) bool {
	return (w == 8 || w == 4 || w == 2) && (h == 8 || h == 4 || h == 2)
}

// PutChroma predicts one w x h chroma block (ITU-T H.264 (V16) (06/2026),
// clause 8.4.2.2.2, Equation 8-270): with A, B, C, D the samples at
// src[srcOff + y*srcStride + x] and its right, lower and lower-right
// neighbours, and xFrac, yFrac the eighth-sample fractions (0..7),
//
//	dst[dstOff + y*dstStride + x] =
//	  ((8-xFrac)*(8-yFrac)*A + xFrac*(8-yFrac)*B + (8-xFrac)*yFrac*C + xFrac*yFrac*D + 32) >> 6
//
// w and h are each 2, 4 or 8. The source footprint is (w+1) x (h+1) samples
// even when a fraction is zero (the term of the missing neighbour is zero, and
// the kernels read it all the same), and the caller proves it lies in src, as
// it does for the interpolation of 8.4.2.2.2, which reads the neighbours when
// a fraction is not zero and where the picture border stands in for them
// otherwise.
//
// dst and src must not share memory, and the two planes of PutChromaPair must
// not share either: the functions panic when the byte range of a destination
// block (first to last sample) intersects the source's or each other's, or
// when a footprint does not fit its slice, stride < w (dst) or w+1 (src), a
// size or a fraction is outside the supported set. Nothing is written before
// the checks pass.
func PutChroma(dst []byte, dstOff, dstStride int, src []byte, srcOff, srcStride, w, h, xFrac, yFrac int) {
	dLast := checkChroma("destination", len(dst), dstOff, dstStride, w, 0, h)
	sLast := checkChroma("source", len(src), srcOff, srcStride, w, 1, h)
	checkFrac(xFrac, yFrac)
	if overlaps(dst, dstOff, dLast, src, srcOff, sLast) {
		panic("mc: destination and source blocks share memory")
	}
	putChromaDispatch(dst, dst, dstOff, dstStride, src, src, srcOff, srcStride, w, h, xFrac, yFrac, 1)
}

// PutChromaPair is PutChroma for the two chroma planes of a prediction, which
// share the position, the strides, the size and the fractions, in one call.
func PutChromaPair(dstCb, dstCr []byte, dstOff, dstStride int, srcCb, srcCr []byte, srcOff, srcStride, w, h, xFrac, yFrac int) {
	d0 := checkChroma("destination Cb", len(dstCb), dstOff, dstStride, w, 0, h)
	d1 := checkChroma("destination Cr", len(dstCr), dstOff, dstStride, w, 0, h)
	s0 := checkChroma("source Cb", len(srcCb), srcOff, srcStride, w, 1, h)
	s1 := checkChroma("source Cr", len(srcCr), srcOff, srcStride, w, 1, h)
	checkFrac(xFrac, yFrac)
	if overlaps(dstCb, dstOff, d0, dstCr, dstOff, d1) ||
		overlaps(dstCb, dstOff, d0, srcCb, srcOff, s0) || overlaps(dstCb, dstOff, d0, srcCr, srcOff, s1) ||
		overlaps(dstCr, dstOff, d1, srcCb, srcOff, s0) || overlaps(dstCr, dstOff, d1, srcCr, srcOff, s1) {
		panic("mc: a destination block shares memory with another block")
	}
	putChromaDispatch(dstCb, dstCr, dstOff, dstStride, srcCb, srcCr, srcOff, srcStride, w, h, xFrac, yFrac, 2)
}

func checkFrac(xFrac, yFrac int) {
	if xFrac < 0 || xFrac > 7 || yFrac < 0 || yFrac > 7 {
		panic(fmt.Sprintf("mc: chroma fractions %d, %d outside 0..7", xFrac, yFrac))
	}
}

// checkChroma proves that a block of w + extra columns and h + extra rows (extra
// is 0 for the destination, 1 for the source) at off fits in a slice of n bytes
// (internal/bounds.Window: no product can wrap on any target) and returns the
// index of the last sample of the w x h block itself.
func checkChroma(what string, n, off, stride, w, extra, h int) int {
	if !validChromaSize(w, h) {
		panic(fmt.Sprintf("mc: unsupported chroma block size %dx%d", w, h))
	}
	if stride < w+extra {
		panic(fmt.Sprintf("mc: %s stride %d is smaller than the block width %d", what, stride, w+extra))
	}
	_, last, ok := bounds.Window(n, off, stride, 0, h-1+extra, 0, w-1+extra)
	if !ok {
		panic(fmt.Sprintf("mc: %s block does not fit: offset %d stride %d size %dx%d length %d", what, off, stride, w, h, n))
	}
	return last
}

// putChromaRef is the portable form, transcribed from Equation 8-270.
func putChromaRef(dst []byte, dstOff, dstStride int, src []byte, srcOff, srcStride, w, h, xFrac, yFrac int) {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p := srcOff + y*srcStride + x
			a, b, c, d := int(src[p]), int(src[p+1]), int(src[p+srcStride]), int(src[p+srcStride+1])
			dst[dstOff+y*dstStride+x] = byte(((8-xFrac)*(8-yFrac)*a + xFrac*(8-yFrac)*b + (8-xFrac)*yFrac*c + xFrac*yFrac*d + 32) >> 6)
		}
	}
}

func putChromaPairRef(dst0, dst1 []byte, dstOff, dstStride int, src0, src1 []byte, srcOff, srcStride, w, h, xFrac, yFrac, planes int) {
	putChromaRef(dst0, dstOff, dstStride, src0, srcOff, srcStride, w, h, xFrac, yFrac)
	if planes == 2 {
		putChromaRef(dst1, dstOff, dstStride, src1, srcOff, srcStride, w, h, xFrac, yFrac)
	}
}
