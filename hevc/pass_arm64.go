//go:build arm64 && !purego

package hevc

//go:noescape
func hpass8NEON0(dst *int16, dstStride int, src *uint16, srcStride, w, h int, c *int16)

//go:noescape
func hpass8NEON1(dst *int16, dstStride int, src *uint16, srcStride, w, h int, c *int16)

//go:noescape
func hpass8NEON2(dst *int16, dstStride int, src *uint16, srcStride, w, h int, c *int16)

//go:noescape
func hpass8NEON3(dst *int16, dstStride int, src *uint16, srcStride, w, h int, c *int16)

//go:noescape
func hpass8NEON4(dst *int16, dstStride int, src *uint16, srcStride, w, h int, c *int16)

//go:noescape
func vpass8NEON(dst *int32, dstStride int, src *int16, srcStride, w, h int, c *int16)

var hpassNEON = [5]func(dst *int16, dstStride int, src *uint16, srcStride, w, h int, c *int16){
	hpass8NEON0, hpass8NEON1, hpass8NEON2, hpass8NEON3, hpass8NEON4,
}

// The callers have proved the footprints (checkH, checkV), so the pointers and the
// loops inside the assembly stay within dst and src: the kernels read and write
// exactly the samples of the footprint and of the block.

func hpassDispatch(dst []int16, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	hpassNEON[shift](&dst[dstOff], dstStride, &src[srcOff-before], srcStride, w, h, &c[0])
}

func vpassDispatch(dst []int32, dstOff, dstStride int, src []int16, srcOff, srcStride, w, h int, c *[8]int16) {
	vpass8NEON(&dst[dstOff], dstStride, &src[srcOff-before*srcStride], srcStride, w, h, &c[0])
}

//go:noescape
func hpass4NEON0(dst *int16, dstStride int, src *uint16, srcStride, w, h int, c *int16)

//go:noescape
func hpass4NEON1(dst *int16, dstStride int, src *uint16, srcStride, w, h int, c *int16)

//go:noescape
func hpass4NEON2(dst *int16, dstStride int, src *uint16, srcStride, w, h int, c *int16)

//go:noescape
func hpass4NEON3(dst *int16, dstStride int, src *uint16, srcStride, w, h int, c *int16)

//go:noescape
func hpass4NEON4(dst *int16, dstStride int, src *uint16, srcStride, w, h int, c *int16)

//go:noescape
func vpass4NEON(dst *int32, dstStride int, src *int16, srcStride, w, h int, c *int16)

var hpass4NEON = [5]func(dst *int16, dstStride int, src *uint16, srcStride, w, h int, c *int16){
	hpass4NEON0, hpass4NEON1, hpass4NEON2, hpass4NEON3, hpass4NEON4,
}

func hpass4Dispatch(dst []int16, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	hpass4NEON[shift](&dst[dstOff], dstStride, &src[srcOff-chromaBefore], srcStride, w, h, &c[0])
}

func vpass4Dispatch(dst []int32, dstOff, dstStride int, src []int16, srcOff, srcStride, w, h int, c *[4]int16) {
	vpass4NEON(&dst[dstOff], dstStride, &src[srcOff-chromaBefore*srcStride], srcStride, w, h, &c[0])
}

//go:noescape
func saoBand8NEON3(dst *uint16, dstStride int, src *uint16, srcStride, w, h int, p *int16)

//go:noescape
func saoBand8NEON4(dst *uint16, dstStride int, src *uint16, srcStride, w, h int, p *int16)

//go:noescape
func saoBand8NEON5(dst *uint16, dstStride int, src *uint16, srcStride, w, h int, p *int16)

//go:noescape
func saoBand8NEON6(dst *uint16, dstStride int, src *uint16, srcStride, w, h int, p *int16)

//go:noescape
func saoBand8NEON7(dst *uint16, dstStride int, src *uint16, srcStride, w, h int, p *int16)

//go:noescape
func saoEdge8NEON(dst *uint16, dstStride int, src *uint16, srcStride, w, h, offA, offB int, p *int16)

var saoBandNEON = [8]func(dst *uint16, dstStride int, src *uint16, srcStride, w, h int, p *int16){
	3: saoBand8NEON3, 4: saoBand8NEON4, 5: saoBand8NEON5, 6: saoBand8NEON6, 7: saoBand8NEON7,
}

func saoBandDispatch(dst []uint16, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h int, shift uint, p *[12]int16) {
	saoBandNEON[shift](&dst[dstOff], dstStride, &src[srcOff], srcStride, w, h, &p[0])
}

func saoEdgeDispatch(dst []uint16, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h, offA, offB int, p *[12]int16) {
	saoEdge8NEON(&dst[dstOff], dstStride, &src[srcOff], srcStride, w, h, offA, offB, &p[0])
}

func vpassU8Dispatch(dst []int32, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	vpassU8NEON[shift](&dst[dstOff], dstStride, &src[srcOff-before*srcStride], srcStride, w, h, &c[0])
}

func vpassU4Dispatch(dst []int32, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	vpassU4NEON[shift](&dst[dstOff], dstStride, &src[srcOff-chromaBefore*srcStride], srcStride, w, h, &c[0])
}

func hpassW8Dispatch(dst []int32, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	hpassW8NEON[shift](&dst[dstOff], dstStride, &src[srcOff-before], srcStride, w, h, &c[0])
}

func hpassW4Dispatch(dst []int32, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	hpassW4NEON[shift](&dst[dstOff], dstStride, &src[srcOff-chromaBefore], srcStride, w, h, &c[0])
}

func widenDispatch(dst []int32, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h int, shift uint) {
	widenNEON(&dst[dstOff], dstStride, &src[srcOff], srcStride, w, h, int(shift))
}

//go:noescape
func idctNEON(dst, d, gb, mat *int32, matStride, n, maxX, maxY, shift int)

func idctDispatch(dst, d, g, mat []int32, matStride, n, maxX, maxY int, shift uint) {
	idctNEON(&dst[0], &d[0], &g[0], &mat[0], matStride, n, maxX, maxY, int(shift))
}

//go:noescape
func averageNEON(dst *uint16, dstStride int, a *uint16, aStride int, b *uint16, bStride, w, h int)

func averageDispatch(dst []uint16, dstOff, dstStride int, a []uint16, aOff, aStride int, b []uint16, bOff, bStride, w, h int) {
	averageNEON(&dst[dstOff], dstStride, &a[aOff], aStride, &b[bOff], bStride, w, h)
}

func weightUniDispatch(dst []uint16, dstOff, dstStride int, p []int32, pOff, pStride, w, h int, log2Wd uint, q *[4]int32) {
	weightUniNEON[log2Wd](&dst[dstOff], dstStride, &p[pOff], pStride, w, h, &q[0])
}

func weightBiDispatch(dst []uint16, dstOff, dstStride int, p0, p1 []int32, pOff, pStride, w, h int, log2Wd uint, q *[4]int32) {
	weightBiNEON[log2Wd+1](&dst[dstOff], dstStride, &p0[pOff], &p1[pOff], pStride, w, h, &q[0])
}
