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

func checkVU[S uint8 | uint16](dst []int32, dstOff, dstStride int, src []S, srcOff, srcStride, w, h, bitDepth, unit, bef, aft int, haveC bool) error {
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
	if overlaps(unsafe.Pointer(unsafe.SliceData(dst)), dstOff, dLast, 4, unsafe.Pointer(unsafe.SliceData(src)), sFirst, sLast, int(unsafe.Sizeof(src[0]))) {
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

// The horizontal filter of a block with no vertical fraction writes int32 results: the first
// pass of the two-dimensional filter into int16 (HPass8) and the widening of its result to int32
// that the one-dimensional case needs are one call.

func checkHW[S uint8 | uint16](dst []int32, dstOff, dstStride int, src []S, srcOff, srcStride, w, h, bitDepth, unit, bef, aft int, haveC bool) error {
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
	sFirst, sLast, ok := bounds.Window(len(src), srcOff, srcStride, 0, h-1, bef, w-1+aft)
	if !ok {
		return refuse("source footprint does not fit: offset %d stride %d size %dx%d length %d", srcOff, srcStride, w, h, len(src))
	}
	if overlaps(unsafe.Pointer(unsafe.SliceData(dst)), dstOff, dLast, 4, unsafe.Pointer(unsafe.SliceData(src)), sFirst, sLast, int(unsafe.Sizeof(src[0]))) {
		return refuse("destination block and source footprint share memory")
	}
	return nil
}

// HPassW8 writes the w x h block of the horizontal eight-tap filter of the samples
// src[srcOff + y*srcStride + x] to dst[dstOff + y*dstStride + x] as int32:
//
//	dst = (c[0]*s[x-3] + c[1]*s[x-2] + ... + c[7]*s[x+4]) >> (bitDepth - 8)
//
// the result HPass8 gives as int16 (the two agree wherever the int16 does not wrap, which the
// filters of the text never make it do). Its preconditions are HPass8's. HPassW8 panics where
// TryHPassW8 returns an error, before anything is written.
func HPassW8(dst []int32, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h, bitDepth int, c *[8]int16) {
	if err := TryHPassW8(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, bitDepth, c); err != nil {
		panic(err)
	}
}

// TryHPassW8 is HPassW8 returning the refusal as an error, with nothing written.
func TryHPassW8(dst []int32, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h, bitDepth int, c *[8]int16) error {
	if err := checkHW(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, bitDepth, 4, before, after, c != nil); err != nil {
		return err
	}
	hpassW8Dispatch(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, uint(bitDepth-8), c)
	return nil
}

// HPassW4 is the four-tap chroma filter, the footprint 1 column before and 2 after, the width any
// even number from 2 to 64; its other preconditions are HPassW8's.
func HPassW4(dst []int32, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h, bitDepth int, c *[4]int16) {
	if err := TryHPassW4(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, bitDepth, c); err != nil {
		panic(err)
	}
}

// TryHPassW4 is HPassW4 returning the refusal as an error, with nothing written.
func TryHPassW4(dst []int32, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h, bitDepth int, c *[4]int16) error {
	if err := checkHW(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, bitDepth, 2, chromaBefore, chromaAfter, c != nil); err != nil {
		return err
	}
	hpassW4Dispatch(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, uint(bitDepth-8), c)
	return nil
}

// Widen writes the w x h block dst[dstOff + y*dstStride + x] = int32(src[srcOff + y*srcStride + x]) << shift:
// the intermediate samples of a block at a full-sample position (8.5.3.3.3.2: the sample shifted
// left by shift3 = Max( 2, 14 - bitDepth )). The width is even from 2 to 64, the height from 1 to
// 64, the shift from 0 to 15; dst and src must not share memory. Widen panics where TryWiden
// returns an error, before anything is written.
func Widen(dst []int32, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h, shift int) {
	if err := TryWiden(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift); err != nil {
		panic(err)
	}
}

// TryWiden is Widen returning the refusal as an error, with nothing written.
func TryWiden(dst []int32, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h, shift int) error {
	switch {
	case w < 2 || w > maxBlock || w%2 != 0 || h < 1 || h > maxBlock:
		return refuse("block %dx%d: the width is even from 2 to %d, the height from 1 to %d", w, h, maxBlock, maxBlock)
	case shift < 0 || shift > 15:
		return refuse("shift %d is not from 0 to 15", shift)
	case dstStride < w || srcStride < w:
		return refuse("a stride (destination %d, source %d) is smaller than the block width %d", dstStride, srcStride, w)
	}
	_, dLast, ok := bounds.Window(len(dst), dstOff, dstStride, 0, h-1, 0, w-1)
	if !ok {
		return refuse("destination block does not fit: offset %d stride %d size %dx%d length %d", dstOff, dstStride, w, h, len(dst))
	}
	sFirst, sLast, ok := bounds.Window(len(src), srcOff, srcStride, 0, h-1, 0, w-1)
	if !ok {
		return refuse("source block does not fit: offset %d stride %d size %dx%d length %d", srcOff, srcStride, w, h, len(src))
	}
	if overlaps(unsafe.Pointer(unsafe.SliceData(dst)), dstOff, dLast, 4, unsafe.Pointer(unsafe.SliceData(src)), sFirst, sLast, int(unsafe.Sizeof(src[0]))) {
		return refuse("destination block and source block share memory")
	}
	widenDispatch(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, uint(shift))
	return nil
}
