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
