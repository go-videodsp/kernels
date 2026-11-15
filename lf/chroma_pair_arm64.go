//go:build arm64 && !purego

package lf

// Cb and Cr in one pass: the eight lines of each plane share a register.

//go:noescape
func deblockChromaPairNormalH(cb, cr *byte, stride int, alpha, beta int, tc0, mask uint32)

//go:noescape
func deblockChromaPairNormalV(cb, cr *byte, stride int, alpha, beta int, tc0, mask uint32)

//go:noescape
func deblockChromaPairStrongV(cb, cr *byte, stride int, alpha, beta int)

func deblockChromaPairNormal(cb, cr []byte, off, stride int, vertical bool, indexA, indexB int, bS [4]uint8) {
	alpha, beta := int(alphaTable[indexA]), int(betaTable[indexB])
	var tc0, mask uint32
	for g, s := range bS {
		if s != 0 {
			tc0 |= uint32(tc0Table[indexA][s-1]) << (8 * g)
			mask |= 0xff << (8 * g)
		}
	}
	if alpha == 0 || beta == 0 || mask == 0 {
		return
	}
	if vertical {
		deblockChromaPairNormalV(&cb[off], &cr[off], stride, alpha, beta, tc0, mask)
	} else {
		deblockChromaPairNormalH(&cb[off], &cr[off], stride, alpha, beta, tc0, mask)
	}
}

func deblockChromaPairStrong(cb, cr []byte, off, stride int, vertical bool, indexA, indexB int) {
	alpha, beta := int(alphaTable[indexA]), int(betaTable[indexB])
	if alpha == 0 || beta == 0 {
		return
	}
	if vertical {
		deblockChromaPairStrongV(&cb[off], &cr[off], stride, alpha, beta)
		return
	}
	// Two one-plane calls measured faster than a two-plane kernel here.
	deblockChromaStrongH(&cb[off], stride, alpha, beta)
	deblockChromaStrongH(&cr[off], stride, alpha, beta)
}
