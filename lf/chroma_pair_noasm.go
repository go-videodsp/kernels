//go:build (!arm64 && !amd64) || purego

package lf

func deblockChromaPairNormal(cb, cr []byte, off, stride int, vertical bool, indexACb, indexBCb, indexACr, indexBCr int, bS [4]uint8) {
	deblockChromaNormalRef(cb, off, stride, vertical, indexACb, indexBCb, bS)
	deblockChromaNormalRef(cr, off, stride, vertical, indexACr, indexBCr, bS)
}

func deblockChromaPairStrong(cb, cr []byte, off, stride int, vertical bool, indexACb, indexBCb, indexACr, indexBCr int) {
	deblockChromaStrongRef(cb, off, stride, vertical, indexACb, indexBCb)
	deblockChromaStrongRef(cr, off, stride, vertical, indexACr, indexBCr)
}
