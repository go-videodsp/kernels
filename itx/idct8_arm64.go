//go:build arm64 && !purego

package itx

//go:noescape
func idct8NEON(dst *byte, stride int, coef, scale *[64]int32, sl, rnd, sr int)

func dequant8x8IDCTAddDispatch(dst []byte, off, stride int, coef, scale *[64]int32, qpDiv int) {
	sl, rnd, sr := shifts(qpDiv)
	idct8NEON(&dst[off], stride, coef, scale, sl, rnd, sr)
}
