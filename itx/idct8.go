package itx

import (
	"fmt"

	"github.com/go-videodsp/kernels/internal/bounds"
)

//go:generate go run ../internal/genitx ..

// Dequant8x8IDCTAdd scales an 8x8 block of residual coefficients, transforms
// it and adds the result to the prediction already in dst, in place (ITU-T
// H.264 (V16) (06/2026), clauses 8.5.9, 8.5.13 and 8.5.14, for 8-bit samples):
//
//	d[i]  = coef[i] * scale[i] << (qpDiv - 6)                    qpDiv >= 6
//	d[i]  = (coef[i] * scale[i] + 2^(5 - qpDiv)) >> (6 - qpDiv)  qpDiv < 6
//	r     = ((inverse transform of d, rows then columns) + 32) >> 6
//	dst   = Clip1(dst + r)
//
// where qpDiv is qP / 6 and scale is the row of LevelScale8x8 for qP % 6, both
// in raster order (index y*8 + x, as coef). All arithmetic is thirty-two-bit
// two's complement, wrapping as the reference does: a conforming stream keeps
// every intermediate in range, and a hostile one gets the same wrapped answer
// from the portable code and from the vector code. The 8x8 block of dst is the
// bytes dst[off + y*stride + x], 0 <= x, y < 8.
//
// Dequant8x8IDCTAdd panics before touching dst when the block does not lie
// inside dst, when stride < 8 (rows would overlap) or when qpDiv is outside
// 0..31 (qP of at most 51 plus six times the extra bit depth is the standard's
// range; the shift counts below 32 behave identically in every kernel).
// coef and scale are not modified.
func Dequant8x8IDCTAdd(dst []byte, off, stride int, coef, scale *[64]int32, qpDiv int) {
	if qpDiv < 0 || qpDiv > 31 {
		panic(fmt.Sprintf("itx: qpDiv %d outside 0..31", qpDiv))
	}
	if stride < 8 {
		panic(fmt.Sprintf("itx: stride %d is smaller than the block width 8", stride))
	}
	if _, _, ok := bounds.Window(len(dst), off, stride, 0, 7, 0, 7); !ok {
		panic(fmt.Sprintf("itx: 8x8 block does not fit: offset %d stride %d length %d", off, stride, len(dst)))
	}
	dequant8x8IDCTAddDispatch(dst, off, stride, coef, scale, qpDiv)
}

// shifts turns qpDiv into the three constants the kernels take: the scaled
// value is ((c * ls) << sl + rnd) >> sr.
func shifts(qpDiv int) (sl, rnd, sr int) {
	if qpDiv >= 6 {
		return qpDiv - 6, 0, 0
	}
	return 0, 1 << (5 - qpDiv), 6 - qpDiv
}

// idct8Ref is the one-dimensional transform of 8.5.13 on eight values,
// Equations 8-338 to 8-364: a, b then the output butterfly.
func idct8Ref(d [8]int32) [8]int32 {
	e0 := d[0] + d[4]
	e1 := -d[3] + d[5] - d[7] - d[7]>>1
	e2 := d[0] - d[4]
	e3 := d[1] + d[7] - d[3] - d[3]>>1
	e4 := d[2]>>1 - d[6]
	e5 := -d[1] + d[7] + d[5] + d[5]>>1
	e6 := d[2] + d[6]>>1
	e7 := d[3] + d[5] + d[1] + d[1]>>1
	f0 := e0 + e6
	f1 := e1 + e7>>2
	f2 := e2 + e4
	f3 := e3 + e5>>2
	f4 := e2 - e4
	f5 := e3>>2 - e5
	f6 := e0 - e6
	f7 := e7 - e1>>2
	return [8]int32{f0 + f7, f2 + f5, f4 + f3, f6 + f1, f6 - f1, f4 - f3, f2 - f5, f0 - f7}
}

// dequant8x8IDCTAddRef is the portable form, transcribed from the equations.
func dequant8x8IDCTAddRef(dst []byte, off, stride int, coef, scale *[64]int32, qpDiv int) {
	var d [64]int32
	for i, c := range coef {
		if qpDiv >= 6 {
			d[i] = c * scale[i] << uint(qpDiv-6)
		} else {
			d[i] = (c*scale[i] + 1<<uint(5-qpDiv)) >> uint(6-qpDiv)
		}
	}
	d[0] += 32
	var g [64]int32
	for r := 0; r < 8; r++ {
		row := idct8Ref([8]int32(d[r*8 : r*8+8]))
		copy(g[r*8:], row[:])
	}
	for c := 0; c < 8; c++ {
		col := idct8Ref([8]int32{g[c], g[8+c], g[16+c], g[24+c], g[32+c], g[40+c], g[48+c], g[56+c]})
		for r := 0; r < 8; r++ {
			v := int32(dst[off+r*stride+c]) + col[r]>>6
			if v < 0 {
				v = 0
			} else if v > 255 {
				v = 255
			}
			dst[off+r*stride+c] = byte(v)
		}
	}
}
