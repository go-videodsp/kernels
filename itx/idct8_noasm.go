//go:build (!arm64 && !amd64) || purego

package itx

func dequant8x8IDCTAddDispatch(dst []byte, off, stride int, coef, scale *[64]int32, qpDiv int) {
	dequant8x8IDCTAddRef(dst, off, stride, coef, scale, qpDiv)
}
