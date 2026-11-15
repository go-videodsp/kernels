//go:build amd64 && !purego

package mc

import "github.com/go-videodsp/kernels/cpu"

// lumaImpl is the AVX2 path when the processor and the operating system have
// AVX2, the portable reference otherwise.
var lumaImpl = lumaRef

func init() {
	if cpu.HasAVX2 {
		lumaImpl = lumaAVX2
	}
}

func lumaDispatch(dst []byte, dstOff, dstStride int, src []byte, srcOff, srcStride, w, h, xFrac, yFrac int, avg bool) {
	lumaImpl(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, xFrac, yFrac, avg)
}

// lumaAVX2 runs the kernel for the position. The caller has proved the
// footprint (checkLuma), so the two pointers and the loops inside the
// assembly stay within dst and src: the kernels read and write exactly the
// bytes of the block and of its footprint, never a whole vector past them.
func lumaAVX2(dst []byte, dstOff, dstStride int, src []byte, srcOff, srcStride, w, h, xFrac, yFrac int, avg bool) {
	a, class := 0, 2
	if avg {
		a = 1
	}
	switch w {
	case 4:
		class = 0
	case 8:
		class = 1
	}
	lumaKernels[a][class][xFrac+4*yFrac](&dst[dstOff], dstStride, &src[srcOff], srcStride, w, h)
}
