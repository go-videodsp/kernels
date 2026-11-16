package hevc

// The four-tap chroma filter of ITU-T H.265 (V11) (01/2026), clause 8.5.3.3.3.3
// (Equations 8-241 to 8-261): the same two passes as the luma filter, with four taps at
// offsets -1 to 2, on blocks whose width is any even number from 2 to 64.

// HPass4 writes the w x h block of the horizontal four-tap filter of the samples
// src[srcOff + y*srcStride + x] to dst[dstOff + y*dstStride + x]:
//
//	dst = int16((c[0]*s[x-1] + c[1]*s[x] + c[2]*s[x+1] + c[3]*s[x+2]) >> (bitDepth - 8))
//
// The contract is HPass8's, with a width that is a multiple of 2 and a footprint of
// 1 column before and 2 after. HPass4 panics where TryHPass4 returns an error.
func HPass4(dst []int16, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h, bitDepth int, c *[4]int16) {
	if err := TryHPass4(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, bitDepth, c); err != nil {
		panic(err)
	}
}

// TryHPass4 is HPass4 returning the refusal as an error, with nothing written.
func TryHPass4(dst []int16, dstOff, dstStride int, src []uint16, srcOff, srcStride, w, h, bitDepth int, c *[4]int16) error {
	if err := checkH(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, bitDepth, 2, chromaBefore, chromaAfter, c != nil); err != nil {
		return err
	}
	hpass4Dispatch(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, uint(bitDepth-8), c)
	return nil
}

// VPass4 is the vertical four-tap filter of the int16 intermediates, shift 6, into int32
// (the second stage of the two-dimensional chroma filter); the footprint is 1 row before and 2
// after. Its preconditions are HPass4's.
func VPass4(dst []int32, dstOff, dstStride int, src []int16, srcOff, srcStride, w, h int, c *[4]int16) {
	if err := TryVPass4(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, c); err != nil {
		panic(err)
	}
}

// TryVPass4 is VPass4 returning the refusal as an error, with nothing written.
func TryVPass4(dst []int32, dstOff, dstStride int, src []int16, srcOff, srcStride, w, h int, c *[4]int16) error {
	if err := checkV(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, 2, chromaBefore, chromaAfter, c != nil); err != nil {
		return err
	}
	vpass4Dispatch(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, c)
	return nil
}
