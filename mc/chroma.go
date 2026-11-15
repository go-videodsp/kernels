package mc

import (
	"errors"
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
	must(checkChromaOne(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, xFrac, yFrac))
	putChromaDispatch(dst, dst, dstOff, dstStride, src, src, srcOff, srcStride, w, h, xFrac, yFrac, 1, false)
}

// TryPutChroma is PutChroma returning the refusal as an error, with nothing
// written, instead of panicking: for a caller that has its own path for a block
// the kernels do not take and must not crash on one it did not foresee.
func TryPutChroma(dst []byte, dstOff, dstStride int, src []byte, srcOff, srcStride, w, h, xFrac, yFrac int) error {
	if err := checkChromaOne(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, xFrac, yFrac); err != nil {
		return err
	}
	putChromaDispatch(dst, dst, dstOff, dstStride, src, src, srcOff, srcStride, w, h, xFrac, yFrac, 1, false)
	return nil
}

// AvgChroma is PutChroma averaged into dst, (dst + p + 1) >> 1 (Equation 8-273):
// the second prediction of a bi-predicted block, fused into the interpolation.
// Its checks and panics are PutChroma's.
func AvgChroma(dst []byte, dstOff, dstStride int, src []byte, srcOff, srcStride, w, h, xFrac, yFrac int) {
	must(checkChromaOne(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, xFrac, yFrac))
	putChromaDispatch(dst, dst, dstOff, dstStride, src, src, srcOff, srcStride, w, h, xFrac, yFrac, 1, true)
}

// TryAvgChroma is AvgChroma returning the refusal as an error, as TryPutChroma.
func TryAvgChroma(dst []byte, dstOff, dstStride int, src []byte, srcOff, srcStride, w, h, xFrac, yFrac int) error {
	if err := checkChromaOne(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, xFrac, yFrac); err != nil {
		return err
	}
	putChromaDispatch(dst, dst, dstOff, dstStride, src, src, srcOff, srcStride, w, h, xFrac, yFrac, 1, true)
	return nil
}

// PutChromaPair is PutChroma for the two chroma planes of a prediction, which
// share the position, the strides, the size and the fractions, in one call.
func PutChromaPair(dstCb, dstCr []byte, dstOff, dstStride int, srcCb, srcCr []byte, srcOff, srcStride, w, h, xFrac, yFrac int) {
	must(checkChromaPair(dstCb, dstCr, dstOff, dstStride, srcCb, srcCr, srcOff, srcStride, w, h, xFrac, yFrac))
	putChromaDispatch(dstCb, dstCr, dstOff, dstStride, srcCb, srcCr, srcOff, srcStride, w, h, xFrac, yFrac, 2, false)
}

// TryPutChromaPair is PutChromaPair returning the refusal as an error, with
// nothing written, instead of panicking.
func TryPutChromaPair(dstCb, dstCr []byte, dstOff, dstStride int, srcCb, srcCr []byte, srcOff, srcStride, w, h, xFrac, yFrac int) error {
	if err := checkChromaPair(dstCb, dstCr, dstOff, dstStride, srcCb, srcCr, srcOff, srcStride, w, h, xFrac, yFrac); err != nil {
		return err
	}
	putChromaDispatch(dstCb, dstCr, dstOff, dstStride, srcCb, srcCr, srcOff, srcStride, w, h, xFrac, yFrac, 2, false)
	return nil
}

// AvgChromaPair is PutChromaPair averaged into the destinations, (dst + p + 1)
// >> 1 (Equation 8-273). Its checks and panics are PutChromaPair's.
func AvgChromaPair(dstCb, dstCr []byte, dstOff, dstStride int, srcCb, srcCr []byte, srcOff, srcStride, w, h, xFrac, yFrac int) {
	must(checkChromaPair(dstCb, dstCr, dstOff, dstStride, srcCb, srcCr, srcOff, srcStride, w, h, xFrac, yFrac))
	putChromaDispatch(dstCb, dstCr, dstOff, dstStride, srcCb, srcCr, srcOff, srcStride, w, h, xFrac, yFrac, 2, true)
}

// TryAvgChromaPair is AvgChromaPair returning the refusal as an error, as
// TryPutChromaPair.
func TryAvgChromaPair(dstCb, dstCr []byte, dstOff, dstStride int, srcCb, srcCr []byte, srcOff, srcStride, w, h, xFrac, yFrac int) error {
	if err := checkChromaPair(dstCb, dstCr, dstOff, dstStride, srcCb, srcCr, srcOff, srcStride, w, h, xFrac, yFrac); err != nil {
		return err
	}
	putChromaDispatch(dstCb, dstCr, dstOff, dstStride, srcCb, srcCr, srcOff, srcStride, w, h, xFrac, yFrac, 2, true)
	return nil
}

// must turns a refusal into the panic of the plain entry points.
func must(err error) {
	if err != nil {
		panic(err.Error())
	}
}

// checkChromaOne proves one plane's two footprints, the fractions and that the
// blocks share no memory.
func checkChromaOne(dst []byte, dstOff, dstStride int, src []byte, srcOff, srcStride, w, h, xFrac, yFrac int) error {
	dLast, err := checkChroma("destination", len(dst), dstOff, dstStride, w, 0, h)
	if err != nil {
		return err
	}
	sLast, err := checkChroma("source", len(src), srcOff, srcStride, w, 1, h)
	if err != nil {
		return err
	}
	if err := checkFrac(xFrac, yFrac); err != nil {
		return err
	}
	if overlaps(dst, dstOff, dLast, src, srcOff, sLast) {
		return errors.New("mc: destination and source blocks share memory")
	}
	return nil
}

// checkChromaPair proves the four footprints and that no destination shares
// memory with another block. The two planes of a prediction usually have equal
// lengths, in which case one proof stands for both.
func checkChromaPair(dstCb, dstCr []byte, dstOff, dstStride int, srcCb, srcCr []byte, srcOff, srcStride, w, h, xFrac, yFrac int) error {
	d0, err := checkChroma("destination Cb", len(dstCb), dstOff, dstStride, w, 0, h)
	if err != nil {
		return err
	}
	d1 := d0
	if len(dstCr) != len(dstCb) {
		if d1, err = checkChroma("destination Cr", len(dstCr), dstOff, dstStride, w, 0, h); err != nil {
			return err
		}
	}
	s0, err := checkChroma("source Cb", len(srcCb), srcOff, srcStride, w, 1, h)
	if err != nil {
		return err
	}
	s1 := s0
	if len(srcCr) != len(srcCb) {
		if s1, err = checkChroma("source Cr", len(srcCr), srcOff, srcStride, w, 1, h); err != nil {
			return err
		}
	}
	if err := checkFrac(xFrac, yFrac); err != nil {
		return err
	}
	if overlaps(dstCb, dstOff, d0, dstCr, dstOff, d1) ||
		overlaps(dstCb, dstOff, d0, srcCb, srcOff, s0) || overlaps(dstCb, dstOff, d0, srcCr, srcOff, s1) ||
		overlaps(dstCr, dstOff, d1, srcCb, srcOff, s0) || overlaps(dstCr, dstOff, d1, srcCr, srcOff, s1) {
		return errors.New("mc: a destination block shares memory with another block")
	}
	return nil
}

func checkFrac(xFrac, yFrac int) error {
	if xFrac < 0 || xFrac > 7 || yFrac < 0 || yFrac > 7 {
		return fmt.Errorf("mc: chroma fractions %d, %d outside 0..7", xFrac, yFrac)
	}
	return nil
}

// checkChroma proves that a block of w + extra columns and h + extra rows (extra
// is 0 for the destination, 1 for the source) at off fits in a slice of n bytes
// (internal/bounds.Window: no product can wrap on any target) and returns the
// index of the last sample of the w x h block itself.
func checkChroma(what string, n, off, stride, w, extra, h int) (int, error) {
	if !validChromaSize(w, h) {
		return 0, fmt.Errorf("mc: unsupported chroma block size %dx%d", w, h)
	}
	if stride < w+extra {
		return 0, fmt.Errorf("mc: %s stride %d is smaller than the block width %d", what, stride, w+extra)
	}
	_, last, ok := bounds.Window(n, off, stride, 0, h-1+extra, 0, w-1+extra)
	if !ok {
		return 0, fmt.Errorf("mc: %s block does not fit: offset %d stride %d size %dx%d length %d", what, off, stride, w, h, n)
	}
	return last, nil
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

// avgChromaRef is Equation 8-273 applied to the prediction of 8-270.
func avgChromaRef(dst []byte, dstOff, dstStride int, src []byte, srcOff, srcStride, w, h, xFrac, yFrac int) {
	var p [64]byte
	putChromaRef(p[:], 0, w, src, srcOff, srcStride, w, h, xFrac, yFrac)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := dstOff + y*dstStride + x
			dst[i] = byte((int(dst[i]) + int(p[y*w+x]) + 1) >> 1)
		}
	}
}

func putChromaPairRef(dst0, dst1 []byte, dstOff, dstStride int, src0, src1 []byte, srcOff, srcStride, w, h, xFrac, yFrac, planes int, avg bool) {
	f := putChromaRef
	if avg {
		f = avgChromaRef
	}
	f(dst0, dstOff, dstStride, src0, srcOff, srcStride, w, h, xFrac, yFrac)
	if planes == 2 {
		f(dst1, dstOff, dstStride, src1, srcOff, srcStride, w, h, xFrac, yFrac)
	}
}
