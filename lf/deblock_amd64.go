//go:build amd64 && !purego

package lf

import "github.com/go-videodsp/kernels/cpu"

// The kernels filter sixteen lines. pix points at q0 of the first line. A
// horizontal edge reads eight rows of sixteen samples, a vertical edge
// sixteen rows of eight bytes (transposed inside the kernel). tc0 and mask
// hold one byte per group of four lines: t'C0 and 0xff when the group's bS is
// not 0.

var (
	deblockNormalImpl = deblockNormalRef
	deblockStrongImpl = deblockStrongRef
)

func init() {
	if cpu.HasAVX2 {
		deblockNormalImpl = deblockNormalAVX2
		deblockStrongImpl = deblockStrongAVX2
	}
}

func deblockNormal(pix []byte, off, stride int, vertical bool, indexA, indexB int, bS [4]uint8) {
	deblockNormalImpl(pix, off, stride, vertical, indexA, indexB, bS)
}

func deblockStrong(pix []byte, off, stride int, vertical bool, indexA, indexB int) {
	deblockStrongImpl(pix, off, stride, vertical, indexA, indexB)
}

func deblockNormalAVX2(pix []byte, off, stride int, vertical bool, indexA, indexB int, bS [4]uint8) {
	alpha, beta := int(alphaTable[indexA]), int(betaTable[indexB])
	var tc0, mask uint32
	for g, s := range bS {
		if s != 0 {
			tc0 |= uint32(tc0Table[indexA][s-1]) << (8 * g)
			mask |= 0xff << (8 * g)
		}
	}
	// No line can change: alpha or beta of 0 fails 8-460's strict tests, and a
	// strength of 0 on every group skips every line.
	if alpha == 0 || beta == 0 || mask == 0 {
		return
	}
	if vertical {
		deblockNormalV(&pix[off], stride, alpha, beta, tc0, mask)
	} else {
		deblockNormalH(&pix[off], stride, alpha, beta, tc0, mask)
	}
}

func deblockStrongAVX2(pix []byte, off, stride int, vertical bool, indexA, indexB int) {
	alpha, beta := int(alphaTable[indexA]), int(betaTable[indexB])
	if alpha == 0 || beta == 0 {
		return
	}
	if vertical {
		deblockStrongV(&pix[off], stride, alpha, beta, alpha>>2+2)
	} else {
		deblockStrongH(&pix[off], stride, alpha, beta, alpha>>2+2)
	}
}
