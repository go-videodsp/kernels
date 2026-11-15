//go:build arm64 && !purego

package lf

// Eight lines per call; tc0 and mask hold one byte per group of two lines.

//go:noescape
func deblockChromaNormalH(pix *byte, stride int, alpha, beta int, tc0, mask uint32)

//go:noescape
func deblockChromaNormalV(pix *byte, stride int, alpha, beta int, tc0, mask uint32)

//go:noescape
func deblockChromaStrongH(pix *byte, stride int, alpha, beta int)

func deblockChromaNormal(pix []byte, off, stride int, vertical bool, indexA, indexB int, bS [4]uint8) {
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
		deblockChromaNormalV(&pix[off], stride, alpha, beta, tc0, mask)
	} else {
		deblockChromaNormalH(&pix[off], stride, alpha, beta, tc0, mask)
	}
}

func deblockChromaStrong(pix []byte, off, stride int, vertical bool, indexA, indexB int) {
	alpha, beta := int(alphaTable[indexA]), int(betaTable[indexB])
	if alpha == 0 || beta == 0 {
		return
	}
	if vertical {
		// A NEON kernel was measured against this and tied: the transposes
		// cost what the arithmetic saves on eight lines.
		deblockChromaStrongRef(pix, off, stride, true, indexA, indexB)
		return
	}
	deblockChromaStrongH(&pix[off], stride, alpha, beta)
}
