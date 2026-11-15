//go:build arm64 && !purego

package lf

// Cb and Cr in one pass: the eight lines of each plane share a register, with
// each plane's own thresholds in its half. Only the strong vertical edge has
// such a kernel: the others were written, measured against two one-plane
// calls and not kept (normal vertical: a tie; normal horizontal: a gain inside
// the run-to-run noise; strong horizontal: a loss).

//go:noescape
func deblockChromaPairStrongV(cb, cr *byte, stride int, alphaCb, betaCb, alphaCr, betaCr int)

func deblockChromaPairNormal(cb, cr []byte, off, stride int, vertical bool, indexACb, indexBCb, indexACr, indexBCr int, bS [4]uint8) {
	deblockChromaNormal(cb, off, stride, vertical, indexACb, indexBCb, bS)
	deblockChromaNormal(cr, off, stride, vertical, indexACr, indexBCr, bS)
}

func deblockChromaPairStrong(cb, cr []byte, off, stride int, vertical bool, indexACb, indexBCb, indexACr, indexBCr int) {
	aCb, bCb := int(alphaTable[indexACb]), int(betaTable[indexBCb])
	aCr, bCr := int(alphaTable[indexACr]), int(betaTable[indexBCr])
	// A plane whose alpha or beta is 0 filters nothing (8-460 is strict): both
	// planes idle, no call.
	if (aCb == 0 || bCb == 0) && (aCr == 0 || bCr == 0) {
		return
	}
	if vertical {
		deblockChromaPairStrongV(&cb[off], &cr[off], stride, aCb, bCb, aCr, bCr)
		return
	}
	deblockChromaStrong(cb, off, stride, false, indexACb, indexBCb)
	deblockChromaStrong(cr, off, stride, false, indexACr, indexBCr)
}
