package hevc

import (
	"unsafe"

	"github.com/go-videodsp/kernels/internal/bounds"
)

// Weighted sample prediction (ITU-T H.265 (V11) (01/2026), clause 8.5.3.3.4): from the 32-bit
// intermediates of the interpolation to the samples of the picture. The default process
// (8.5.3.3.4.2) is the explicit one with the weights 1, the offsets 0 and
// log2Wd = Max( 2, 14 - bitDepth ), so these two functions are both.

const (
	maxWeight = 512
	maxWOff   = 1 << 16
	minLog2Wd = 2
	maxLog2Wd = 13
)

func checkWeight[T uint8 | uint16](dst []T, dstOff, dstStride int, ps [][]int32, pOff, pStride, w, h, bitDepth, log2Wd int, scalars ...int32) error {
	switch {
	case w < 2 || w > maxBlock || w%2 != 0 || h < 1 || h > maxBlock:
		return refuse("block %dx%d: the width is even from 2 to %d, the height from 1 to %d", w, h, maxBlock, maxBlock)
	case bitDepth < 8 || bitDepth > 12:
		return refuse("bit depth %d is not from 8 to 12", bitDepth)
	case log2Wd < minLog2Wd || log2Wd > maxLog2Wd:
		return refuse("log2Wd %d is not from %d to %d", log2Wd, minLog2Wd, maxLog2Wd)
	case dstStride < w:
		return refuse("destination stride %d is smaller than the block width %d", dstStride, w)
	case pStride < w:
		return refuse("source stride %d is smaller than the block width %d", pStride, w)
	}
	for i, s := range scalars {
		lim := int32(maxWeight)
		if i >= len(scalars)/2 {
			lim = maxWOff
		}
		if s < -lim || s > lim {
			return refuse("weight or offset %d is beyond %d", s, lim)
		}
	}
	_, dLast, ok := bounds.Window(len(dst), dstOff, dstStride, 0, h-1, 0, w-1)
	if !ok {
		return refuse("destination block does not fit: offset %d stride %d size %dx%d length %d", dstOff, dstStride, w, h, len(dst))
	}
	for _, p := range ps {
		first, last, ok := bounds.Window(len(p), pOff, pStride, 0, h-1, 0, w-1)
		if !ok {
			return refuse("intermediate block does not fit: offset %d stride %d size %dx%d length %d", pOff, pStride, w, h, len(p))
		}
		if overlaps(unsafe.Pointer(unsafe.SliceData(dst)), dstOff, dLast, int(unsafe.Sizeof(dst[0])), unsafe.Pointer(unsafe.SliceData(p)), first, last, 4) {
			return refuse("destination block and intermediates share memory")
		}
	}
	return nil
}

// WeightUni writes the w x h block of the explicit weighted prediction of one list
// (Equation 8-275) of the intermediates p[pOff + y*pStride + x] to dst[dstOff + y*dstStride + x]:
//
//	dst = Clip3( 0, (1 << bitDepth) - 1, ( ( p * wt + 2^(log2Wd - 1) ) >> log2Wd ) + o )
//
// in 32-bit arithmetic. The width is even from 2 to 64, the height from 1 to 64, bitDepth 8 to
// 12, log2Wd 2 to 13, |wt| at most 512 and |o| at most 65536; the intermediates are those of
// interpolation (within 17 bits in magnitude). dst and p must not share memory. WeightUni
// panics where TryWeightUni returns an error, before anything is written.
func WeightUni(dst []uint16, dstOff, dstStride int, p []int32, pOff, pStride, w, h, bitDepth, log2Wd int, wt, o int32) {
	if err := TryWeightUni(dst, dstOff, dstStride, p, pOff, pStride, w, h, bitDepth, log2Wd, wt, o); err != nil {
		panic(err)
	}
}

// TryWeightUni is WeightUni returning the refusal as an error, with nothing written.
func TryWeightUni(dst []uint16, dstOff, dstStride int, p []int32, pOff, pStride, w, h, bitDepth, log2Wd int, wt, o int32) error {
	if err := checkWeight(dst, dstOff, dstStride, [][]int32{p}, pOff, pStride, w, h, bitDepth, log2Wd, wt, o); err != nil {
		return err
	}
	q := [4]int32{wt, 1 << uint(log2Wd-1), o, int32(1)<<uint(bitDepth) - 1}
	weightUniDispatch(dst, dstOff, dstStride, p, pOff, pStride, w, h, uint(log2Wd), &q)
	return nil
}

// WeightBi writes the explicit weighted prediction of two lists (Equation 8-277):
//
//	dst = Clip3( 0, (1 << bitDepth) - 1, ( p0 * w0 + p1 * w1 + ( ( o0 + o1 + 1 ) << log2Wd ) ) >> ( log2Wd + 1 ) )
//
// with WeightUni's preconditions; the default average of 8.5.3.3.4.2 is w0 = w1 = 1, o0 = o1 = 0
// and log2Wd = Max( 2, 14 - bitDepth ).
func WeightBi(dst []uint16, dstOff, dstStride int, p0, p1 []int32, pOff, pStride, w, h, bitDepth, log2Wd int, w0, w1, o0, o1 int32) {
	if err := TryWeightBi(dst, dstOff, dstStride, p0, p1, pOff, pStride, w, h, bitDepth, log2Wd, w0, w1, o0, o1); err != nil {
		panic(err)
	}
}

// TryWeightBi is WeightBi returning the refusal as an error, with nothing written.
func TryWeightBi(dst []uint16, dstOff, dstStride int, p0, p1 []int32, pOff, pStride, w, h, bitDepth, log2Wd int, w0, w1, o0, o1 int32) error {
	if err := checkWeight(dst, dstOff, dstStride, [][]int32{p0, p1}, pOff, pStride, w, h, bitDepth, log2Wd, w0, w1, o0, o1); err != nil {
		return err
	}
	q := [4]int32{w0, w1, (o0 + o1 + 1) << uint(log2Wd), int32(1)<<uint(bitDepth) - 1}
	weightBiDispatch(dst, dstOff, dstStride, p0, p1, pOff, pStride, w, h, uint(log2Wd), &q)
	return nil
}
