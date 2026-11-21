package lf

import (
	"errors"
	"fmt"
	"math"
	"unsafe"
)

// Plane is a picture plane whose geometry was proved once, by NewPlane, so
// that the edge filters below pay two comparisons per call instead of the
// footprint arithmetic of DeblockLumaV and friends. The only constructor is
// NewPlane; a Plane built any other way has no limits and refuses every call.
//
// Each method takes the offset of q0 as the plain function does (the same
// footprints, the same panics for the indexes and the strengths) and panics,
// before touching the plane, when the footprint does not fit. The limits are
// worked out in NewPlane from len(pix) and stride alone, with no product that
// can wrap: len(pix) is capped at MaxInt/16 there, so no multiple of stride up
// to 16 can overflow whatever stride is.
type Plane struct {
	pix    []byte
	stride int
	// Per kind of edge, the offsets for which the footprint fits are
	// lo .. lo+span-1, with first = off - back >= 0 and last = off + forward
	// < len(pix). A span of 0 (the zero Plane, or a plane too short) admits no
	// offset.
	lumaVSpan, lumaHLo, lumaHSpan       int
	chromaVSpan, chromaHLo, chromaHSpan int
}

// Limits of the footprints, in samples (see checkEdge and checkChroma): a
// vertical luma edge reads columns -4..+3 of 16 lines, a horizontal one rows
// -4..+3 of 16 columns; chroma reads -2..+1 of 8 lines or columns.
const (
	lumaVLo   = 4 // columns before the edge
	chromaVLo = 2
)

// span is the number of offsets lo..hi, 0 when there are none.
func span(lo, hi int) int {
	if hi < lo {
		return 0
	}
	return hi - lo + 1
}

// fits reports lo <= off < lo+span in one comparison: off-lo wraps to a huge
// unsigned value below lo, and span is at most len(pix) so no wrapped value
// of an off near MinInt reaches it.
func fits(off, lo, span int) bool {
	return uint(off-lo) < uint(span)
}

// NewPlane proves pix and stride once. It refuses a stride below 16 (the 16
// lines of a luma edge must not overlap; chroma needs 8 and is covered) and a
// slice longer than MaxInt/16 (the bound that keeps every product below in
// range on a 32-bit target). A refused plane is a refusal, never a panic: the
// caller keeps the plain functions for it.
func NewPlane(pix []byte, stride int) (Plane, error) {
	return newPlane(pix, stride, math.MaxInt/16)
}

// newPlane is NewPlane with the length cap a parameter, so a test can reach
// both sides of it without a slice of 2^59 bytes.
func newPlane(pix []byte, stride, maxLen int) (Plane, error) {
	n := len(pix)
	if stride < 16 {
		return Plane{}, fmt.Errorf("lf: stride %d is smaller than the 16 lines of an edge", stride)
	}
	if n > maxLen {
		return Plane{}, errors.New("lf: plane too large for the checked-once footprints")
	}
	if stride > n {
		return Plane{}, errors.New("lf: plane stride exceeds the plane")
	}
	return Plane{
		pix: pix, stride: stride,
		lumaVSpan:   span(lumaVLo, n-4-15*stride),
		lumaHLo:     4 * stride,
		lumaHSpan:   span(4*stride, n-16-3*stride),
		chromaVSpan: span(chromaVLo, n-2-7*stride),
		chromaHLo:   2 * stride,
		chromaHSpan: span(2*stride, n-8-stride),
	}, nil
}

func (p *Plane) luma(off int, vertical bool) {
	if vertical {
		if !fits(off, lumaVLo, p.lumaVSpan) {
			p.refuse("luma", off, vertical)
		}
		return
	}
	if !fits(off, p.lumaHLo, p.lumaHSpan) {
		p.refuse("luma", off, vertical)
	}
}

func (p *Plane) chroma(off int, vertical bool) {
	if vertical {
		if !fits(off, chromaVLo, p.chromaVSpan) {
			p.refuse("chroma", off, vertical)
		}
		return
	}
	if !fits(off, p.chromaHLo, p.chromaHSpan) {
		p.refuse("chroma", off, vertical)
	}
}

func (p *Plane) refuse(kind string, off int, vertical bool) {
	panic(fmt.Sprintf("lf: %s edge footprint does not fit: offset %d stride %d vertical %v length %d", kind, off, p.stride, vertical, len(p.pix)))
}

func checkLumaStrength(bS [4]uint8) {
	for _, s := range bS {
		if s > 3 {
			panic(fmt.Sprintf("lf: bS %d is not below 4; use the Intra form for bS 4", s))
		}
	}
}

// LumaV is DeblockLumaV on the plane.
func (p *Plane) LumaV(off, indexA, indexB int, bS [4]uint8) {
	p.luma(off, true)
	checkIndex("indexA", indexA)
	checkIndex("indexB", indexB)
	checkLumaStrength(bS)
	deblockNormal(p.pix, off, p.stride, true, indexA, indexB, bS)
}

// LumaH is DeblockLumaH on the plane.
func (p *Plane) LumaH(off, indexA, indexB int, bS [4]uint8) {
	p.luma(off, false)
	checkIndex("indexA", indexA)
	checkIndex("indexB", indexB)
	checkLumaStrength(bS)
	deblockNormal(p.pix, off, p.stride, false, indexA, indexB, bS)
}

// LumaIntraV is DeblockLumaIntraV on the plane.
func (p *Plane) LumaIntraV(off, indexA, indexB int) {
	p.luma(off, true)
	checkIndex("indexA", indexA)
	checkIndex("indexB", indexB)
	deblockStrong(p.pix, off, p.stride, true, indexA, indexB)
}

// LumaIntraH is DeblockLumaIntraH on the plane.
func (p *Plane) LumaIntraH(off, indexA, indexB int) {
	p.luma(off, false)
	checkIndex("indexA", indexA)
	checkIndex("indexB", indexB)
	deblockStrong(p.pix, off, p.stride, false, indexA, indexB)
}

// ChromaV is DeblockChromaV on the plane.
func (p *Plane) ChromaV(off, indexA, indexB int, bS [4]uint8) {
	p.chroma(off, true)
	checkIndex("indexA", indexA)
	checkIndex("indexB", indexB)
	checkChromaStrength(bS)
	deblockChromaNormal(p.pix, off, p.stride, true, indexA, indexB, bS)
}

// ChromaH is DeblockChromaH on the plane.
func (p *Plane) ChromaH(off, indexA, indexB int, bS [4]uint8) {
	p.chroma(off, false)
	checkIndex("indexA", indexA)
	checkIndex("indexB", indexB)
	checkChromaStrength(bS)
	deblockChromaNormal(p.pix, off, p.stride, false, indexA, indexB, bS)
}

// ChromaIntraV is DeblockChromaIntraV on the plane.
func (p *Plane) ChromaIntraV(off, indexA, indexB int) {
	p.chroma(off, true)
	checkIndex("indexA", indexA)
	checkIndex("indexB", indexB)
	deblockChromaStrong(p.pix, off, p.stride, true, indexA, indexB)
}

// ChromaIntraH is DeblockChromaIntraH on the plane.
func (p *Plane) ChromaIntraH(off, indexA, indexB int) {
	p.chroma(off, false)
	checkIndex("indexA", indexA)
	checkIndex("indexB", indexB)
	deblockChromaStrong(p.pix, off, p.stride, false, indexA, indexB)
}

// PlanePair is the Cb and the Cr plane of a picture: two Planes of one length
// and one stride that share no memory, proved once by NewPlanePair. Its
// methods are the DeblockChromaPair functions on it.
type PlanePair struct {
	cb, cr Plane
}

// NewPlanePair proves both planes, that their lengths and strides are equal
// (so one set of limits stands for both) and that their bytes are disjoint,
// decided from the addresses.
func NewPlanePair(cb, cr []byte, stride int) (PlanePair, error) {
	if len(cb) != len(cr) {
		return PlanePair{}, errors.New("lf: the Cb and Cr planes differ in length")
	}
	a, err := NewPlane(cb, stride)
	if err != nil {
		return PlanePair{}, err
	}
	b := a // the limits depend on the length and the stride alone, which are equal
	b.pix = cr
	pa := uintptr(unsafe.Pointer(unsafe.SliceData(cb)))
	pb := uintptr(unsafe.Pointer(unsafe.SliceData(cr)))
	n := uintptr(len(cb))
	if pa < pb+n && pb < pa+n {
		return PlanePair{}, errors.New("lf: the Cb and Cr planes share memory")
	}
	return PlanePair{cb: a, cr: b}, nil
}

// ChromaPairV is DeblockChromaPairV on the pair.
func (q *PlanePair) ChromaPairV(off, indexACb, indexBCb, indexACr, indexBCr int, bS [4]uint8) {
	q.cb.chroma(off, true)
	checkIndex("indexA", indexACb)
	checkIndex("indexB", indexBCb)
	checkIndex("indexA", indexACr)
	checkIndex("indexB", indexBCr)
	checkChromaStrength(bS)
	deblockChromaPairNormal(q.cb.pix, q.cr.pix, off, q.cb.stride, true, indexACb, indexBCb, indexACr, indexBCr, bS)
}

// ChromaPairH is DeblockChromaPairH on the pair.
func (q *PlanePair) ChromaPairH(off, indexACb, indexBCb, indexACr, indexBCr int, bS [4]uint8) {
	q.cb.chroma(off, false)
	checkIndex("indexA", indexACb)
	checkIndex("indexB", indexBCb)
	checkIndex("indexA", indexACr)
	checkIndex("indexB", indexBCr)
	checkChromaStrength(bS)
	deblockChromaPairNormal(q.cb.pix, q.cr.pix, off, q.cb.stride, false, indexACb, indexBCb, indexACr, indexBCr, bS)
}

// ChromaPairIntraV is DeblockChromaPairIntraV on the pair.
func (q *PlanePair) ChromaPairIntraV(off, indexACb, indexBCb, indexACr, indexBCr int) {
	q.cb.chroma(off, true)
	checkIndex("indexA", indexACb)
	checkIndex("indexB", indexBCb)
	checkIndex("indexA", indexACr)
	checkIndex("indexB", indexBCr)
	deblockChromaPairStrong(q.cb.pix, q.cr.pix, off, q.cb.stride, true, indexACb, indexBCb, indexACr, indexBCr)
}

// ChromaPairIntraH is DeblockChromaPairIntraH on the pair.
func (q *PlanePair) ChromaPairIntraH(off, indexACb, indexBCb, indexACr, indexBCr int) {
	q.cb.chroma(off, false)
	checkIndex("indexA", indexACb)
	checkIndex("indexB", indexBCb)
	checkIndex("indexA", indexACr)
	checkIndex("indexB", indexBCr)
	deblockChromaPairStrong(q.cb.pix, q.cr.pix, off, q.cb.stride, false, indexACb, indexBCb, indexACr, indexBCr)
}
