package lf

import (
	"fmt"
	"unsafe"

	"github.com/go-videodsp/kernels/internal/bounds"
)

// Thresholds are the indexA and indexB of one edge (Equations 8-454 and 8-455),
// each 0..51.
type Thresholds struct{ IndexA, IndexB uint8 }

// MBEdges describes the deblocking of one H.264 frame macroblock, 4:2:0, 8-bit
// samples (ITU-T H.264 (V16) (06/2026), clause 8.7), for DeblockMB.
//
// BS[d][e][g] is the boundary strength of edge e (0..3) of direction d (0 for
// the vertical edges, 1 for the horizontal ones), line group g (0..3: four lines
// of luma, two of chroma). An edge whose four strengths are all 0 is not
// filtered (the decoder zeroes the edges it does not filter: a missing
// neighbour or disable_deblocking_filter_idc for edge 0, the 4x4 edges 1 and 3
// of a macroblock with the 8x8 transform). An edge with strengths 1 to 3 (some
// groups may be 0) is filtered with the normal filter; edge 0 with all four
// strengths 4 with the strong filter; any other pattern (a 4 on an inner edge,
// or a 4 beside other values) is refused. The chroma planes use the strengths of
// luma edges 0 and 2 (4:2:0 chroma has two edges per direction).
//
// Luma[d][k] and Chroma[p][d][k] are the thresholds of direction d for plane p
// (0 Cb, 1 Cr): k = 0 for the edge shared with the neighbouring macroblock (the
// average QP of the two macroblocks), k = 1 for the inner edges (the
// macroblock's own QP).
type MBEdges struct {
	BS     [2][4][4]uint8
	Luma   [2][2]Thresholds
	Chroma [2][2][2]Thresholds
}

// DeblockMB filters the edges of one macroblock in the order of clause 8.7:
// the luma vertical edges left to right, then the luma horizontal edges top to
// bottom, and the same for Cb and Cr, whose edges are 0 and 2 (chroma samples 0
// and 4). yOff is the first luma sample of the macroblock in y and cOff the
// first chroma sample in cb and cr. The result is that of the per-edge calls
// (DeblockLumaV, DeblockLumaIntraV, DeblockChromaPairV and their H forms) in
// that order, with the bounds, index and strength proofs made once for the
// macroblock instead of once per edge.
//
// The proof is a region proof, at least as strong as the per-edge proofs it
// replaces: the luma window is the 16x16 macroblock plus 4 columns to the left
// when vertical edge 0 is filtered and 4 rows above when horizontal edge 0 is,
// and the chroma windows are 8x8 plus 2 and 2 likewise; a window that does not
// fit its slice is refused even where one edge's own footprint would fit.
// DeblockMB panics, before touching a plane, when a window does not fit, when
// yStride < 16 or cStride < 8, when cb and cr windows share memory, when an
// index is outside 0..51 or a strength pattern is not allowed. Planes are
// checked only when one of their edges is filtered.
func DeblockMB(y []byte, yOff, yStride int, cb, cr []byte, cOff, cStride int, e *MBEdges) {
	// 0: not filtered, 1: normal, 2: strong
	var kind [2][4]uint8
	for d := 0; d < 2; d++ {
		for k := 0; k < 4; k++ {
			n4, nz := 0, false
			for _, s := range e.BS[d][k] {
				switch {
				case s > 4:
					panic(fmt.Sprintf("lf: bS %d outside 0..4", s))
				case s == 4:
					n4++
				}
				nz = nz || s != 0
			}
			switch {
			case n4 == 0 && nz:
				kind[d][k] = 1
			case n4 == 4 && k == 0:
				kind[d][k] = 2
			case n4 != 0:
				panic(fmt.Sprintf("lf: strengths %v of edge %d, direction %d: bS 4 needs all four groups on edge 0", e.BS[d][k], k, d))
			}
		}
	}

	lumaActive, chromaActive := false, false
	for d := 0; d < 2; d++ {
		for k := 0; k < 4; k++ {
			if kind[d][k] != 0 {
				lumaActive = true
				if k&1 == 0 {
					chromaActive = true
				}
			}
		}
	}
	if lumaActive {
		for d := 0; d < 2; d++ {
			for _, t := range e.Luma[d] {
				checkIndex("indexA", int(t.IndexA))
				checkIndex("indexB", int(t.IndexB))
			}
		}
		if yStride < 16 {
			panic(fmt.Sprintf("lf: stride %d is smaller than the 16 lines of an edge", yStride))
		}
		top, left := 0, 0
		if kind[1][0] != 0 {
			top = 4
		}
		if kind[0][0] != 0 {
			left = 4
		}
		if _, _, ok := bounds.Window(len(y), yOff, yStride, top, 15, left, 15); !ok {
			panic(fmt.Sprintf("lf: macroblock window does not fit: offset %d stride %d length %d", yOff, yStride, len(y)))
		}
	}
	if chromaActive {
		for p := 0; p < 2; p++ {
			for d := 0; d < 2; d++ {
				for _, t := range e.Chroma[p][d] {
					checkIndex("indexA", int(t.IndexA))
					checkIndex("indexB", int(t.IndexB))
				}
			}
		}
		if cStride < 8 {
			panic(fmt.Sprintf("lf: chroma stride %d is smaller than the 8 lines of a macroblock", cStride))
		}
		top, left := 0, 0
		if kind[1][0] != 0 {
			top = 2
		}
		if kind[0][0] != 0 {
			left = 2
		}
		for _, pl := range [2][]byte{cb, cr} {
			if _, _, ok := bounds.Window(len(pl), cOff, cStride, top, 7, left, 7); !ok {
				panic(fmt.Sprintf("lf: chroma macroblock window does not fit: offset %d stride %d length %d", cOff, cStride, len(pl)))
			}
		}
		first, last := cOff-top*cStride-left, cOff+7*cStride+7
		pa := uintptr(unsafe.Pointer(unsafe.SliceData(cb)))
		pb := uintptr(unsafe.Pointer(unsafe.SliceData(cr)))
		if pa+uintptr(first) <= pb+uintptr(last) && pb+uintptr(first) <= pa+uintptr(last) {
			panic(fmt.Sprintf("lf: the Cb and Cr windows share memory (offset %d, stride %d)", cOff, cStride))
		}
	}

	for d := 0; d < 2; d++ {
		vertical := d == 0
		for k := 0; k < 4; k++ {
			if kind[d][k] == 0 {
				continue
			}
			th := e.Luma[d][min(k, 1)]
			off := yOff + 4*k
			if !vertical {
				off = yOff + 4*k*yStride
			}
			if kind[d][k] == 2 {
				deblockStrong(y, off, yStride, vertical, int(th.IndexA), int(th.IndexB))
			} else {
				deblockNormal(y, off, yStride, vertical, int(th.IndexA), int(th.IndexB), e.BS[d][k])
			}
		}
		for k := 0; k < 4; k += 2 {
			if kind[d][k] == 0 {
				continue
			}
			cbt, crt := e.Chroma[0][d][min(k, 1)], e.Chroma[1][d][min(k, 1)]
			off := cOff + 2*k
			if !vertical {
				off = cOff + 2*k*cStride
			}
			if kind[d][k] == 2 {
				deblockChromaPairStrong(cb, cr, off, cStride, vertical, int(cbt.IndexA), int(cbt.IndexB), int(crt.IndexA), int(crt.IndexB))
			} else {
				deblockChromaPairNormal(cb, cr, off, cStride, vertical, int(cbt.IndexA), int(cbt.IndexB), int(crt.IndexA), int(crt.IndexB), e.BS[d][k])
			}
		}
	}
}
