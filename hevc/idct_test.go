package hevc

import (
	"errors"
	"math/rand"
	"testing"
)

// modelTransform is the text's two-dimensional transform (8.6.4.2, columns first, the
// intermediate clip of 8-314, then rows, then the shift of 8-299) in 64 bits, from the matrix
// entries alone: m(j, i) is the basis function of frequency j at position i.
func modelTransform(d []int32, n, bdShift int, m func(j, i int) int64) []int32 {
	g := make([]int64, n*n)
	for x := 0; x < n; x++ {
		for i := 0; i < n; i++ {
			var s int64
			for j := 0; j < n; j++ {
				s += m(j, i) * int64(d[j*n+x])
			}
			g[i*n+x] = min(max((s+64)>>7, -32768), 32767)
		}
	}
	out := make([]int32, n*n)
	for y := 0; y < n; y++ {
		for i := 0; i < n; i++ {
			var s int64
			for k := 0; k < n; k++ {
				s += m(k, i) * g[y*n+k]
			}
			out[y*n+i] = int32((s + 1<<uint(bdShift-1)) >> uint(bdShift))
		}
	}
	return out
}

func dctEntry(n int) func(j, i int) int64 {
	return func(j, i int) int64 { return int64(dctMatrix[j*(32/n)*32+i]) }
}

func dstEntry(j, i int) int64 { return int64(dst4Matrix[j*4+i]) }

// randomCoefficients fills a block of n x n with the shapes of a decoder's blocks: a few
// non-zero coefficients near the top left, a dense block, the extremes, one coefficient.
func randomCoefficients(rng *rand.Rand, n, kind int) []int32 {
	d := make([]int32, n*n)
	switch kind {
	case 0: // sparse, low frequencies
		for k := 0; k < 1+rng.Intn(6); k++ {
			d[rng.Intn(1+rng.Intn(n))*n+rng.Intn(1+rng.Intn(n))] = int32(rng.Intn(2001) - 1000)
		}
	case 1: // dense
		for i := range d {
			d[i] = int32(rng.Intn(65536) - 32768)
		}
	case 2: // the extremes: the largest sums
		for i := range d {
			d[i] = [2]int32{-32768, 32767}[rng.Intn(2)]
		}
	case 3: // one coefficient anywhere
		d[rng.Intn(n*n)] = int32(rng.Intn(65536) - 32768)
	default: // the last row or column only
		if rng.Intn(2) == 0 {
			for x := 0; x < n; x++ {
				d[(n-1)*n+x] = int32(rng.Intn(4001) - 2000)
			}
		} else {
			for y := 0; y < n; y++ {
				d[y*n+n-1] = int32(rng.Intn(4001) - 2000)
			}
		}
	}
	return d
}

func TestIDCTAgainstTheText(t *testing.T) {
	rng := rand.New(rand.NewSource(51))
	seen := map[int]int{}
	for iter := 0; iter < 6000; iter++ {
		log2 := 2 + rng.Intn(4)
		n := 1 << uint(log2)
		bdShift := 8 + rng.Intn(5)
		if iter%11 == 0 {
			bdShift = 1 + rng.Intn(24)
		}
		d := randomCoefficients(rng, n, iter%5)
		dst := make([]int32, n*n+rng.Intn(5))
		for i := range dst {
			dst[i] = int32(sentinel16)
		}
		if err := TryIDCT(dst, d, log2, bdShift); err != nil {
			t.Fatalf("iter %d: %v", iter, err)
		}
		want := modelTransform(d, n, bdShift, dctEntry(n))
		for i, w := range want {
			if dst[i] != w {
				t.Fatalf("iter %d: %dx%d shift %d kind %d: sample %d is %d, the text gives %d", iter, n, n, bdShift, iter%5, i, dst[i], w)
			}
		}
		for i := n * n; i < len(dst); i++ {
			if dst[i] != int32(sentinel16) {
				t.Fatalf("iter %d: a sample past the block was written", iter)
			}
		}
		seen[n]++
	}
	for _, n := range []int{4, 8, 16, 32} {
		if seen[n] == 0 {
			t.Errorf("no %dx%d block", n, n)
		}
	}
}

func TestIDST4AgainstTheText(t *testing.T) {
	rng := rand.New(rand.NewSource(52))
	for iter := 0; iter < 3000; iter++ {
		bdShift := 8 + rng.Intn(5)
		d := randomCoefficients(rng, 4, iter%5)
		dst := make([]int32, 16)
		if err := TryIDST4(dst, d, bdShift); err != nil {
			t.Fatalf("iter %d: %v", iter, err)
		}
		want := modelTransform(d, 4, bdShift, dstEntry)
		for i, w := range want {
			if dst[i] != w {
				t.Fatalf("iter %d: sample %d is %d, the text gives %d", iter, i, dst[i], w)
			}
		}
	}
}

func TestIDCTEveryExtentOfEverySize(t *testing.T) {
	// one non-zero coefficient at every position of every size: the extents the kernels skip by
	for log2 := 2; log2 <= 5; log2++ {
		n := 1 << uint(log2)
		for pos := 0; pos < n*n; pos++ {
			d := make([]int32, n*n)
			d[pos] = 12345
			dst := make([]int32, n*n)
			IDCT(dst, d, log2, 12)
			want := modelTransform(d, n, 12, dctEntry(n))
			for i, w := range want {
				if dst[i] != w {
					t.Fatalf("%dx%d, coefficient at %d: sample %d is %d, the text gives %d", n, n, pos, i, dst[i], w)
				}
			}
		}
	}
}

func TestIDCTRefusals(t *testing.T) {
	dst := make([]int32, 1024)
	d := make([]int32, 1024)
	for name, f := range map[string]func() error{
		"log2 1":            func() error { return TryIDCT(dst, d, 1, 12) },
		"log2 6":            func() error { return TryIDCT(dst, d, 6, 12) },
		"short coefficient": func() error { return TryIDCT(dst, d[:15], 2, 12) },
		"short residual":    func() error { return TryIDCT(dst[:63], d, 3, 12) },
		"shift 0":           func() error { return TryIDCT(dst, d, 3, 0) },
		"shift 25":          func() error { return TryIDCT(dst, d, 3, 25) },
		"idst short":        func() error { return TryIDST4(dst[:15], d, 12) },
		"idst shift 0":      func() error { return TryIDST4(dst, d, 0) },
	} {
		for i := range dst {
			dst[i] = int32(sentinel16)
		}
		if err := f(); !errors.Is(err, ErrRefused) {
			t.Errorf("%s: %v", name, err)
		}
		for i := range dst {
			if dst[i] != int32(sentinel16) {
				t.Fatalf("%s: a refused call wrote dst[%d]", name, i)
			}
		}
	}
	for _, v := range []int32{32768, -32769} {
		d[5] = v
		if err := TryIDCT(dst, d, 3, 12); !errors.Is(err, ErrRefused) {
			t.Errorf("coefficient %d: %v", v, err)
		}
		if err := TryIDST4(dst, d, 12); !errors.Is(err, ErrRefused) {
			t.Errorf("idst coefficient %d: %v", v, err)
		}
	}
	d[5] = 0
	mem := make([]int32, 64)
	if err := TryIDCT(mem, mem, 3, 12); !errors.Is(err, ErrRefused) {
		t.Errorf("overlap: %v", err)
	}
	mustPanic(t, "IDCT", func() { IDCT(dst, d, 1, 12) })
	mustPanic(t, "IDST4", func() { IDST4(dst, d, 0) })
	IDCT(dst, d, 3, 12)
	IDST4(dst, d, 12)
}

// The portable reference computes the same two stages, and is what a target without a kernel
// runs (the all-zero and the DC-only blocks are answered before it).
func TestIDCTReferenceAgainstTheText(t *testing.T) {
	rng := rand.New(rand.NewSource(53))
	for iter := 0; iter < 400; iter++ {
		n := 4 << uint(rng.Intn(4))
		d := randomCoefficients(rng, n, iter%5)
		maxX, maxY := -1, -1
		for y := 0; y < n; y++ {
			for x := 0; x < n; x++ {
				if d[y*n+x] != 0 {
					maxX, maxY = max(maxX, x), y
				}
			}
		}
		if maxX < 0 {
			continue
		}
		got, g := make([]int32, n*n), make([]int32, n*n)
		idctRef(got, d, g, dctMatrix[:], 32*(32/n), n, maxX, maxY, 12)
		for i, w := range modelTransform(d, n, 12, dctEntry(n)) {
			if got[i] != w {
				t.Fatalf("%dx%d: sample %d is %d, the text gives %d", n, n, i, got[i], w)
			}
		}
	}
}

// The matrix is the text's: even rows symmetric, odd antisymmetric by construction, so what
// is left to check is the table itself: the basis functions are orthogonal to within the
// rounding of the integer entries, and a row's own product is the same for every row.
func TestTheDCTMatrixIsOrthogonalToItsRounding(t *testing.T) {
	for k := 0; k < 32; k++ {
		var self int64
		for m := 0; m < 32; m++ {
			self += int64(dctMatrix[k*32+m]) * int64(dctMatrix[k*32+m])
		}
		if self < 130000 || self > 132200 {
			t.Errorf("row %d: product with itself %d, about 131072 expected", k, self)
		}
		for l := k + 1; l < 32; l++ {
			var dot int64
			for m := 0; m < 32; m++ {
				dot += int64(dctMatrix[k*32+m]) * int64(dctMatrix[l*32+m])
			}
			if dot < -400 || dot > 400 {
				t.Errorf("rows %d and %d: product %d, about 0 expected", k, l, dot)
			}
		}
	}
	// the 4-, 8- and 16-point matrices are the rows 0, 8, 16, 24 (and so on) of this one
	if dctMatrix[8*32+0] != 83 || dctMatrix[8*32+1] != 36 || dctMatrix[16*32+0] != 64 || dctMatrix[16*32+1] != -64 {
		t.Error("the 4-point matrix is not in the rows 0, 8, 16, 24")
	}
	var dstSelf int64
	for m := 0; m < 4; m++ {
		dstSelf += int64(dst4Matrix[m]) * int64(dst4Matrix[m])
	}
	if dstSelf < 16000 || dstSelf > 16800 {
		t.Errorf("the DST row 0 has a product %d with itself, about 16384 expected", dstSelf)
	}
}
