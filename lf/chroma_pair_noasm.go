//go:build !arm64 || purego

package lf

func deblockChromaPairNormal(cb, cr []byte, off, stride int, vertical bool, indexA, indexB int, bS [4]uint8) {
	deblockChromaNormalRef(cb, off, stride, vertical, indexA, indexB, bS)
	deblockChromaNormalRef(cr, off, stride, vertical, indexA, indexB, bS)
}

func deblockChromaPairStrong(cb, cr []byte, off, stride int, vertical bool, indexA, indexB int) {
	deblockChromaStrongRef(cb, off, stride, vertical, indexA, indexB)
	deblockChromaStrongRef(cr, off, stride, vertical, indexA, indexB)
}
