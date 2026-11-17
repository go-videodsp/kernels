//go:build arm64 && !purego

package hevc

// The dispatch of the byte-source kernels (8-bit pictures): the declarations are generated
// (hpassb_arm64.go, vpassu_arm64.go); the callers have proved the footprints, as for the
// uint16 forms.

func hpass8BDispatch(dst []int16, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	hpass8BNEON[shift](&dst[dstOff], dstStride, &src[srcOff-before], srcStride, w, h, &c[0])
}

func hpass4BDispatch(dst []int16, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	hpass4BNEON[shift](&dst[dstOff], dstStride, &src[srcOff-chromaBefore], srcStride, w, h, &c[0])
}

func vpassU8BDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	vpassU8BNEON[shift](&dst[dstOff], dstStride, &src[srcOff-before*srcStride], srcStride, w, h, &c[0])
}

func vpassU4BDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	vpassU4BNEON[shift](&dst[dstOff], dstStride, &src[srcOff-chromaBefore*srcStride], srcStride, w, h, &c[0])
}

func hpassW8BDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	hpassW8BNEON[shift](&dst[dstOff], dstStride, &src[srcOff-before], srcStride, w, h, &c[0])
}

func hpassW4BDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	hpassW4BNEON[shift](&dst[dstOff], dstStride, &src[srcOff-chromaBefore], srcStride, w, h, &c[0])
}

func widenBDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint) {
	widenBNEON(&dst[dstOff], dstStride, &src[srcOff], srcStride, w, h, int(shift))
}

//go:noescape
func averageBNEON(dst *uint8, dstStride int, a *uint8, aStride int, b *uint8, bStride, w, h int)

//go:noescape
func saoBand8BNEON(dst *uint8, dstStride int, src *uint8, srcStride, w, h int, p *int16)

//go:noescape
func saoEdge8BNEON(dst *uint8, dstStride int, src *uint8, srcStride, w, h, offA, offB int, p *int16)

func averageBDispatch(dst []uint8, dstOff, dstStride int, a []uint8, aOff, aStride int, b []uint8, bOff, bStride, w, h int) {
	averageBNEON(&dst[dstOff], dstStride, &a[aOff], aStride, &b[bOff], bStride, w, h)
}

func weightUniBDispatch(dst []uint8, dstOff, dstStride int, p []int32, pOff, pStride, w, h int, log2Wd uint, q *[4]int32) {
	weightUniBNEON[log2Wd](&dst[dstOff], dstStride, &p[pOff], pStride, w, h, &q[0])
}

func weightBiBDispatch(dst []uint8, dstOff, dstStride int, p0, p1 []int32, pOff, pStride, w, h int, log2Wd uint, q *[4]int32) {
	weightBiBNEON[log2Wd+1](&dst[dstOff], dstStride, &p0[pOff], &p1[pOff], pStride, w, h, &q[0])
}

func saoBandBDispatch(dst []uint8, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, p *[12]int16) {
	saoBand8BNEON(&dst[dstOff], dstStride, &src[srcOff], srcStride, w, h, &p[0])
}

func saoEdgeBDispatch(dst []uint8, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h, offA, offB int, p *[12]int16) {
	saoEdge8BNEON(&dst[dstOff], dstStride, &src[srcOff], srcStride, w, h, offA, offB, &p[0])
}
