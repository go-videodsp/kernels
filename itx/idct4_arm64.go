//go:build arm64 && !purego

package itx

//go:noescape
func idct4NEON(dst *byte, offs *[16]int, coef *[16][16]int32, scale *[16]int32, stride, sl, rnd, sr int, mask, skipDC uint)

//go:noescape
func lumaDC4NEON(dc *[16]int32, v, sl, rnd, sr int)

func dequant4x4IDCTAddBlocksDispatch(dst []byte, off, stride int, coef *[16][16]int32, mask uint16, scale *[16]int32, qpDiv int, skipDC bool) {
	var offs [16]int
	for b := range offs {
		offs[b] = off + blockY[b]*stride + blockX[b]
	}
	sl, rnd, sr := shifts4(qpDiv)
	skip := uint(0)
	if skipDC {
		skip = 1
	}
	idct4NEON(&dst[0], &offs, coef, scale, stride, sl, rnd, sr, uint(mask), skip)
}

func lumaDCDequantDispatch(dc *[16]int32, ls0 int32, qpDiv int) {
	sl, rnd, sr := shifts(qpDiv)
	lumaDC4NEON(dc, int(ls0), sl, rnd, sr)
}
