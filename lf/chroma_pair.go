package lf

import (
	"fmt"
	"unsafe"
)

// DeblockChromaPairV filters the same vertical edge of the Cb and the Cr
// plane in one call: DeblockChromaV on cb and on cr, with the same off,
// stride and strengths bS (chroma takes the strength of the luma edge, which
// is the same for both planes) and each plane's own indexA and indexB. They
// differ between the planes when second_chroma_qp_index_offset differs from
// chroma_qp_index_offset (clause 8.5.8, Table 8-15, Equations 8-309 to 8-311),
// which is why the indexes are per plane. The call pays the Go-side checks,
// the dispatch and the constant setup once.
//
// cb and cr must not share memory: the call panics when the bytes of the two
// footprints intersect (decided from the addresses). Other panics, and the
// footprint, are DeblockChromaV's, for either plane.
func DeblockChromaPairV(cb, cr []byte, off, stride, indexACb, indexBCb, indexACr, indexBCr int, bS [4]uint8) {
	checkChromaPair(cb, cr, off, stride, true, indexACb, indexBCb, indexACr, indexBCr)
	checkChromaStrength(bS)
	deblockChromaPairNormal(cb, cr, off, stride, true, indexACb, indexBCb, indexACr, indexBCr, bS)
}

// DeblockChromaPairH is DeblockChromaPairV for a horizontal edge.
func DeblockChromaPairH(cb, cr []byte, off, stride, indexACb, indexBCb, indexACr, indexBCr int, bS [4]uint8) {
	checkChromaPair(cb, cr, off, stride, false, indexACb, indexBCb, indexACr, indexBCr)
	checkChromaStrength(bS)
	deblockChromaPairNormal(cb, cr, off, stride, false, indexACb, indexBCb, indexACr, indexBCr, bS)
}

// DeblockChromaPairIntraV is the bS 4 form for both planes.
func DeblockChromaPairIntraV(cb, cr []byte, off, stride, indexACb, indexBCb, indexACr, indexBCr int) {
	checkChromaPair(cb, cr, off, stride, true, indexACb, indexBCb, indexACr, indexBCr)
	deblockChromaPairStrong(cb, cr, off, stride, true, indexACb, indexBCb, indexACr, indexBCr)
}

// DeblockChromaPairIntraH is the bS 4 form for both planes.
func DeblockChromaPairIntraH(cb, cr []byte, off, stride, indexACb, indexBCb, indexACr, indexBCr int) {
	checkChromaPair(cb, cr, off, stride, false, indexACb, indexBCb, indexACr, indexBCr)
	deblockChromaPairStrong(cb, cr, off, stride, false, indexACb, indexBCb, indexACr, indexBCr)
}

// checkChromaPair proves both footprints and that they share no memory.
func checkChromaPair(cb, cr []byte, off, stride int, vertical bool, indexACb, indexBCb, indexACr, indexBCr int) {
	checkChroma(len(cb), off, stride, vertical, indexACb, indexBCb)
	checkChroma(len(cr), off, stride, vertical, indexACr, indexBCr)
	var first, last int
	if vertical {
		first, last = off-2, off+7*stride+1
	} else {
		first, last = off-2*stride, off+stride+7
	}
	pa := uintptr(unsafe.Pointer(unsafe.SliceData(cb)))
	pb := uintptr(unsafe.Pointer(unsafe.SliceData(cr)))
	if pa+uintptr(first) <= pb+uintptr(last) && pb+uintptr(first) <= pa+uintptr(last) {
		panic(fmt.Sprintf("lf: the Cb and Cr footprints share memory (offset %d, stride %d)", off, stride))
	}
}
