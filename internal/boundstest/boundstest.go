// Package boundstest is the arbitrary-precision oracle of the bounds proofs,
// for the tests of mc and lf. It uses math/big, so no step of it can wrap and
// it shares no arithmetic with the int code it judges.
package boundstest

import "math/big"

// Fits reports, in exact integers, whether every index
// off - rowsBefore*stride - colsBefore .. off + rowsAfter*stride + colsAfter lies in [0, n).
func Fits(n, off, stride, rowsBefore, rowsAfter, colsBefore, colsAfter int) bool {
	b := func(v int) *big.Int { return big.NewInt(int64(v)) }
	back := new(big.Int).Add(new(big.Int).Mul(b(stride), b(rowsBefore)), b(colsBefore))
	fwd := new(big.Int).Add(new(big.Int).Mul(b(stride), b(rowsAfter)), b(colsAfter))
	first := new(big.Int).Sub(b(off), back)
	last := new(big.Int).Add(b(off), fwd)
	return first.Sign() >= 0 && last.Cmp(b(n)) < 0
}
