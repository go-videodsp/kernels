//go:build amd64 && !purego

package mc

import "github.com/go-videodsp/kernels/cpu"

//go:noescape
func chroma8AVX2(dst0, dst1 *byte, dstStride int, src0, src1 *byte, srcStride, h, planes, wx, wy0, wy1 int)

//go:noescape
func chroma4AVX2(dst0, dst1 *byte, dstStride int, src0, src1 *byte, srcStride, h, planes, wx, wy0, wy1 int)

//go:noescape
func chroma2AVX2(dst0, dst1 *byte, dstStride int, src0, src1 *byte, srcStride, h, planes, wx, wy0, wy1 int)

//go:noescape
func chromaAvg8AVX2(dst0, dst1 *byte, dstStride int, src0, src1 *byte, srcStride, h, planes, wx, wy0, wy1 int)

//go:noescape
func chromaAvg4AVX2(dst0, dst1 *byte, dstStride int, src0, src1 *byte, srcStride, h, planes, wx, wy0, wy1 int)

//go:noescape
func chromaAvg2AVX2(dst0, dst1 *byte, dstStride int, src0, src1 *byte, srcStride, h, planes, wx, wy0, wy1 int)

var putChromaImpl = putChromaPairRef

func init() {
	if cpu.HasAVX2 {
		putChromaImpl = putChromaAVX2
	}
}

func putChromaDispatch(dst0, dst1 []byte, dstOff, dstStride int, src0, src1 []byte, srcOff, srcStride, w, h, xFrac, yFrac, planes int, avg bool) {
	putChromaImpl(dst0, dst1, dstOff, dstStride, src0, src1, srcOff, srcStride, w, h, xFrac, yFrac, planes, avg)
}

func putChromaAVX2(dst0, dst1 []byte, dstOff, dstStride int, src0, src1 []byte, srcOff, srcStride, w, h, xFrac, yFrac, planes int, avg bool) {
	wx := xFrac<<8 | (8 - xFrac)
	f := chroma2AVX2
	switch {
	case w == 8 && avg:
		f = chromaAvg8AVX2
	case w == 8:
		f = chroma8AVX2
	case w == 4 && avg:
		f = chromaAvg4AVX2
	case w == 4:
		f = chroma4AVX2
	case avg:
		f = chromaAvg2AVX2
	}
	f(&dst0[dstOff], &dst1[dstOff], dstStride, &src0[srcOff], &src1[srcOff], srcStride, h, planes, wx, 8-yFrac, yFrac)
}
