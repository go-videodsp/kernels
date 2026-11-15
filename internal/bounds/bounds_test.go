package bounds

import (
	"math"
	"math/big"
	"math/rand"
	"testing"
)

// oracle is the definition of Window in arbitrary-precision integers: nothing
// in it can wrap, so it shares no arithmetic with the code under test.
func oracle(n, off, stride, rb, ra, cb, ca int) (first, last int64, ok bool) {
	b := func(v int) *big.Int { return big.NewInt(int64(v)) }
	for _, v := range []int{stride, rb, ra, cb, ca} {
		if v < 0 {
			return 0, 0, false
		}
	}
	if n <= 0 || off < 0 || off >= n {
		return 0, 0, false
	}
	f := new(big.Int).Sub(b(off), new(big.Int).Add(new(big.Int).Mul(b(stride), b(rb)), b(cb)))
	l := new(big.Int).Add(b(off), new(big.Int).Add(new(big.Int).Mul(b(stride), b(ra)), b(ca)))
	if f.Sign() < 0 || l.Cmp(b(n)) >= 0 {
		return 0, 0, false
	}
	return f.Int64(), l.Int64(), true
}

var edges = []int{0, 1, 2, 3, 4, 7, 8, 15, 16, 19, 64, 1 << 15, math.MaxInt32, math.MaxInt/2 - 1, math.MaxInt / 2, math.MaxInt/2 + 1, math.MaxInt/2 + 2, math.MaxInt - 1, math.MaxInt, -1, -2, math.MinInt}

func check(t *testing.T, n, off, stride, rb, ra, cb, ca int) {
	t.Helper()
	wf, wl, wok := oracle(n, off, stride, rb, ra, cb, ca)
	f, l, ok := Window(n, off, stride, rb, ra, cb, ca)
	if ok != wok || (ok && (int64(f) != wf || int64(l) != wl)) {
		t.Fatalf("Window(n=%d off=%d stride=%d rb=%d ra=%d cb=%d ca=%d) = %d %d %v, oracle %d %d %v", n, off, stride, rb, ra, cb, ca, f, l, ok, wf, wl, wok)
	}
}

// The wrap that was the defect: 1<<62 * 4 is negative in int64.
func TestHugeStrideIsRefused(t *testing.T) {
	if _, _, ok := Window(64, 0, math.MaxInt/2+1, 0, 3, 0, 7); ok { // 1<<62 on 64-bit targets
		t.Fatal("stride MaxInt/2+1 with four rows was accepted")
	}
	if _, _, ok := Window(64, 63, math.MaxInt, 4, 0, 0, 0); ok {
		t.Fatal("stride MaxInt with four rows before was accepted")
	}
}

func TestWindowAgainstBigOracleOnEdges(t *testing.T) {
	small := []int{0, 1, 3, 7, 15, 19}
	for _, n := range []int{-1, 0, 1, 64, 1 << 20} {
		for _, off := range edges {
			for _, stride := range edges {
				for _, rb := range small {
					for _, ra := range small {
						check(t, n, off, stride, rb, ra, 2, 3)
					}
				}
			}
		}
	}
	// Every argument from the edge set, including the negative and huge sizes.
	for _, n := range edges {
		for _, off := range edges {
			for _, stride := range edges {
				for _, rb := range edges {
					check(t, n, off, stride, rb, 3, 2, 3)
					check(t, n, off, stride, 3, rb, rb, 1)
				}
			}
		}
	}
}

func TestWindowAgainstBigOracleRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(9))
	pick := func() int {
		switch rng.Intn(4) {
		case 0:
			return edges[rng.Intn(len(edges))]
		case 1:
			return rng.Intn(100)
		case 2:
			return int(rng.Int63())
		}
		return rng.Intn(1 << 20)
	}
	for i := 0; i < 400000; i++ {
		check(t, pick(), pick(), pick(), rng.Intn(20), rng.Intn(20), rng.Intn(5), rng.Intn(20))
	}
}

func FuzzWindow(f *testing.F) {
	f.Add(64, 0, 8, 0, 7, 0, 7)
	f.Add(64, 0, math.MaxInt/2+1, 0, 3, 0, 7)
	f.Add(math.MaxInt, math.MaxInt-1, math.MaxInt, 4, 4, 4, 4)
	f.Fuzz(func(t *testing.T, n, off, stride, rb, ra, cb, ca int) {
		check(t, n, off, stride, rb, ra, cb, ca)
	})
}
