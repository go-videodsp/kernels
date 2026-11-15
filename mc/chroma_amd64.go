//go:build amd64 && !purego

package mc

import "github.com/go-videodsp/kernels/cpu"

//go:noescape
func chroma8AVX2(dst0, dst1 *byte, dstStride int, src0, src1 *byte, srcStride, h, planes, wx, wy0, wy1 int)

//go:noescape
func chroma4AVX2(dst0, dst1 *byte, dstStride int, src0, src1 *byte, srcStride, h, planes, wx, wy0, wy1 int)

//go:noescape
func chroma2AVX2(dst0, dst1 *byte, dstStride int, src0, src1 *byte, srcStride, h, planes, wx, wy0, wy1 int)

var putChromaImpl = putChromaPairRef

func init() {
	if cpu.HasAVX2 {
		putChromaImpl = putChromaAVX2
	}
}

func putChromaDispatch(dst0, dst1 []byte, dstOff, dstStride int, src0, src1 []byte, srcOff, srcStride, w, h, xFrac, yFrac, planes int) {
	putChromaImpl(dst0, dst1, dstOff, dstStride, src0, src1, srcOff, srcStride, w, h, xFrac, yFrac, planes)
}

func putChromaAVX2(dst0, dst1 []byte, dstOff, dstStride int, src0, src1 []byte, srcOff, srcStride, w, h, xFrac, yFrac, planes int) {
	wx := xFrac<<8 | (8 - xFrac)
	f := chroma2AVX2
	switch w {
	case 8:
		f = chroma8AVX2
	case 4:
		f = chroma4AVX2
	}
	f(&dst0[dstOff], &dst1[dstOff], dstStride, &src0[srcOff], &src1[srcOff], srcStride, h, planes, wx, 8-yFrac, yFrac)
}
