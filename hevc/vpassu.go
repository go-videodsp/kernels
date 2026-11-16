package hevc

import (
	"unsafe"

	"github.com/go-videodsp/kernels/internal/bounds"
)

// The vertical filter over the samples of a picture, for a block whose fractional position
// is vertical only (ITU-T H.265 (V11) (01/2026), clauses 8.5.3.3.3.2 and 8.5.3.3.3.3: the
// sum of the taps times the samples of the rows around the position, shifted right by
// shift1 = bitDepth - 8): the one-dimensional filter of a block that has no horizontal
// fraction, which has no first pass and so reads the samples themselves.

func checkVU(dst []int32, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h, bitDepth, unit, bef, aft int, haveC bool) error {
	if err := checkGeometry(dstStride, srcStride, w, h, unit, haveC); err != nil {
		return err
	}
	if bitDepth < 8 || bitDepth > 12 {
		return refuse("bit depth %d is not from 8 to 12", bitDepth)
	}
	_, dLast, ok := bounds.Window(len(dst), dstOff, dstStride, 0, h-1, 0, w-1)
	if !ok {
		return refuse("destination block does not fit: offset %d stride %d size %dx%d length %d", dstOff, dstStride, w, h, len(dst))
	}
	sFirst, sLast, ok := bounds.Window(len(src), srcOff, srcStride, bef, h-1+aft, 0, w-1)
	if !ok {
		return refuse("source footprint does not fit: offset %d stride %d size %dx%d length %d", srcOff, srcStride, w, h, len(src))
	}
	if overlaps(unsafe.Pointer(unsafe.SliceData(dst)), dstOff, dLast, 4, unsafe.Pointer(unsafe.SliceData(src)), sFirst, sLast, 2) {
		return refuse("destination block and source footprint share memory")
	}
	return nil
}

// VPassU8 writes the w x h block of the vertical eight-tap filter of the samples
// src[srcOff + y*srcStride + x] to dst[dstOff + y*dstStride + x]:
//
//	dst = (c[0]*s[y-3] + c[1]*s[y-2] + ... + c[7]*s[y+4]) >> (bitDepth - 8)
//
// w is a multiple of 4 up to 64, h from 1 to 64, bitDepth from 8 to 12, the samples at most
// 32767. The source footprint is the block extended by 3 rows before and 4 after; the caller
// proves it lies in src. dst and src must not share memory. VPassU8 panics where TryVPassU8
// returns an error, before anything is written.
func VPassU8(dst []int32, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h, bitDepth int, c *[8]int16) {
	if err := TryVPassU8(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, bitDepth, c); err != nil {
		panic(err)
	}
}

// TryVPassU8 is VPassU8 returning the refusal as an error, with nothing written.
func TryVPassU8(dst []int32, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h, bitDepth int, c *[8]int16) error {
	if err := checkVU(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, bitDepth, 4, before, after, c != nil); err != nil {
		return err
	}
	vpassU8Dispatch(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, uint(bitDepth-8), c)
	return nil
}

// VPassU4 is the four-tap chroma filter, the footprint 1 row before and 2 after, the width any
// even number from 2 to 64; its other preconditions are VPassU8's.
func VPassU4(dst []int32, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h, bitDepth int, c *[4]int16) {
	if err := TryVPassU4(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, bitDepth, c); err != nil {
		panic(err)
	}
}

// TryVPassU4 is VPassU4 returning the refusal as an error, with nothing written.
func TryVPassU4(dst []int32, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h, bitDepth int, c *[4]int16) error {
	if err := checkVU(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, bitDepth, 2, chromaBefore, chromaAfter, c != nil); err != nil {
		return err
	}
	vpassU4Dispatch(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, uint(bitDepth-8), c)
	return nil
}
