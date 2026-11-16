//go:build amd64 && !purego

package hevc

import "github.com/go-videodsp/kernels/cpu"

//go:noescape
func hpass8AVX2(dst *int16, dstStride int, src *uint16, srcStride, w, h int, c *int16, shift int)

//go:noescape
func vpass8AVX2(dst *int32, dstStride int, src *int16, srcStride, w, h int, c *int16)

// useAVX2 is the AVX2 path when the processor and the operating system have AVX2,
// the portable reference otherwise.
var useAVX2 = cpu.HasAVX2

// The callers have proved the footprints (checkH, checkV): the kernels read and
// write exactly the samples of the footprint and of the block.

func hpassDispatch(dst []int16, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	if !useAVX2 {
		hpassRef(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
		return
	}
	hpass8AVX2(&dst[dstOff], dstStride, &src[srcOff-before], srcStride, w, h, &c[0], int(shift))
}

func vpassDispatch(dst []int32, dstOff, dstStride int, src []int16, srcOff, srcStride, w, h int, c *[8]int16) {
	if !useAVX2 {
		vpassRef(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, c)
		return
	}
	vpass8AVX2(&dst[dstOff], dstStride, &src[srcOff-before*srcStride], srcStride, w, h, &c[0])
}

//go:noescape
func hpass4AVX2(dst *int16, dstStride int, src *uint16, srcStride, w, h int, c *int16, shift int)

//go:noescape
func vpass4AVX2(dst *int32, dstStride int, src *int16, srcStride, w, h int, c *int16)

func hpass4Dispatch(dst []int16, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	if !useAVX2 {
		hpass4Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
		return
	}
	hpass4AVX2(&dst[dstOff], dstStride, &src[srcOff-chromaBefore], srcStride, w, h, &c[0], int(shift))
}

func vpass4Dispatch(dst []int32, dstOff, dstStride int, src []int16, srcOff, srcStride, w, h int, c *[4]int16) {
	if !useAVX2 {
		vpass4Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, c)
		return
	}
	vpass4AVX2(&dst[dstOff], dstStride, &src[srcOff-chromaBefore*srcStride], srcStride, w, h, &c[0])
}

//go:noescape
func saoBandAVX2(dst *uint16, dstStride int, src *uint16, srcStride, w, h int, p *int16, shift int)

//go:noescape
func saoEdgeAVX2(dst *uint16, dstStride int, src *uint16, srcStride, w, h, offA, offB int, p *int16)

func saoBandDispatch(dst []uint16, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h int, shift uint, p *[12]int16) {
	if !useAVX2 {
		var off [4]int16
		copy(off[:], p[:4])
		saoBandRef(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, int(shift)+5, int(p[4]), &off)
		return
	}
	saoBandAVX2(&dst[dstOff], dstStride, &src[srcOff], srcStride, w, h, &p[0], int(shift))
}

func saoEdgeDispatch(dst []uint16, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h, offA, offB int, p *[12]int16) {
	if !useAVX2 {
		var off [4]int16
		copy(off[:], p[:4])
		bd := 8
		for 1<<uint(bd)-1 < int(p[8]) {
			bd++
		}
		saoEdgeRef(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, bd, offA, offB, &off)
		return
	}
	saoEdgeAVX2(&dst[dstOff], dstStride, &src[srcOff], srcStride, w, h, offA, offB, &p[0])
}

//go:noescape
func vpassU8AVX2(dst *int32, dstStride int, src *uint16, srcStride, w, h int, c *int16, shift int)

//go:noescape
func vpassU4AVX2(dst *int32, dstStride int, src *uint16, srcStride, w, h int, c *int16, shift int)

func vpassU8Dispatch(dst []int32, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	if !useAVX2 {
		vpassU8Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
		return
	}
	vpassU8AVX2(&dst[dstOff], dstStride, &src[srcOff-before*srcStride], srcStride, w, h, &c[0], int(shift))
}

func vpassU4Dispatch(dst []int32, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	if !useAVX2 {
		vpassU4Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
		return
	}
	vpassU4AVX2(&dst[dstOff], dstStride, &src[srcOff-chromaBefore*srcStride], srcStride, w, h, &c[0], int(shift))
}

//go:noescape
func idctAVX2x8(dst, d, gb, mat *int32, matStride, n, maxX, maxY, shift int)

//go:noescape
func idctAVX2x4(dst, d, gb, mat *int32, matStride, n, maxX, maxY, shift int)

func idctDispatch(dst, d, g, mat []int32, matStride, n, maxX, maxY int, shift uint) {
	switch {
	case !useAVX2:
		idctRef(dst, d, g, mat, matStride, n, maxX, maxY, shift)
	case n == 4:
		idctAVX2x4(&dst[0], &d[0], &g[0], &mat[0], matStride, n, maxX, maxY, int(shift))
	default:
		idctAVX2x8(&dst[0], &d[0], &g[0], &mat[0], matStride, n, maxX, maxY, int(shift))
	}
}

//go:noescape
func averageAVX2(dst *uint16, dstStride int, a *uint16, aStride int, b *uint16, bStride, w, h int)

func averageDispatch(dst []uint16, dstOff, dstStride int, a []uint16, aOff, aStride int, b []uint16, bOff, bStride, w, h int) {
	if !useAVX2 {
		averageRef(dst, dstOff, dstStride, a, aOff, aStride, b, bOff, bStride, w, h)
		return
	}
	averageAVX2(&dst[dstOff], dstStride, &a[aOff], aStride, &b[bOff], bStride, w, h)
}

//go:noescape
func weightUniAVX2(dst *uint16, dstStride int, p *int32, pStride, w, h int, q *int32, shift int)

//go:noescape
func weightBiAVX2(dst *uint16, dstStride int, p *int32, p1 *int32, pStride, w, h int, q *int32, shift int)

func weightUniDispatch(dst []uint16, dstOff, dstStride int, p []int32, pOff, pStride, w, h int, log2Wd uint, q *[4]int32) {
	if !useAVX2 {
		weightUniRef(dst, dstOff, dstStride, p, pOff, pStride, w, h, log2Wd, q)
		return
	}
	weightUniAVX2(&dst[dstOff], dstStride, &p[pOff], pStride, w, h, &q[0], int(log2Wd))
}

func weightBiDispatch(dst []uint16, dstOff, dstStride int, p0, p1 []int32, pOff, pStride, w, h int, log2Wd uint, q *[4]int32) {
	if !useAVX2 {
		weightBiRef(dst, dstOff, dstStride, p0, p1, pOff, pStride, w, h, log2Wd, q)
		return
	}
	weightBiAVX2(&dst[dstOff], dstStride, &p0[pOff], &p1[pOff], pStride, w, h, &q[0], int(log2Wd)+1)
}
