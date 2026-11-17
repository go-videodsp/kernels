package hevc

import (
	"unsafe"

	"github.com/go-videodsp/kernels/internal/bounds"
)

// Sample adaptive offset (ITU-T H.265 (V11) (01/2026), clause 8.7.3), the part that runs
// on a block whose samples all have their neighbours: the band offset (Equation 8-414) and
// the edge offset (Equations 8-411 and 8-412) of one block of a coding tree block. The
// caller handles the samples whose neighbours are unavailable (picture, slice or tile
// edges), the samples that must not be filtered, and decides what the block is.

// saoMaxOffset bounds the offsets: SaoOffsetVal is at most 31 << 2 in magnitude for the bit
// depths up to 12, so a larger one is a mistake and is refused, which also keeps the sum of
// a sample and its offset inside 16 bits.
const saoMaxOffset = 1 << 12

// saoEdgeOffsets are the element offsets of the two neighbours of an edge offset class,
// Table 8-13: hPos and vPos for classes 0 to 3.
func saoEdgeOffsets(class, stride int) (a, b int) {
	hPos := [4][2]int{{-1, 1}, {0, 0}, {-1, 1}, {1, -1}}
	vPos := [4][2]int{{0, 0}, {-1, 1}, {-1, 1}, {-1, 1}}
	return vPos[class][0]*stride + hPos[class][0], vPos[class][1]*stride + hPos[class][1]
}

// saoMargins are the rows and columns of neighbours an edge offset class reads around the block.
func saoMargins(class int) (rows, cols int) {
	hPos := [4]int{1, 0, 1, 1}
	vPos := [4]int{0, 1, 1, 1}
	return vPos[class], hPos[class]
}

func checkSAO[T uint8 | uint16](dst []T, dstOff, dstStride int, src []T, srcOff, srcStride, w, h, bitDepth int, off *[4]int16, rows, cols int) error {
	switch {
	case w < 1 || w > maxBlock || h < 1 || h > maxBlock:
		return refuse("block %dx%d is not from 1x1 to %dx%d", w, h, maxBlock, maxBlock)
	case bitDepth < 8 || bitDepth > 12:
		return refuse("bit depth %d is not from 8 to 12", bitDepth)
	case off == nil:
		return refuse("no offsets")
	case dstStride < w:
		return refuse("destination stride %d is smaller than the block width %d", dstStride, w)
	case srcStride <= 0:
		return refuse("source stride must be positive, got %d", srcStride)
	}
	for _, o := range off {
		if o < -saoMaxOffset || o > saoMaxOffset {
			return refuse("offset %d is beyond %d", o, saoMaxOffset)
		}
	}
	_, dLast, ok := bounds.Window(len(dst), dstOff, dstStride, 0, h-1, 0, w-1)
	if !ok {
		return refuse("destination block does not fit: offset %d stride %d size %dx%d length %d", dstOff, dstStride, w, h, len(dst))
	}
	sFirst, sLast, ok := bounds.Window(len(src), srcOff, srcStride, rows, h-1+rows, cols, w-1+cols)
	if !ok {
		return refuse("source footprint does not fit: offset %d stride %d size %dx%d length %d", srcOff, srcStride, w, h, len(src))
	}
	if overlaps(unsafe.Pointer(unsafe.SliceData(dst)), dstOff, dLast, int(unsafe.Sizeof(dst[0])), unsafe.Pointer(unsafe.SliceData(src)), sFirst, sLast, int(unsafe.Sizeof(src[0]))) {
		return refuse("destination block and source footprint share memory")
	}
	return nil
}

// SAOBand writes the band offset of the w x h block of src[srcOff + y*srcStride + x] to
// dst[dstOff + y*dstStride + x]: the sample's band is s >> (bitDepth - 5), the four bands
// bandPos, bandPos + 1, bandPos + 2 and bandPos + 3 (modulo 32) have the offsets off[0] to
// off[3], and the result is the sample plus its offset (none outside those bands), clipped
// to 0 .. (1 << bitDepth) - 1. w and h are from 1 to 64, bitDepth from 8 to 12, bandPos from
// 0 to 31, and an offset is at most 4096 in magnitude. dst and src must not share memory.
// SAOBand panics where TrySAOBand returns an error, before anything is written.
//
// The kernels run the columns in groups of eight; the other columns of a row run in Go.
func SAOBand(dst []uint16, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h, bitDepth, bandPos int, off *[4]int16) {
	if err := TrySAOBand(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, bitDepth, bandPos, off); err != nil {
		panic(err)
	}
}

// TrySAOBand is SAOBand returning the refusal as an error, with nothing written.
func TrySAOBand(dst []uint16, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h, bitDepth, bandPos int, off *[4]int16) error {
	if bandPos < 0 || bandPos > 31 {
		return refuse("band position %d is not from 0 to 31", bandPos)
	}
	if err := checkSAO(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, bitDepth, off, 0, 0); err != nil {
		return err
	}
	var p [12]int16
	copy(p[:4], off[:])
	for k := 0; k < 4; k++ {
		p[4+k] = int16((bandPos + k) & 31)
	}
	p[8] = int16(1<<uint(bitDepth) - 1)
	if w8 := w &^ 7; w8 > 0 {
		saoBandDispatch(dst, dstOff, dstStride, src, srcOff, srcStride, w8, h, uint(bitDepth-5), &p)
	}
	if w8 := w &^ 7; w8 < w {
		saoBandRef(dst, dstOff+w8, dstStride, src, srcOff+w8, srcStride, w-w8, h, bitDepth, bandPos, off)
	}
	return nil
}

// SAOEdge writes the edge offset of the w x h block: for the sample s and its two neighbours
// a and b along the class (0 horizontal, 1 vertical, 2 and 3 the diagonals, Table 8-13), the
// edge value is sign(s - a) + sign(s - b) (8-411), and its offset is off[0] for -2, off[1]
// for -1, none for 0, off[2] for 1 and off[3] for 2 (8-412); the result is clipped as in
// SAOBand. Every neighbour of every sample is read, so the footprint is the block plus the
// row and column of neighbours the class names: 1 column each side for class 0, 1 row each
// side for class 1, both for 2 and 3. The preconditions are SAOBand's.
func SAOEdge(dst []uint16, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h, bitDepth, class int, off *[4]int16) {
	if err := TrySAOEdge(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, bitDepth, class, off); err != nil {
		panic(err)
	}
}

// TrySAOEdge is SAOEdge returning the refusal as an error, with nothing written.
func TrySAOEdge(dst []uint16, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h, bitDepth, class int, off *[4]int16) error {
	if class < 0 || class > 3 {
		return refuse("edge offset class %d is not from 0 to 3", class)
	}
	rows, cols := saoMargins(class)
	if err := checkSAO(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, bitDepth, off, rows, cols); err != nil {
		return err
	}
	var p [12]int16
	copy(p[:4], off[:])
	p[4], p[5], p[6], p[7] = -2, -1, 1, 2
	p[8] = int16(1<<uint(bitDepth) - 1)
	offA, offB := saoEdgeOffsets(class, srcStride)
	if w8 := w &^ 7; w8 > 0 {
		saoEdgeDispatch(dst, dstOff, dstStride, src, srcOff, srcStride, w8, h, offA, offB, &p)
	}
	if w8 := w &^ 7; w8 < w {
		saoEdgeRef(dst, dstOff+w8, dstStride, src, srcOff+w8, srcStride, w-w8, h, bitDepth, offA, offB, off)
	}
	return nil
}
