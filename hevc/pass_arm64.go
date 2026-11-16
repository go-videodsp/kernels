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
