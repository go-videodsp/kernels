package lf

import (
	"fmt"

	"github.com/go-videodsp/kernels/internal/bounds"
)

// DeblockChromaV filters one vertical chroma edge of 8 lines (a 4:2:0
// macroblock edge) with boundary strength below 4: clause 8.7.2.3 with
// chromaStyleFilteringFlag 1 (Equations 8-466 to 8-469; only p0 and q0
// change), after the thresholds of 8.7.2.2. pix[off] is q0 of the first
// line, the sample just right of the edge; line k is row k, so
// p_i = pix[off + k*stride - i - 1] and q_i = pix[off + k*stride + i]
// (Equations 8-442, 8-443). bS holds the strength of each group of two lines
// (the chroma lines whose luma lines share a strength, clause 8.7.2), 0 to 3;
// indexA and indexB are those of Equations 8-454 and 8-455 computed with the
// chroma QP of each macroblock (clause 8.7.2): QPc of Table 8-15 (clause
// 8.5.8) at qPI = Clip3(-QpBdOffsetC, 51, QPY + qPOffset), Equation 8-311,
// with qPOffset = chroma_qp_index_offset for Cb (8-309) and
// second_chroma_qp_index_offset for Cr (8-310), so Cb and Cr can have
// different indexes (and I_PCM macroblocks use the QPc of QPY 0).
//
// The footprint is columns -2 .. +1 around the edge on all 8 lines. It
// panics, before touching pix, when the footprint does not fit, when
// stride < 8, when an index is outside 0..51 or a strength outside 0..3.
func DeblockChromaV(pix []byte, off, stride, indexA, indexB int, bS [4]uint8) {
	checkChroma(len(pix), off, stride, true, indexA, indexB)
	checkChromaStrength(bS)
	deblockChromaNormal(pix, off, stride, true, indexA, indexB, bS)
}

// DeblockChromaH is DeblockChromaV for a horizontal edge: pix[off] is q0 of
// the first line, the sample just below the edge, and line k is column k
// (Equations 8-444, 8-445). The footprint is rows -2 .. +1 on 8 columns.
func DeblockChromaH(pix []byte, off, stride, indexA, indexB int, bS [4]uint8) {
	checkChroma(len(pix), off, stride, false, indexA, indexB)
	checkChromaStrength(bS)
	deblockChromaNormal(pix, off, stride, false, indexA, indexB, bS)
}

// DeblockChromaIntraV filters one vertical chroma edge of 8 lines with
// boundary strength 4 (clause 8.7.2.4 with chromaStyleFilteringFlag 1,
// Equations 8-480 and 8-487). Geometry, footprint and panics are
// DeblockChromaV's.
func DeblockChromaIntraV(pix []byte, off, stride, indexA, indexB int) {
	checkChroma(len(pix), off, stride, true, indexA, indexB)
	deblockChromaStrong(pix, off, stride, true, indexA, indexB)
}

// DeblockChromaIntraH is DeblockChromaIntraV for a horizontal edge.
func DeblockChromaIntraH(pix []byte, off, stride, indexA, indexB int) {
	checkChroma(len(pix), off, stride, false, indexA, indexB)
	deblockChromaStrong(pix, off, stride, false, indexA, indexB)
}

func checkChromaStrength(bS [4]uint8) {
	for _, s := range bS {
		if s > 3 {
			panic(fmt.Sprintf("lf: bS %d is not below 4; use the Intra form for bS 4", s))
		}
	}
}

// checkChroma is the bounds proof (internal/bounds.Window: no product can wrap
// on any target). Lines of a horizontal edge are 8 adjacent columns, hence
// stride >= 8 for both directions.
func checkChroma(n, off, stride int, vertical bool, indexA, indexB int) {
	checkIndex("indexA", indexA)
	checkIndex("indexB", indexB)
	if stride < 8 {
		panic(fmt.Sprintf("lf: stride %d is smaller than the 8 lines of a chroma edge", stride))
	}
	var ok bool
	if vertical {
		_, _, ok = bounds.Window(n, off, stride, 0, 7, 2, 1)
	} else {
		_, _, ok = bounds.Window(n, off, stride, 2, 1, 0, 7)
	}
	if !ok {
		panic(fmt.Sprintf("lf: chroma edge footprint does not fit: offset %d stride %d vertical %v length %d", off, stride, vertical, n))
	}
}
