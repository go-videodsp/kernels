//go:build (!arm64 && !amd64) || purego

package lf

func deblockNormal(pix []byte, off, stride int, vertical bool, indexA, indexB int, bS [4]uint8) {
	deblockNormalRef(pix, off, stride, vertical, indexA, indexB, bS)
}

func deblockChromaNormal(pix []byte, off, stride int, vertical bool, indexA, indexB int, bS [4]uint8) {
	deblockChromaNormalRef(pix, off, stride, vertical, indexA, indexB, bS)
}

func deblockChromaStrong(pix []byte, off, stride int, vertical bool, indexA, indexB int) {
	deblockChromaStrongRef(pix, off, stride, vertical, indexA, indexB)
}

func deblockStrong(pix []byte, off, stride int, vertical bool, indexA, indexB int) {
	deblockStrongRef(pix, off, stride, vertical, indexA, indexB)
}
