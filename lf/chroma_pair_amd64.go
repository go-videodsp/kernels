//go:build amd64 && !purego

package lf

// The pair API on amd64 is two one-plane calls to the AVX2 kernels (no
// two-plane kernel: the arm64 one packs both planes into one 128-bit register
// of sixteen lanes; here the one-plane kernels already use an XMM of eight
// lanes and the call overhead, not the arithmetic, is what a pair kernel
// would save).

func deblockChromaPairNormal(cb, cr []byte, off, stride int, vertical bool, indexACb, indexBCb, indexACr, indexBCr int, bS [4]uint8) {
	deblockChromaNormal(cb, off, stride, vertical, indexACb, indexBCb, bS)
	deblockChromaNormal(cr, off, stride, vertical, indexACr, indexBCr, bS)
}

func deblockChromaPairStrong(cb, cr []byte, off, stride int, vertical bool, indexACb, indexBCb, indexACr, indexBCr int) {
	deblockChromaStrong(cb, off, stride, vertical, indexACb, indexBCb)
	deblockChromaStrong(cr, off, stride, vertical, indexACr, indexBCr)
}
