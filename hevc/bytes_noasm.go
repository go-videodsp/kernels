//go:build (!arm64 && !amd64) || purego

package hevc

// The byte-source kernels have no assembly here: the portable reference is the path.

func hpass8BDispatch(dst []int16, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	hpassRef(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
}

func hpass4BDispatch(dst []int16, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	hpass4Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
}

func vpassU8BDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	vpassU8Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
}

func vpassU4BDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	vpassU4Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
}

func hpassW8BDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	hpassW8Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
}

func hpassW4BDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	hpassW4Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
}

func widenBDispatch(dst []int32, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, shift uint) {
	widenRef(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift)
}

func averageBDispatch(dst []uint8, dstOff, dstStride int, a []uint8, aOff, aStride int, b []uint8, bOff, bStride, w, h int) {
	averageRef(dst, dstOff, dstStride, a, aOff, aStride, b, bOff, bStride, w, h)
}

func weightUniBDispatch(dst []uint8, dstOff, dstStride int, p []int32, pOff, pStride, w, h int, log2Wd uint, q *[4]int32) {
	weightUniRef(dst, dstOff, dstStride, p, pOff, pStride, w, h, log2Wd, q)
}

func weightBiBDispatch(dst []uint8, dstOff, dstStride int, p0, p1 []int32, pOff, pStride, w, h int, log2Wd uint, q *[4]int32) {
	weightBiRef(dst, dstOff, dstStride, p0, p1, pOff, pStride, w, h, log2Wd, q)
}

func saoBandBDispatch(dst []uint8, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h int, p *[12]int16) {
	var off [4]int16
	copy(off[:], p[:4])
	saoBandRef(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, 8, int(p[4]), &off)
}

func saoEdgeBDispatch(dst []uint8, dstOff, dstStride int, src []uint8, srcOff, srcStride, w, h, offA, offB int, p *[12]int16) {
	var off [4]int16
	copy(off[:], p[:4])
	saoEdgeRef(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, 8, offA, offB, &off)
}
