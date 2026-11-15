//go:build arm64 && !purego

package lf

// The kernels filter sixteen lines. pix points at q0 of the first line; the
// assembly reads the eight samples p3..q3 of every line (a horizontal edge
// loads eight rows, a vertical edge sixteen rows of eight bytes and
// transposes). tc0 and mask hold one byte per group of four lines: t'C0 and
// 0xff when the group's bS is not 0.

//go:noescape
func deblockNormalH(pix *byte, stride int, alpha, beta int, tc0, mask uint32)

//go:noescape
func deblockNormalV(pix *byte, stride int, alpha, beta int, tc0, mask uint32)

//go:noescape
func deblockStrongH(pix *byte, stride int, alpha, beta, thr int)

//go:noescape
func deblockStrongV(pix *byte, stride int, alpha, beta, thr int)

func deblockNormal(pix []byte, off, stride int, vertical bool, indexA, indexB int, bS [4]uint8) {
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

func deblockStrong(pix []byte, off, stride int, vertical bool, indexA, indexB int) {
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
