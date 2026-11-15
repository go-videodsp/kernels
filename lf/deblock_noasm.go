//go:build !arm64 || purego

package lf

func deblockNormal(pix []byte, off, stride int, vertical bool, indexA, indexB int, bS [4]uint8) {
	deblockNormalRef(pix, off, stride, vertical, indexA, indexB, bS)
}

func deblockStrong(pix []byte, off, stride int, vertical bool, indexA, indexB int) {
	deblockStrongRef(pix, off, stride, vertical, indexA, indexB)
}
