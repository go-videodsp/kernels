package lf

// DeblockChromaPairV filters the same vertical edge of the Cb and the Cr
// plane in one call: the two edges of a 4:2:0 macroblock share bS, indexA and
// indexB only through the chroma QPs of Equations 8-454 and 8-455 (QPc is per
// component, so a caller whose Cb and Cr indexes differ calls DeblockChromaV
// twice). Each plane is DeblockChromaV on cb and cr with the same off, stride,
// indexes and strengths; the call pays the Go-side checks, the dispatch and
// the constant setup once. cb and cr must not share memory.
func DeblockChromaPairV(cb, cr []byte, off, stride, indexA, indexB int, bS [4]uint8) {
	checkChroma(len(cb), off, stride, true, indexA, indexB)
	checkChroma(len(cr), off, stride, true, indexA, indexB)
	checkChromaStrength(bS)
	deblockChromaPairNormal(cb, cr, off, stride, true, indexA, indexB, bS)
}

// DeblockChromaPairH is DeblockChromaPairV for a horizontal edge.
func DeblockChromaPairH(cb, cr []byte, off, stride, indexA, indexB int, bS [4]uint8) {
	checkChroma(len(cb), off, stride, false, indexA, indexB)
	checkChroma(len(cr), off, stride, false, indexA, indexB)
	checkChromaStrength(bS)
	deblockChromaPairNormal(cb, cr, off, stride, false, indexA, indexB, bS)
}

// DeblockChromaPairIntraV is the bS 4 form for both planes.
func DeblockChromaPairIntraV(cb, cr []byte, off, stride, indexA, indexB int) {
	checkChroma(len(cb), off, stride, true, indexA, indexB)
	checkChroma(len(cr), off, stride, true, indexA, indexB)
	deblockChromaPairStrong(cb, cr, off, stride, true, indexA, indexB)
}

// DeblockChromaPairIntraH is the bS 4 form for both planes.
func DeblockChromaPairIntraH(cb, cr []byte, off, stride, indexA, indexB int) {
	checkChroma(len(cb), off, stride, false, indexA, indexB)
	checkChroma(len(cr), off, stride, false, indexA, indexB)
	deblockChromaPairStrong(cb, cr, off, stride, false, indexA, indexB)
}
