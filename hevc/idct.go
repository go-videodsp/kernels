package hevc

import (
	"unsafe"
)

// The two-dimensional inverse transform of ITU-T H.265 (V11) (01/2026), clause 8.6.4.2, of
// one block of coefficients, columns first (Equation 8-314 clips the intermediate to 16 bits)
// then rows, the final shift (8-299) included: from the scaled coefficients d to residual
// samples r, in 32-bit arithmetic. The coefficients are within 16 bits (7-27), so no sum of
// at most 32 products of a matrix entry (at most 90) overflows 32 bits and the result is the
// one of the 64-bit evaluation the text gives.

const (
	maxCoeff = 1<<15 - 1
	minCoeff = -(1 << 15)
	// the final shift of the residual (8-299), bdShift = 20 - bitDepth, is from 8 (12-bit video)
	// to 12 (8-bit); a few more are taken for the transforms whose callers shift otherwise.
	minIDCTShift = 1
	maxIDCTShift = 24
)

// idctExtent checks the block and finds the largest column and row that hold a non-zero
// coefficient (-1 and -1 for an all-zero block).
func idctExtent(dst, d []int32, n, bdShift int) (maxX, maxY int, err error) {
	switch {
	case n*n > len(d) || n*n > len(dst):
		return 0, 0, refuse("a block of %dx%d needs %d coefficients and %d residual samples, got %d and %d", n, n, n*n, n*n, len(d), len(dst))
	case bdShift < minIDCTShift || bdShift > maxIDCTShift:
		return 0, 0, refuse("shift %d is not from %d to %d", bdShift, minIDCTShift, maxIDCTShift)
	}
	d, dst = d[:n*n], dst[:n*n]
	if overlaps(unsafe.Pointer(unsafe.SliceData(dst)), 0, n*n-1, 4, unsafe.Pointer(unsafe.SliceData(d)), 0, n*n-1, 4) {
		return 0, 0, refuse("residual samples and coefficients share memory")
	}
	maxX, maxY = -1, -1
	for y := 0; y < n; y++ {
		for x, v := range d[y*n : y*n+n] {
			if v < minCoeff || v > maxCoeff {
				return 0, 0, refuse("coefficient %d is not within 16 bits", v)
			}
			if v != 0 {
				maxX, maxY = max(maxX, x), y
			}
		}
	}
	return maxX, maxY, nil
}

// IDCT writes the n x n residual block of the inverse DCT of the coefficients d (both
// indexed y*n + x, n = 1 << log2Size from 4 to 32):
//
//	g[i][x] = Clip3( -32768, 32767, ( sum over j of transMatrix[j][i] * d[j][x] + 64 ) >> 7 )
//	dst[y][i] = ( sum over k of transMatrix[k][i] * g[y][k] + 2^(bdShift-1) ) >> bdShift
//
// (8-314 to 8-317 and 8-299). Every coefficient is within 16 bits; bdShift is from 1 to 24
// (the text's is 20 - bitDepth); dst and d must not share memory. IDCT panics where TryIDCT
// returns an error, before anything is written.
func IDCT(dst, d []int32, log2Size, bdShift int) {
	if err := TryIDCT(dst, d, log2Size, bdShift); err != nil {
		panic(err)
	}
}

// TryIDCT is IDCT returning the refusal as an error, with nothing written.
func TryIDCT(dst, d []int32, log2Size, bdShift int) error {
	if log2Size < 2 || log2Size > 5 {
		return refuse("log2 of the block size is %d, not from 2 to 5", log2Size)
	}
	n := 1 << uint(log2Size)
	maxX, maxY, err := idctExtent(dst, d, n, bdShift)
	if err != nil {
		return err
	}
	rnd := int32(1) << uint(bdShift-1)
	switch {
	case maxX < 0:
		for i := 0; i < n*n; i++ {
			dst[i] = 0
		}
	case maxX == 0 && maxY == 0: // only the DC coefficient: every sample is the same
		v := min(max((64*d[0]+64)>>7, minCoeff), maxCoeff)
		s := (64*v + rnd) >> uint(bdShift)
		for i := 0; i < n*n; i++ {
			dst[i] = s
		}
	default:
		// the intermediate block, of the size the block needs: zeroing 4 KB for a 4 x 4 block
		// would cost more than the transform
		switch log2Size {
		case 2:
			var g [16]int32
			idctDispatch(dst, d, g[:], dctMatrix[:], 32*(32/n), n, maxX, maxY, uint(bdShift))
		case 3:
			var g [64]int32
			idctDispatch(dst, d, g[:], dctMatrix[:], 32*(32/n), n, maxX, maxY, uint(bdShift))
		case 4:
			var g [256]int32
			idctDispatch(dst, d, g[:], dctMatrix[:], 32*(32/n), n, maxX, maxY, uint(bdShift))
		default:
			var g [1024]int32
			idctDispatch(dst, d, g[:], dctMatrix[:], 32*(32/n), n, maxX, maxY, uint(bdShift))
		}
	}
	return nil
}

// IDST4 is IDCT for the 4 x 4 block of the 4-point inverse DST (8-316), which the text uses
// for the luma of an intra 4 x 4 block.
func IDST4(dst, d []int32, bdShift int) {
	if err := TryIDST4(dst, d, bdShift); err != nil {
		panic(err)
	}
}

// TryIDST4 is IDST4 returning the refusal as an error, with nothing written.
func TryIDST4(dst, d []int32, bdShift int) error {
	maxX, maxY, err := idctExtent(dst, d, 4, bdShift)
	if err != nil {
		return err
	}
	if maxX < 0 {
		for i := 0; i < 16; i++ {
			dst[i] = 0
		}
		return nil
	}
	var g [16]int32
	idctDispatch(dst, d, g[:], dst4Matrix[:], 4, 4, maxX, maxY, uint(bdShift))
	return nil
}
