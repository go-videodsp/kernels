//go:build amd64 && !purego

package itx

import "github.com/go-videodsp/kernels/cpu"

//go:noescape
func idct4AVX2(dst *byte, offs *[16]int, coef *[16][16]int32, scale *[16]int32, stride, sl, rnd, sr int, mask, skipDC uint)

//go:noescape
func lumaDC4AVX2(dc *[16]int32, v, sl, rnd, sr int)

var (
	idct4Impl  = dequant4x4IDCTAddBlocksRef
	lumaDCImpl = lumaDCDequantRef
)

func init() {
	if cpu.HasAVX2 {
		idct4Impl = dequant4x4IDCTAddBlocksAVX2
		lumaDCImpl = lumaDCDequantAVX2
	}
}

func dequant4x4IDCTAddBlocksDispatch(dst []byte, off, stride int, coef *[16][16]int32, mask uint16, scale *[16]int32, qpDiv int, skipDC bool) {
	idct4Impl(dst, off, stride, coef, mask, scale, qpDiv, skipDC)
}

func lumaDCDequantDispatch(dc *[16]int32, ls0 int32, qpDiv int) { lumaDCImpl(dc, ls0, qpDiv) }

func dequant4x4IDCTAddBlocksAVX2(dst []byte, off, stride int, coef *[16][16]int32, mask uint16, scale *[16]int32, qpDiv int, skipDC bool) {
	var offs [16]int
	for b := range offs {
		offs[b] = off + blockY[b]*stride + blockX[b]
	}
	sl, rnd, sr := shifts4(qpDiv)
	skip := uint(0)
	if skipDC {
		skip = 1
	}
	idct4AVX2(&dst[0], &offs, coef, scale, stride, sl, rnd, sr, uint(mask), skip)
}

func lumaDCDequantAVX2(dc *[16]int32, ls0 int32, qpDiv int) {
	sl, rnd, sr := shifts(qpDiv)
	lumaDC4AVX2(dc, int(ls0), sl, rnd, sr)
}
