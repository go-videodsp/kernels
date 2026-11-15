package mc

import "fmt"

//go:generate go run ../internal/genmc ..

// PutLuma writes the w x h block of H.264 luma samples predicted at the
// quarter-sample offset (xFrac, yFrac) (ITU-T H.264 (V16) (06/2026), clause
// 8.4.2.2.1, Table 8-12) to dst.
//
// src[srcOff] is the full-sample location of the block's top-left sample
// (G in Figure 8-4). The kernel reads the plane without clamping, so the
// caller guarantees the whole footprint lies inside src: the block extended
// by the filter's margins, 2 samples before and 3 after, in each direction in
// which the position is interpolated (xFrac != 0 for the horizontal one,
// yFrac != 0 for the vertical one). The footprint is the bounding rectangle of
// those margins. A block whose footprint leaves the picture must be built by
// edge emulation first (Equations 8-239 and 8-240 clamp the coordinates).
//
// The supported sizes are the seven the standard uses for luma prediction:
// 16x16, 16x8, 8x16, 8x8, 8x4, 4x8 and 4x4. PutLuma panics, before any sample
// is read or written, when the size or the fractional offset is not supported,
// the source stride is not positive, the destination stride is smaller than
// w (rows would overlap), or the footprint or the destination block does not
// fit its slice.
//
// dst and src must not share memory: PutLuma panics when the bytes of the
// destination block (first to last sample, rows included) intersect the bytes
// of the source footprint, which it can tell from the addresses. Two slices of
// one array are fine while those byte ranges are disjoint. The check is on
// the bounding ranges, so a destination placed between the rows of the source
// footprint is refused even if no sample is shared.
func PutLuma(dst []byte, dstOff, dstStride int, src []byte, srcOff, srcStride, w, h, xFrac, yFrac int) {
	checkLuma(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, xFrac, yFrac)
	lumaDispatch(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, xFrac, yFrac, false)
}

// AvgLuma is PutLuma fused with the default weighted sample prediction of
// clause 8.4.2.3.1: each predicted sample p is averaged into the sample
// already in dst, dst = (dst + p + 1) >> 1 (Equation 8-273), the second
// prediction of a bi-predicted block. The preconditions are PutLuma's.
func AvgLuma(dst []byte, dstOff, dstStride int, src []byte, srcOff, srcStride, w, h, xFrac, yFrac int) {
	checkLuma(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, xFrac, yFrac)
	lumaDispatch(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, xFrac, yFrac, true)
}

func validSize(w, h int) bool {
	switch w {
	case 16:
		return h == 16 || h == 8
	case 8:
		return h == 16 || h == 8 || h == 4
	case 4:
		return h == 8 || h == 4
	}
	return false
}

// checkLuma is the bounds proof: it runs once per call, in Go, before any
// assembly. Arithmetic is in int64 so that no 32-bit target can overflow
// (h and the margins are at most 19, a stride at most 2^31).
func checkLuma(dst []byte, dstOff, dstStride int, src []byte, srcOff, srcStride, w, h, xFrac, yFrac int) {
	dstLen, srcLen := len(dst), len(src)
	if !validSize(w, h) {
		panic(fmt.Sprintf("mc: unsupported luma block size %dx%d", w, h))
	}
	if xFrac < 0 || xFrac > 3 || yFrac < 0 || yFrac > 3 {
		panic(fmt.Sprintf("mc: fractional offset (%d, %d) outside 0..3", xFrac, yFrac))
	}
	if srcStride <= 0 {
		panic(fmt.Sprintf("mc: source stride must be positive, got %d", srcStride))
	}
	// Rows of the destination block must not overlap: the kernels write in
	// column strips, so overlapping rows would make the result depend on the
	// order of the writes.
	if dstStride < w {
		panic(fmt.Sprintf("mc: destination stride %d is smaller than the block width %d", dstStride, w))
	}

	// Destination block: rows 0..h-1, columns 0..w-1.
	dLast := int64(dstOff) + int64(h-1)*int64(dstStride) + int64(w-1)
	if dstOff < 0 || dLast >= int64(dstLen) {
		panic(fmt.Sprintf("mc: destination block does not fit: offset %d stride %d size %dx%d length %d", dstOff, dstStride, w, h, dstLen))
	}

	// Source footprint: margins -2/+3 in each interpolated direction.
	before, after := int64(0), int64(0) // rows
	if yFrac != 0 {
		before, after = 2, 3
	}
	left, right := int64(0), int64(0) // columns
	if xFrac != 0 {
		left, right = 2, 3
	}
	first := int64(srcOff) - before*int64(srcStride) - left
	last := int64(srcOff) + (int64(h-1)+after)*int64(srcStride) + int64(w-1) + right
	if first < 0 || last >= int64(srcLen) {
		panic(fmt.Sprintf("mc: source footprint does not fit: offset %d stride %d size %dx%d frac (%d, %d) length %d", srcOff, srcStride, w, h, xFrac, yFrac, len(src)))
	}
	if overlaps(dst, int(dstOff), int(dLast), src, int(first), int(last)) {
		panic("mc: destination block and source footprint share memory")
	}
}
