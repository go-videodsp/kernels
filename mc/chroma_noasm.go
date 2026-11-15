//go:build (!arm64 && !amd64) || purego

package mc

func putChromaDispatch(dst0, dst1 []byte, dstOff, dstStride int, src0, src1 []byte, srcOff, srcStride, w, h, xFrac, yFrac, planes int) {
	putChromaPairRef(dst0, dst1, dstOff, dstStride, src0, src1, srcOff, srcStride, w, h, xFrac, yFrac, planes)
}
