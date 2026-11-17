//go:build amd64 && !purego

package hevc

// The dispatch of the byte-source kernels (8-bit pictures), as pass_amd64.go's for uint16:
// the portable reference where AVX2 is missing.

//go:noescape
func hpass8BAVX2(dst *int16, dstStride int, src *uint8, srcStride, w, h int, c *int16, shift int)

//go:noescape
func hpass4BAVX2(dst *int16, dstStride int, src *uint8, srcStride, w, h int, c *int16, shift int)

//go:noescape
func vpassU8BAVX2(dst *int32, dstStride int, src *uint8, srcStride, w, h int, c *int16, shift int)

//go:noescape
func vpassU4BAVX2(dst *int32, dstStride int, src *uint8, srcStride, w, h int, c *int16, shift int)

//go:noescape
func hpassW8BAVX2(dst *int32, dstStride int, src *uint8, srcStride, w, h int, c *int16, shift int)

//go:noescape
func hpassW4BAVX2(dst *int32, dstStride int, src *uint8, srcStride, w, h int, c *int16, shift int)

//go:noescape
func widenBAVX2(dst *int32, dstStride int, src *uint8, srcStride, w, h, shift int)

func hpass8BDispatch(dst []int16, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	if !useAVX2 {
		hpassRef(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
		return
	}
	hpass8BAVX2(&dst[dstOff], dstStride, &src[srcOff-before], srcStride, w, h, &c[0], int(shift))
}

func hpass4BDispatch(dst []int16, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	if !useAVX2 {
		hpass4Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
		return
	}
	hpass4BAVX2(&dst[dstOff], dstStride, &src[srcOff-chromaBefore], srcStride, w, h, &c[0], int(shift))
}

func vpassU8BDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	if !useAVX2 {
		vpassU8Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
		return
	}
	vpassU8BAVX2(&dst[dstOff], dstStride, &src[srcOff-before*srcStride], srcStride, w, h, &c[0], int(shift))
}

func vpassU4BDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	if !useAVX2 {
		vpassU4Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
		return
	}
	vpassU4BAVX2(&dst[dstOff], dstStride, &src[srcOff-chromaBefore*srcStride], srcStride, w, h, &c[0], int(shift))
}

func hpassW8BDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	if !useAVX2 {
		hpassW8Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
		return
	}
	hpassW8BAVX2(&dst[dstOff], dstStride, &src[srcOff-before], srcStride, w, h, &c[0], int(shift))
}

func hpassW4BDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	if !useAVX2 {
		hpassW4Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
		return
	}
	hpassW4BAVX2(&dst[dstOff], dstStride, &src[srcOff-chromaBefore], srcStride, w, h, &c[0], int(shift))
}

func widenBDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint) {
	if !useAVX2 {
		widenRef(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift)
		return
	}
	widenBAVX2(&dst[dstOff], dstStride, &src[srcOff], srcStride, w, h, int(shift))
}

//go:noescape
func averageBAVX2(dst *uint8, dstStride int, a *uint8, aStride int, b *uint8, bStride, w, h int)

//go:noescape
func weightUniBAVX2(dst *uint8, dstStride int, p *int32, pStride, w, h int, q *int32, shift int)

//go:noescape
func weightBiBAVX2(dst *uint8, dstStride int, p *int32, p1 *int32, pStride, w, h int, q *int32, shift int)

//go:noescape
func saoBandBAVX2(dst *uint8, dstStride int, src *uint8, srcStride, w, h int, p *int16)

//go:noescape
func saoEdgeBAVX2(dst *uint8, dstStride int, src *uint8, srcStride, w, h, offA, offB int, p *int16)

func averageBDispatch(dst []uint8, dstOff, dstStride int, a []uint8, aOff, aStride int, b []uint8, bOff, bStride, w, h int) {
	if !useAVX2 {
		averageRef(dst, dstOff, dstStride, a, aOff, aStride, b, bOff, bStride, w, h)
		return
	}
	averageBAVX2(&dst[dstOff], dstStride, &a[aOff], aStride, &b[bOff], bStride, w, h)
}

func weightUniBDispatch(dst []uint8, dstOff, dstStride int, p []int32, pOff, pStride, w, h int, log2Wd uint, q *[4]int32) {
	if !useAVX2 {
		weightUniRef(dst, dstOff, dstStride, p, pOff, pStride, w, h, log2Wd, q)
		return
	}
	weightUniBAVX2(&dst[dstOff], dstStride, &p[pOff], pStride, w, h, &q[0], int(log2Wd))
}

func weightBiBDispatch(dst []uint8, dstOff, dstStride int, p0, p1 []int32, pOff, pStride, w, h int, log2Wd uint, q *[4]int32) {
	if !useAVX2 {
		weightBiRef(dst, dstOff, dstStride, p0, p1, pOff, pStride, w, h, log2Wd, q)
		return
	}
	weightBiBAVX2(&dst[dstOff], dstStride, &p0[pOff], &p1[pOff], pStride, w, h, &q[0], int(log2Wd+1))
}

func saoBandBDispatch(dst []uint8, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, p *[12]int16) {
	if !useAVX2 {
		var off [4]int16
		copy(off[:], p[:4])
		saoBandRef(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, 8, int(p[4]), &off)
		return
	}
	saoBandBAVX2(&dst[dstOff], dstStride, &src[srcOff], srcStride, w, h, &p[0])
}

func saoEdgeBDispatch(dst []uint8, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h, offA, offB int, p *[12]int16) {
	if !useAVX2 {
		var off [4]int16
		copy(off[:], p[:4])
		saoEdgeRef(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, 8, offA, offB, &off)
		return
	}
	saoEdgeBAVX2(&dst[dstOff], dstStride, &src[srcOff], srcStride, w, h, offA, offB, &p[0])
}
