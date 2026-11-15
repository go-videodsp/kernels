//go:build arm64 && !purego

package mc

//go:noescape
func chroma8NEON(dst0, dst1 *byte, dstStride int, src0, src1 *byte, srcStride, h, planes, wx, wy0, wy1 int)

//go:noescape
func chroma4NEON(dst0, dst1 *byte, dstStride int, src0, src1 *byte, srcStride, h, planes, wx, wy0, wy1 int)

//go:noescape
func chroma2NEON(dst0, dst1 *byte, dstStride int, src0, src1 *byte, srcStride, h, planes, wx, wy0, wy1 int)

func putChromaDispatch(dst0, dst1 []byte, dstOff, dstStride int, src0, src1 []byte, srcOff, srcStride, w, h, xFrac, yFrac, planes int) {
	wx := xFrac<<8 | (8 - xFrac)
	f := chroma2NEON
	switch w {
	case 8:
		f = chroma8NEON
	case 4:
		f = chroma4NEON
	}
	f(&dst0[dstOff], &dst1[dstOff], dstStride, &src0[srcOff], &src1[srcOff], srcStride, h, planes, wx, 8-yFrac, yFrac)
}
