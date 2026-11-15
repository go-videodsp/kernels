//go:build arm64 && !purego

package lf

// Cb and Cr in one pass: the eight lines of each plane share a register, with
// each plane's own thresholds and tC0 in its half.

//go:noescape
func deblockChromaPairNormalH(cb, cr *byte, stride int, alphaCb, betaCb, alphaCr, betaCr int, tc0Cb, tc0Cr, mask uint32)

//go:noescape
func deblockChromaPairNormalV(cb, cr *byte, stride int, alphaCb, betaCb, alphaCr, betaCr int, tc0Cb, tc0Cr, mask uint32)

//go:noescape
func deblockChromaPairStrongV(cb, cr *byte, stride int, alphaCb, betaCb, alphaCr, betaCr int)

// strengths returns the packed tC0 of one plane (one byte per group of two
// lines) and the mask of the groups whose bS is not 0.
func strengths(indexA int, bS [4]uint8) (tc0, mask uint32) {
	for g, s := range bS {
		if s != 0 {
			tc0 |= uint32(tc0Table[indexA][s-1]) << (8 * g)
			mask |= 0xff << (8 * g)
		}
	}
	return
}

func deblockChromaPairNormal(cb, cr []byte, off, stride int, vertical bool, indexACb, indexBCb, indexACr, indexBCr int, bS [4]uint8) {
	aCb, bCb := int(alphaTable[indexACb]), int(betaTable[indexBCb])
	aCr, bCr := int(alphaTable[indexACr]), int(betaTable[indexBCr])
	tcCb, mask := strengths(indexACb, bS)
	tcCr, _ := strengths(indexACr, bS)
	// A plane whose alpha or beta is 0 filters nothing (8-460 is strict), and
	// so does a strength of 0 on every group; both planes idle: no call.
	if mask == 0 || (aCb == 0 || bCb == 0) && (aCr == 0 || bCr == 0) {
		return
	}
	if vertical {
		deblockChromaPairNormalV(&cb[off], &cr[off], stride, aCb, bCb, aCr, bCr, tcCb, tcCr, mask)
	} else {
		deblockChromaPairNormalH(&cb[off], &cr[off], stride, aCb, bCb, aCr, bCr, tcCb, tcCr, mask)
	}
}

func deblockChromaPairStrong(cb, cr []byte, off, stride int, vertical bool, indexACb, indexBCb, indexACr, indexBCr int) {
	aCb, bCb := int(alphaTable[indexACb]), int(betaTable[indexBCb])
	aCr, bCr := int(alphaTable[indexACr]), int(betaTable[indexBCr])
	if (aCb == 0 || bCb == 0) && (aCr == 0 || bCr == 0) {
		return
	}
	if vertical {
		deblockChromaPairStrongV(&cb[off], &cr[off], stride, aCb, bCb, aCr, bCr)
		return
	}
	// Two one-plane calls measured faster than a two-plane kernel here.
	deblockChromaStrongHOne(cb, off, stride, indexACb, indexBCb)
	deblockChromaStrongHOne(cr, off, stride, indexACr, indexBCr)
}

// deblockChromaStrongHOne is the one-plane horizontal strong kernel with its
// own early exit.
func deblockChromaStrongHOne(pix []byte, off, stride, indexA, indexB int) {
	alpha, beta := int(alphaTable[indexA]), int(betaTable[indexB])
	if alpha == 0 || beta == 0 {
		return
	}
	deblockChromaStrongH(&pix[off], stride, alpha, beta)
}
