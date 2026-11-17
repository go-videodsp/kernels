package hevc

import (
	"unsafe"

	"github.com/go-videodsp/kernels/internal/bounds"
)

// The kernels over the bytes of an 8-bit picture: the filters of HPass8, HPass4, VPassU8,
// VPassU4, HPassW8, HPassW4 and the widening of Widen with a source of []uint8, the stride and
// the offsets in samples (bytes). Their contract is the uint16 form's, with the samples from 0
// to 255 and the bit depth 8 (shift 0); each returns the refusal as an error, with nothing
// written, and has no panicking twin: the 8-bit pictures of the callers are checked at the
// call.

// TryHPass8Bytes is TryHPass8 over a []uint8 source.
func TryHPass8Bytes(dst []int16, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, c *[8]int16) error {
	if err := checkH(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, 8, 4, before, after, c != nil); err != nil {
		return err
	}
	hpass8BDispatch(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, 0, c)
	return nil
}

// TryHPass4Bytes is TryHPass4 over a []uint8 source.
func TryHPass4Bytes(dst []int16, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, c *[4]int16) error {
	if err := checkH(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, 8, 2, chromaBefore, chromaAfter, c != nil); err != nil {
		return err
	}
	hpass4BDispatch(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, 0, c)
	return nil
}

// TryVPassU8Bytes is TryVPassU8 over a []uint8 source.
func TryVPassU8Bytes(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, c *[8]int16) error {
	if err := checkVU(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, 8, 4, before, after, c != nil); err != nil {
		return err
	}
	vpassU8BDispatch(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, 0, c)
	return nil
}

// TryVPassU4Bytes is TryVPassU4 over a []uint8 source.
func TryVPassU4Bytes(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, c *[4]int16) error {
	if err := checkVU(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, 8, 2, chromaBefore, chromaAfter, c != nil); err != nil {
		return err
	}
	vpassU4BDispatch(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, 0, c)
	return nil
}

// TryHPassW8Bytes is TryHPassW8 over a []uint8 source.
func TryHPassW8Bytes(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, c *[8]int16) error {
	if err := checkHW(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, 8, 4, before, after, c != nil); err != nil {
		return err
	}
	hpassW8BDispatch(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, 0, c)
	return nil
}

// TryHPassW4Bytes is TryHPassW4 over a []uint8 source.
func TryHPassW4Bytes(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, c *[4]int16) error {
	if err := checkHW(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, 8, 2, chromaBefore, chromaAfter, c != nil); err != nil {
		return err
	}
	hpassW4BDispatch(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, 0, c)
	return nil
}

// TryWidenBytes is TryWiden over a []uint8 source: dst[dstOff + y*dstStride + x] =
// int32(src[srcOff + y*srcStride + x]) << shift.
func TryWidenBytes(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h, shift int) error {
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
	if overlaps(unsafe.Pointer(unsafe.SliceData(dst)), dstOff, dLast, 4, unsafe.Pointer(unsafe.SliceData(src)), sFirst, sLast, 1) {
		return refuse("destination block and source block share memory")
	}
	widenBDispatch(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, uint(shift))
	return nil
}
