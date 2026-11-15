//go:build amd64 && !purego

package lf

import "github.com/go-videodsp/kernels/cpu"

// Eight lines per call; tc0 and mask hold one byte per group of two lines: tC
// (t'C0 + 1, Equation 8-466) and 0xff when the group's bS is not 0.

var (
	deblockChromaNormalImpl = deblockChromaNormalRef
	deblockChromaStrongImpl = deblockChromaStrongRef
)

func init() {
	if cpu.HasAVX2 {
		deblockChromaNormalImpl = deblockChromaNormalAVX2
		deblockChromaStrongImpl = deblockChromaStrongAVX2
	}
}

func deblockChromaNormal(pix []byte, off, stride int, vertical bool, indexA, indexB int, bS [4]uint8) {
	deblockChromaNormalImpl(pix, off, stride, vertical, indexA, indexB, bS)
}

func deblockChromaStrong(pix []byte, off, stride int, vertical bool, indexA, indexB int) {
	deblockChromaStrongImpl(pix, off, stride, vertical, indexA, indexB)
}

func deblockChromaNormalAVX2(pix []byte, off, stride int, vertical bool, indexA, indexB int, bS [4]uint8) {
	alpha, beta := int(alphaTable[indexA]), int(betaTable[indexB])
	var tc, mask uint32
	for g, s := range bS {
		if s != 0 {
			tc |= (uint32(tc0Table[indexA][s-1]) + 1) << (8 * g)
			mask |= 0xff << (8 * g)
		}
	}
	if alpha == 0 || beta == 0 || mask == 0 {
		return
	}
	if vertical {
		deblockChromaNormalV(&pix[off], stride, alpha, beta, tc, mask)
	} else {
		deblockChromaNormalH(&pix[off], stride, alpha, beta, tc, mask)
	}
}

func deblockChromaStrongAVX2(pix []byte, off, stride int, vertical bool, indexA, indexB int) {
	alpha, beta := int(alphaTable[indexA]), int(betaTable[indexB])
	if alpha == 0 || beta == 0 {
		return
	}
	if vertical {
		deblockChromaStrongV(&pix[off], stride, alpha, beta)
	} else {
		deblockChromaStrongH(&pix[off], stride, alpha, beta)
	}
}
