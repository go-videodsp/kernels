//go:build amd64 && !purego

package itx

import "github.com/go-videodsp/kernels/cpu"

//go:noescape
func idct8AVX2(dst *byte, stride int, coef, scale *[64]int32, sl, rnd, sr int)

var idct8Impl = dequant8x8IDCTAddRef

func init() {
	if cpu.HasAVX2 {
		idct8Impl = dequant8x8IDCTAddAVX2
	}
}

func dequant8x8IDCTAddDispatch(dst []byte, off, stride int, coef, scale *[64]int32, qpDiv int) {
	idct8Impl(dst, off, stride, coef, scale, qpDiv)
}

func dequant8x8IDCTAddAVX2(dst []byte, off, stride int, coef, scale *[64]int32, qpDiv int) {
	sl, rnd, sr := shifts(qpDiv)
	idct8AVX2(&dst[off], stride, coef, scale, sl, rnd, sr)
}
