package mc

import "testing"

// dst and src sharing memory is refused; disjoint parts of one array are not.
func TestSharedMemoryRefused(t *testing.T) {
	buf := make([]byte, 4096)
	same := func(f func(dst []byte, dstOff, dstStride int, src []byte, srcOff, srcStride, w, h, xFrac, yFrac int)) {
		// One slice for both: the destination block is inside the footprint.
		mustPanic(t, "same slice", func() { f(buf, 100, 64, buf, 100, 64, 8, 8, 0, 0) })
		mustPanic(t, "same slice, footprint", func() { f(buf, 200, 64, buf, 100, 64, 8, 8, 2, 2) })
		// Two views of one array that overlap, with different offsets.
		mustPanic(t, "views overlap", func() { f(buf[:2048], 0, 64, buf[100:], 0, 64, 8, 8, 0, 0) })
		// Disjoint halves of one array, and the byte just after / before.
		f(buf[:2048], 0, 64, buf[2048:], 3*64+3, 64, 8, 8, 2, 2)
		f(buf[:7*64+8], 0, 64, buf[7*64+8:], 0, 64, 8, 8, 0, 0)
		f(buf[2048:], 0, 64, buf[:2048], 3*64+3, 64, 8, 8, 2, 2)
		mustPanic(t, "one byte of overlap", func() { f(buf[:7*64+8], 0, 64, buf[7*64+7:], 0, 64, 8, 8, 0, 0) })
	}
	same(PutLuma)
	same(AvgLuma)

	bi := func(d []byte, dOff int, s []byte, sOff int) {
		WeightBi(d, dOff, 64, s, sOff, 64, 8, 8, 21, 43, 2, 3, 5)
	}
	mustPanic(t, "bi same slice", func() { bi(buf, 100, buf, 100) })
	mustPanic(t, "bi interleaved rows", func() { bi(buf, 0, buf, 64) })
	mustPanic(t, "bi average path", func() { WeightBi(buf, 0, 64, buf, 64, 64, 8, 8, 32, 32, 0, 0, 5) })
	bi(buf[:2048], 0, buf[2048:], 0)
	bi(buf, 0, buf, 7*64+8)
	mustPanic(t, "bi one byte", func() { bi(buf, 0, buf, 7*64+7) })
}
