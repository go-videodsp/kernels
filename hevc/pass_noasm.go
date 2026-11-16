//go:build (!arm64 && !amd64) || purego

package hevc

// The portable path: every target without a kernel, and any build with the purego tag.

func hpassDispatch(dst []int16, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	hpassRef(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
}

func vpassDispatch(dst []int32, dstOff, dstStride int, src []int16, srcOff, srcStride, w, h int, c *[8]int16) {
	vpassRef(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, c)
}

func hpass4Dispatch(dst []int16, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	hpass4Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
}

func vpass4Dispatch(dst []int32, dstOff, dstStride int, src []int16, srcOff, srcStride, w, h int, c *[4]int16) {
	vpass4Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, c)
}

func saoBandDispatch(dst []uint16, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h int, shift uint, p *[12]int16) {
	var off [4]int16
	copy(off[:], p[:4])
	saoBandRef(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, int(shift)+5, int(p[4]), &off)
}

func saoEdgeDispatch(dst []uint16, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h, offA, offB int, p *[12]int16) {
	var off [4]int16
	copy(off[:], p[:4])
	hi := int(p[8])
	bd := 8
	for 1<<uint(bd)-1 < hi {
		bd++
	}
	saoEdgeRef(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, bd, offA, offB, &off)
}

func vpassU8Dispatch(dst []int32, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	vpassU8Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
}

func vpassU4Dispatch(dst []int32, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	vpassU4Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
}

func hpassW8Dispatch(dst []int32, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h int, shift uint, c *[8]int16) {
	hpassW8Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
}

func hpassW4Dispatch(dst []int32, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h int, shift uint, c *[4]int16) {
	hpassW4Ref(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift, c)
}

func widenDispatch(dst []int32, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h int, shift uint) {
	widenRef(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, shift)
}

func idctDispatch(dst, d, g, mat []int32, matStride, n, maxX, maxY int, shift uint) {
	idctRef(dst, d, g, mat, matStride, n, maxX, maxY, shift)
}

func averageDispatch(dst []uint16, dstOff, dstStride int, a []uint16, aOff, aStride int, b []uint16, bOff, bStride, w, h int) {
	averageRef(dst, dstOff, dstStride, a, aOff, aStride, b, bOff, bStride, w, h)
}

func weightUniDispatch(dst []uint16, dstOff, dstStride int, p []int32, pOff, pStride, w, h int, log2Wd uint, q *[4]int32) {
	weightUniRef(dst, dstOff, dstStride, p, pOff, pStride, w, h, log2Wd, q)
}

func weightBiDispatch(dst []uint16, dstOff, dstStride int, p0, p1 []int32, pOff, pStride, w, h int, log2Wd uint, q *[4]int32) {
	weightBiRef(dst, dstOff, dstStride, p0, p1, pOff, pStride, w, h, log2Wd, q)
}
