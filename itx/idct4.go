package itx

import (
	"fmt"

	"github.com/go-videodsp/kernels/internal/bounds"
)

//go:generate go run ../internal/genitx ..

// blockX and blockY place block b of the standard's z-scan order: blocks 4q to
// 4q+3 make the 8x8 quadrant q, and within a quadrant the order is (0,0),
// (4,0), (0,4), (4,4) (the order of the four chroma blocks of a macroblock).
var blockX = [16]int{0, 4, 0, 4, 8, 12, 8, 12, 0, 4, 0, 4, 8, 12, 8, 12}
var blockY = [16]int{0, 0, 4, 4, 0, 0, 4, 4, 8, 8, 12, 12, 8, 8, 12, 12}

// Dequant4x4IDCTAddBlocks scales, transforms and adds the 4x4 blocks selected
// by mask to the prediction in dst, in place (ITU-T H.264 (V16) (06/2026),
// clauses 8.5.9, 8.5.12 and 8.5.14, for 8-bit samples). n is 16 (the blocks of
// a 16x16 region) or 4 (the blocks of an 8x8 region); bit b of mask selects
// block b of the z-scan order, and bits at or above n are ignored. For each
// selected block, with c = coef[b] and ls = scale in raster order (index
// y*4 + x):
//
//	d[i] = c[i] * ls[i] << (qpDiv - 4)                    qpDiv >= 4
//	d[i] = (c[i] * ls[i] + 2^(3 - qpDiv)) >> (4 - qpDiv)  qpDiv < 4
//	r    = (inverse transform of d, rows then columns + 32) >> 6
//	dst  = Clip1(dst + r)
//
// except that with skipDC the first coefficient is not scaled (the DC of an
// Intra16x16 or chroma block arrives already scaled by its own transform).
// The 4x4 block of coefficients b is placed at dst[off + blockY*stride +
// blockX]. Arithmetic is thirty-two-bit two's complement, wrapping as the
// portable reference does. Blocks not selected are not read or written, and
// coef and scale are not modified.
//
// Dequant4x4IDCTAddBlocks panics before touching dst when the region does not
// lie inside dst (16x16 for n = 16, 8x8 for n = 4), when stride < 16 (n = 16)
// or 8 (n = 4), when n is neither, or when qpDiv is outside 0..31.
func Dequant4x4IDCTAddBlocks(dst []byte, off, stride, n int, coef *[16][16]int32, mask uint16, scale *[16]int32, qpDiv int, skipDC bool) {
	if n != 4 && n != 16 {
		panic(fmt.Sprintf("itx: %d blocks is not 4 or 16", n))
	}
	if qpDiv < 0 || qpDiv > 31 {
		panic(fmt.Sprintf("itx: qpDiv %d outside 0..31", qpDiv))
	}
	size := 16
	if n == 4 {
		size = 8
	}
	if stride < size {
		panic(fmt.Sprintf("itx: stride %d is smaller than the region width %d", stride, size))
	}
	if _, _, ok := bounds.Window(len(dst), off, stride, 0, size-1, 0, size-1); !ok {
		panic(fmt.Sprintf("itx: %dx%d region does not fit: offset %d stride %d length %d", size, size, off, stride, len(dst)))
	}
	mask &= 1<<uint(n) - 1
	if mask == 0 {
		return
	}
	dequant4x4IDCTAddBlocksDispatch(dst, off, stride, coef, mask, scale, qpDiv, skipDC)
}

// LumaDCDequant is the transform and scaling of the sixteen luma DC
// coefficients of an Intra16x16 macroblock, in place (clause 8.5.10): the
// 4x4 Hadamard transform f = A c A and, with ls0 the first entry of the
// scale for qP % 6,
//
//	dc[i] = f[i] * ls0 << (qpDiv - 6)                    qpDiv >= 6
//	dc[i] = (f[i] * ls0 + 2^(5 - qpDiv)) >> (6 - qpDiv)  qpDiv < 6
//
// The matrix is in raster order. Arithmetic wraps as the reference does.
// It panics when qpDiv is outside 0..31.
func LumaDCDequant(dc *[16]int32, ls0 int32, qpDiv int) {
	if qpDiv < 0 || qpDiv > 31 {
		panic(fmt.Sprintf("itx: qpDiv %d outside 0..31", qpDiv))
	}
	lumaDCDequantDispatch(dc, ls0, qpDiv)
}

// shifts4 is shifts for the 4x4 scaling: ((c * ls) << sl + rnd) >> sr.
func shifts4(qpDiv int) (sl, rnd, sr int) {
	if qpDiv >= 4 {
		return qpDiv - 4, 0, 0
	}
	return 0, 1 << (3 - qpDiv), 4 - qpDiv
}

// idct4Ref is the one-dimensional transform of Equations 8-338 to 8-345.
func idct4Ref(d [4]int32) [4]int32 {
	e0 := d[0] + d[2]
	e1 := d[0] - d[2]
	e2 := d[1]>>1 - d[3]
	e3 := d[1] + d[3]>>1
	return [4]int32{e0 + e3, e1 + e2, e1 - e2, e0 - e3}
}

// dequant4x4IDCTAddBlocksRef is the portable form, transcribed from the
// equations.
func dequant4x4IDCTAddBlocksRef(dst []byte, off, stride int, coef *[16][16]int32, mask uint16, scale *[16]int32, qpDiv int, skipDC bool) {
	for b := 0; b < 16; b++ {
		if mask>>uint(b)&1 == 0 {
			continue
		}
		var d [16]int32
		for i, c := range coef[b] {
			switch {
			case i == 0 && skipDC:
				d[i] = c
			case qpDiv >= 4:
				d[i] = c * scale[i] << uint(qpDiv-4)
			default:
				d[i] = (c*scale[i] + 1<<uint(3-qpDiv)) >> uint(4-qpDiv)
			}
		}
		var g [16]int32
		for r := 0; r < 4; r++ {
			row := idct4Ref([4]int32(d[r*4 : r*4+4]))
			copy(g[r*4:], row[:])
		}
		for c := 0; c < 4; c++ {
			col := idct4Ref([4]int32{g[c], g[4+c], g[8+c], g[12+c]})
			for r := 0; r < 4; r++ {
				p := off + (blockY[b]+r)*stride + blockX[b] + c
				v := int32(dst[p]) + (col[r]+32)>>6
				if v < 0 {
					v = 0
				} else if v > 255 {
					v = 255
				}
				dst[p] = byte(v)
			}
		}
	}
}

// lumaDCDequantRef is the portable form: Equations 8-324 to 8-327 as written,
// the two Hadamard passes and the scaling.
func lumaDCDequantRef(dc *[16]int32, ls0 int32, qpDiv int) {
	hadamard := func(a, b, c, d int32) (int32, int32, int32, int32) {
		t0, t1, t2, t3 := a+d, b+c, b-c, a-d
		return t0 + t1, t3 + t2, t0 - t1, t3 - t2
	}
	for i := 0; i < 16; i += 4 {
		dc[i], dc[i+1], dc[i+2], dc[i+3] = hadamard(dc[i], dc[i+1], dc[i+2], dc[i+3])
	}
	for i := 0; i < 4; i++ {
		dc[i], dc[i+4], dc[i+8], dc[i+12] = hadamard(dc[i], dc[i+4], dc[i+8], dc[i+12])
	}
	for i := range dc {
		if qpDiv >= 6 {
			dc[i] = dc[i] * ls0 << uint(qpDiv-6)
		} else {
			dc[i] = (dc[i]*ls0 + 1<<uint(5-qpDiv)) >> uint(6-qpDiv)
		}
	}
}
