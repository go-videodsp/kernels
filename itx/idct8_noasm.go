//go:build (!arm64 && !amd64) || purego

package itx

func dequant8x8IDCTAddDispatch(dst []byte, off, stride int, coef, scale *[64]int32, qpDiv int) {
	dequant8x8IDCTAddRef(dst, off, stride, coef, scale, qpDiv)
}

func dequant4x4IDCTAddBlocksDispatch(dst []byte, off, stride int, coef *[16][16]int32, mask uint16, scale *[16]int32, qpDiv int, skipDC bool) {
	dequant4x4IDCTAddBlocksRef(dst, off, stride, coef, mask, scale, qpDiv, skipDC)
}

func lumaDCDequantDispatch(dc *[16]int32, ls0 int32, qpDiv int) { lumaDCDequantRef(dc, ls0, qpDiv) }
