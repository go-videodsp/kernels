package hevc

import (
	"errors"
	"fmt"
	"unsafe"

	"github.com/go-videodsp/kernels/internal/bounds"
)

//go:generate go run ../internal/genhevc ..

// ErrRefused is wrapped by every error a Try form returns: the call was not made.
var ErrRefused = errors.New("hevc: call refused")

func refuse(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrRefused, fmt.Sprintf(format, a...))
}

const (
	maxBlock = 64
	taps     = 8
	before   = 3 // luma taps before the position
	after    = 4 // luma taps after it

	chromaTaps   = 4
	chromaBefore = 1
	chromaAfter  = 2
)

// checkGeometry is what the passes share: the block size (a multiple of unit columns, 4 for
// luma and 2 for chroma), the strides and the coefficients.
func checkGeometry(dstStride, srcStride, w, h, unit int, haveC bool) error {
	switch {
	case w < unit || w > maxBlock || w%unit != 0:
		return refuse("block width %d is not a multiple of %d from %d to %d", w, unit, unit, maxBlock)
	case h < 1 || h > maxBlock:
		return refuse("block height %d is not from 1 to %d", h, maxBlock)
	case !haveC:
		return refuse("no filter coefficients")
	case dstStride < w:
		return refuse("destination stride %d is smaller than the block width %d", dstStride, w)
	case srcStride <= 0:
		return refuse("source stride must be positive, got %d", srcStride)
	}
	return nil
}

// overlaps reports whether the bytes of a (first and last element index, element
// size) and of b intersect in memory.
func overlaps(aBase unsafe.Pointer, aFirst, aLast, aSize int, bBase unsafe.Pointer, bFirst, bLast, bSize int) bool {
	a0 := uintptr(aBase) + uintptr(aFirst)*uintptr(aSize)
	a1 := uintptr(aBase) + uintptr(aLast)*uintptr(aSize) + uintptr(aSize) - 1
	b0 := uintptr(bBase) + uintptr(bFirst)*uintptr(bSize)
	b1 := uintptr(bBase) + uintptr(bLast)*uintptr(bSize) + uintptr(bSize) - 1
	return a0 <= b1 && b0 <= a1
}

func checkH(dst []int16, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h, bitDepth, unit, before, after int, haveC bool) error {
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
	sFirst, sLast, ok := bounds.Window(len(src), srcOff, srcStride, 0, h-1, before, w-1+after)
	if !ok {
		return refuse("source footprint does not fit: offset %d stride %d size %dx%d length %d", srcOff, srcStride, w, h, len(src))
	}
	if overlaps(unsafe.Pointer(unsafe.SliceData(dst)), dstOff, dLast, 2, unsafe.Pointer(unsafe.SliceData(src)), sFirst, sLast, 2) {
		return refuse("destination block and source footprint share memory")
	}
	return nil
}

func checkV(dst []int32, dstOff, dstStride int, src []int16, srcOff, srcStride, w, h, unit, before, after int, haveC bool) error {
	if err := checkGeometry(dstStride, srcStride, w, h, unit, haveC); err != nil {
		return err
	}
	_, dLast, ok := bounds.Window(len(dst), dstOff, dstStride, 0, h-1, 0, w-1)
	if !ok {
		return refuse("destination block does not fit: offset %d stride %d size %dx%d length %d", dstOff, dstStride, w, h, len(dst))
	}
	sFirst, sLast, ok := bounds.Window(len(src), srcOff, srcStride, before, h-1+after, 0, w-1)
	if !ok {
		return refuse("source footprint does not fit: offset %d stride %d size %dx%d length %d", srcOff, srcStride, w, h, len(src))
	}
	if overlaps(unsafe.Pointer(unsafe.SliceData(dst)), dstOff, dLast, 4, unsafe.Pointer(unsafe.SliceData(src)), sFirst, sLast, 2) {
		return refuse("destination block and source footprint share memory")
	}
	return nil
}

// HPass8 writes the w x h block of the horizontal eight-tap filter of the samples
// src[srcOff + y*srcStride + x] to dst[dstOff + y*dstStride + x]:
//
//	dst = int16((c[0]*s[x-3] + c[1]*s[x-2] + ... + c[7]*s[x+4]) >> (bitDepth - 8))
//
// w is a multiple of 4 up to 64, h from 1 to 64, bitDepth from 8 to 12. The source
// footprint is the block extended by 3 columns before and 4 after; the caller
// proves it lies in src (edge emulation first, where it would leave the picture).
// dst and src must not share memory. HPass8 panics where TryHPass8 returns an
// error, before anything is written.
func HPass8(dst []int16, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h, bitDepth int, c *[8]int16) {
	if err := TryHPass8(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, bitDepth, c); err != nil {
		panic(err)
	}
}

// TryHPass8 is HPass8 returning the refusal as an error, with nothing written.
func TryHPass8(dst []int16, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h, bitDepth int, c *[8]int16) error {
	if err := checkH(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, bitDepth, 4, before, after, c != nil); err != nil {
		return err
	}
	hpassDispatch(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, uint(bitDepth-8), c)
	return nil
}

// VPass8 writes the w x h block of the vertical eight-tap filter of the int16
// intermediates src[srcOff + y*srcStride + x] to dst[dstOff + y*dstStride + x]:
//
//	dst = (c[0]*s[y-3] + c[1]*s[y-2] + ... + c[7]*s[y+4]) >> 6
//
// the second stage of the two-dimensional filter (Equations 8-230 to 8-238). The
// preconditions are HPass8's, with the footprint extended by 3 rows before and 4
// after.
func VPass8(dst []int32, dstOff, dstStride int, src []int16, srcOff, srcStride, w, h int, c *[8]int16) {
	if err := TryVPass8(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, c); err != nil {
		panic(err)
	}
}

// TryVPass8 is VPass8 returning the refusal as an error, with nothing written.
func TryVPass8(dst []int32, dstOff, dstStride int, src []int16, srcOff, srcStride, w, h int, c *[8]int16) error {
	if err := checkV(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, 4, before, after, c != nil); err != nil {
		return err
	}
	vpassDispatch(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, c)
	return nil
}
