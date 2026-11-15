//go:build !amd64 || purego

package mc

// weightUniParts and weightBiParts run the per-block dispatch on each partition
// of a proved macroblock (arm64 NEON, or the portable code).

func weightUniParts(dst []byte, off, stride int, parts []UniPart, logWD int) {
	weightUniPartsDispatch(dst, off, stride, parts, logWD)
}

func weightBiParts(dst []byte, dstOff, dstStride int, src []byte, srcOff, srcStride int, parts []BiPart, logWD int) {
	weightBiPartsDispatch(dst, dstOff, dstStride, src, srcOff, srcStride, parts, logWD)
}
