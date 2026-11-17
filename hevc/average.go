package hevc

import (
	"unsafe"

	"github.com/go-videodsp/kernels/internal/bounds"
)

// The average of two blocks of samples, (a + b + 1) >> 1: the default weighted sample
// prediction (ITU-T H.265 (V11) (01/2026), clause 8.5.3.3.4.2, Equation 8-264) of two
// blocks at full-sample positions, where the intermediate samples are the reference
// samples shifted left, so the result is the average of the samples and needs no clipping.

func checkAverage[T uint8 | uint16](dst []T, dstOff, dstStride int, a []T, aOff, aStride int, b []T, bOff, bStride, w, h int) error {
	switch {
	case w < 2 || w > maxBlock || w%2 != 0 || h < 1 || h > maxBlock:
		return refuse("block %dx%d: the width is even from 2 to %d, the height from 1 to %d", w, h, maxBlock, maxBlock)
	case dstStride < w || aStride < w || bStride < w:
		return refuse("a stride (destination %d, a %d, b %d) is smaller than the block width %d", dstStride, aStride, bStride, w)
	}
	_, dLast, ok := bounds.Window(len(dst), dstOff, dstStride, 0, h-1, 0, w-1)
	if !ok {
		return refuse("destination block does not fit: offset %d stride %d size %dx%d length %d", dstOff, dstStride, w, h, len(dst))
	}
	for _, s := range [2]struct {
		p       []T
		off, st int
	}{{a, aOff, aStride}, {b, bOff, bStride}} {
		first, last, ok := bounds.Window(len(s.p), s.off, s.st, 0, h-1, 0, w-1)
		if !ok {
			return refuse("source block does not fit: offset %d stride %d size %dx%d length %d", s.off, s.st, w, h, len(s.p))
		}
		if overlaps(unsafe.Pointer(unsafe.SliceData(dst)), dstOff, dLast, int(unsafe.Sizeof(dst[0])), unsafe.Pointer(unsafe.SliceData(s.p)), first, last, int(unsafe.Sizeof(dst[0]))) {
			return refuse("destination block and source block share memory")
		}
	}
	return nil
}

// Average writes the w x h block dst[dstOff + y*dstStride + x] = (a[aOff + y*aStride + x] +
// b[bOff + y*bStride + x] + 1) >> 1. The width is even from 2 to 64, the height from 1 to 64,
// the samples are any 16-bit values; dst must not share memory with a or b (a and b may be
// the same). Average panics where TryAverage returns an error, before anything is written.
func Average(dst []uint16, dstOff, dstStride int, a []uint16, aOff, aStride int, b []uint16, bOff, bStride, w, h int) {
	if err := TryAverage(dst, dstOff, dstStride, a, aOff, aStride, b, bOff, bStride, w, h); err != nil {
		panic(err)
	}
}

// TryAverage is Average returning the refusal as an error, with nothing written.
func TryAverage(dst []uint16, dstOff, dstStride int, a []uint16, aOff, aStride int, b []uint16, bOff, bStride, w, h int) error {
	if err := checkAverage(dst, dstOff, dstStride, a, aOff, aStride, b, bOff, bStride, w, h); err != nil {
		return err
	}
	averageDispatch(dst, dstOff, dstStride, a, aOff, aStride, b, bOff, bStride, w, h)
	return nil
}
