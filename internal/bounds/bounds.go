// Package bounds holds the one piece of arithmetic every kernel's bounds proof
// rests on: whether a window of a strided plane lies inside a slice.
//
// The proof used to multiply the stride by the number of rows in int64. A
// stride of 1<<62 and four rows wraps that product to a negative number, the
// "last index" then looked small, and the assembly ran far outside the slice
// (a memory-safety defect, v0.1.0 to v0.3.0). Window never multiplies two
// values it has not first compared with the slice length, and does the one
// product it needs with bits.Mul64 and a refusal when the high word is not 0,
// so no step can wrap on any target.
package bounds

import "math/bits"

// Window reports whether every index
//
//	off - rowsBefore*stride - colsBefore  ..  off + rowsAfter*stride + colsAfter
//
// lies in [0, n), and returns the first and the last of them. Every argument
// must be non-negative and off must be a valid index; anything else (a negative
// value, an offset or a size outside the slice, a stride whose product with the
// row count does not fit, or exceeds the slice) reports false with no panic and
// no arithmetic that can wrap.
func Window(n, off, stride, rowsBefore, rowsAfter, colsBefore, colsAfter int) (first, last int, ok bool) {
	if n <= 0 || off < 0 || off >= n || stride < 0 || rowsBefore < 0 || rowsAfter < 0 || colsBefore < 0 || colsAfter < 0 {
		return 0, 0, false
	}
	un, uoff := uint64(n), uint64(off)

	hi, lo := bits.Mul64(uint64(stride), uint64(rowsBefore))
	if hi != 0 || lo > uoff {
		return 0, 0, false
	}
	// lo <= off < 2^63 and colsBefore < 2^63: the sum cannot wrap.
	back := lo + uint64(colsBefore)
	if back > uoff {
		return 0, 0, false
	}

	hi, lo = bits.Mul64(uint64(stride), uint64(rowsAfter))
	if hi != 0 || lo >= un {
		return 0, 0, false
	}
	// lo < n < 2^63 and colsAfter < 2^63: the sum cannot wrap.
	fwd := lo + uint64(colsAfter)
	if fwd >= un-uoff {
		return 0, 0, false
	}
	return int(uoff - back), int(uoff + fwd), true
}
