package mc

import (
	"errors"
	"math"
	"unsafe"
)

// Plane is a picture plane whose slice and stride were proved once, by
// NewPlane. The methods below take the plane as the destination or the source
// of a block and prove only what depends on the block (its offsets, size and
// fractions, and that the two planes are different memory) with a few integer
// comparisons, where PutLuma and the other plain functions redo the whole
// footprint arithmetic on every call. The only constructor is NewPlane; the
// zero Plane has no pixels and refuses every block.
//
// Every method returns an error, with nothing written, where the plain
// functions panic or return one: a refusal is a value the caller answers by
// taking its own path, as with the Try functions.
type Plane struct {
	pix    []byte
	stride int
}

// NewPlane proves pix and stride once: a stride of at least 16 (a luma block is
// at most 16 wide, a chroma block with its extra column 9, so the rows of any
// block cannot overlap) and a slice no longer than MaxInt/32, which keeps every
// product of a stride by a row count below (at most 21 rows) inside int on a
// 32-bit target. Refusals are errors.
func NewPlane(pix []byte, stride int) (Plane, error) {
	return newPlane(pix, stride, math.MaxInt/32)
}

// newPlane is NewPlane with the length cap a parameter, so a test can reach
// both sides of it without a slice of 2^58 bytes.
func newPlane(pix []byte, stride, maxLen int) (Plane, error) {
	if stride < 16 {
		return Plane{}, errors.New("mc: plane stride is smaller than 16")
	}
	if len(pix) > maxLen {
		return Plane{}, errors.New("mc: plane too large for the checked-once footprints")
	}
	if stride > len(pix) {
		return Plane{}, errors.New("mc: plane stride exceeds the plane")
	}
	return Plane{pix: pix, stride: stride}, nil
}

var (
	errSize      = errors.New("mc: unsupported block size")
	errFrac      = errors.New("mc: fractional offset outside its range")
	errFootprint = errors.New("mc: block footprint does not fit its plane")
	errShared    = errors.New("mc: destination and source planes share memory")
)

// disjoint reports whether the bytes of two planes are different memory,
// decided from the addresses (whole slices: a block of one plane can then
// never touch a block of the other).
func disjoint(a, b []byte) bool {
	pa := uintptr(unsafe.Pointer(unsafe.SliceData(a)))
	pb := uintptr(unsafe.Pointer(unsafe.SliceData(b)))
	return pa+uintptr(len(a)) <= pb || pb+uintptr(len(b)) <= pa
}

// fits reports that a block with rowsBefore/colsBefore of margin before off and
// rowsAfter/colsAfter after the last row and column, w columns, lies inside n
// samples at stride, with off >= 0: off - before >= 0 and off + after < n. Every
// product is at most 21*stride, and stride is at most n (checked here), so none
// can wrap given NewPlane's cap on n.
func fits(n, stride, off, rowsBefore, rowsAfter, colsBefore, colsAfter int) bool {
	lo := rowsBefore*stride + colsBefore
	hi := n - 1 - (rowsAfter*stride + colsAfter)
	return hi >= lo && uint(off-lo) <= uint(hi-lo)
}

// PutLuma is PutLuma of the package for the block of dst at dstOff and the
// source footprint of src at srcOff (the integer-sample position; the
// fractions xFrac and yFrac select the interpolation), with the same sizes and
// the same margins. The dst plane stride must be at least w, which NewPlane
// guarantees.
func (dst *Plane) PutLuma(dstOff int, src *Plane, srcOff, w, h, xFrac, yFrac int) error {
	if err := dst.checkLuma(dstOff, src, srcOff, w, h, xFrac, yFrac); err != nil {
		return err
	}
	lumaDispatch(dst.pix, dstOff, dst.stride, src.pix, srcOff, src.stride, w, h, xFrac, yFrac, false)
	return nil
}

// AvgLuma is AvgLuma of the package on planes; see PutLuma.
func (dst *Plane) AvgLuma(dstOff int, src *Plane, srcOff, w, h, xFrac, yFrac int) error {
	if err := dst.checkLuma(dstOff, src, srcOff, w, h, xFrac, yFrac); err != nil {
		return err
	}
	lumaDispatch(dst.pix, dstOff, dst.stride, src.pix, srcOff, src.stride, w, h, xFrac, yFrac, true)
	return nil
}

func (dst *Plane) checkLuma(dstOff int, src *Plane, srcOff, w, h, xFrac, yFrac int) error {
	if !validSize(w, h) {
		return errSize
	}
	if uint(xFrac) > 3 || uint(yFrac) > 3 {
		return errFrac
	}
	before, after := 0, 0
	if yFrac != 0 {
		before, after = 2, 3
	}
	left, right := 0, 0
	if xFrac != 0 {
		left, right = 2, 3
	}
	if !fits(len(dst.pix), dst.stride, dstOff, 0, h-1, 0, w-1) ||
		!fits(len(src.pix), src.stride, srcOff, before, h-1+after, left, w-1+right) {
		return errFootprint
	}
	if !disjoint(dst.pix, src.pix) {
		return errShared
	}
	return nil
}

// ChromaPair is the Cb and the Cr plane of a picture: two Planes of one length
// and stride whose bytes are disjoint, proved once by NewChromaPair.
type ChromaPair struct {
	cb, cr Plane
}

// NewChromaPair proves both planes, their equal length (one footprint proof
// stands for both) and that they share no memory.
func NewChromaPair(cb, cr []byte, stride int) (ChromaPair, error) {
	if len(cb) != len(cr) {
		return ChromaPair{}, errors.New("mc: the Cb and Cr planes differ in length")
	}
	a, err := NewPlane(cb, stride)
	if err != nil {
		return ChromaPair{}, err
	}
	b := a // a plane is its slice and its stride, and the strides are equal
	b.pix = cr
	if !disjoint(cb, cr) {
		return ChromaPair{}, errShared
	}
	return ChromaPair{cb: a, cr: b}, nil
}

// PutChroma is PutChromaPair of the package for both planes of dst at dstOff
// and of src at srcOff (the integer position; the fractions are in eighths).
func (dst *ChromaPair) PutChroma(dstOff int, src *ChromaPair, srcOff, w, h, xFrac, yFrac int) error {
	if err := dst.check(dstOff, src, srcOff, w, h, xFrac, yFrac); err != nil {
		return err
	}
	putChromaDispatch(dst.cb.pix, dst.cr.pix, dstOff, dst.cb.stride, src.cb.pix, src.cr.pix, srcOff, src.cb.stride, w, h, xFrac, yFrac, 2, false)
	return nil
}

// AvgChroma is AvgChromaPair of the package on planes; see PutChroma.
func (dst *ChromaPair) AvgChroma(dstOff int, src *ChromaPair, srcOff, w, h, xFrac, yFrac int) error {
	if err := dst.check(dstOff, src, srcOff, w, h, xFrac, yFrac); err != nil {
		return err
	}
	putChromaDispatch(dst.cb.pix, dst.cr.pix, dstOff, dst.cb.stride, src.cb.pix, src.cr.pix, srcOff, src.cb.stride, w, h, xFrac, yFrac, 2, true)
	return nil
}

func (dst *ChromaPair) check(dstOff int, src *ChromaPair, srcOff, w, h, xFrac, yFrac int) error {
	if !validChromaSize(w, h) {
		return errSize
	}
	if uint(xFrac) > 7 || uint(yFrac) > 7 {
		return errFrac
	}
	if !fits(len(dst.cb.pix), dst.cb.stride, dstOff, 0, h-1, 0, w-1) ||
		!fits(len(src.cb.pix), src.cb.stride, srcOff, 0, h, 0, w) {
		return errFootprint
	}
	if !disjoint(dst.cb.pix, src.cb.pix) || !disjoint(dst.cb.pix, src.cr.pix) ||
		!disjoint(dst.cr.pix, src.cb.pix) || !disjoint(dst.cr.pix, src.cr.pix) {
		return errShared
	}
	return nil
}
