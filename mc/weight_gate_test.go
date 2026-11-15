package mc

import (
	"math"
	"math/rand"
	"testing"

	"github.com/go-videodsp/kernels/internal/boundstest"
)

func panics(f func()) (p bool) {
	defer func() { p = recover() != nil }()
	f()
	return
}

// The fast gate of WeightUni and WeightBi must accept exactly what the exact
// arithmetic accepts. blockFits is judged by internal/boundstest (math/big, no
// wrap possible), not by a second copy of int arithmetic (that comparison
// missed the overflow once); weightOK by checkWeights. Slice lengths include
// ones above safeLen (a plane too large to allocate here is still just an int),
// strides and offsets include MaxInt and its halves.
func TestFastGateEqualsExactOracle(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	sizesAll := [][2]int{{16, 16}, {16, 8}, {8, 16}, {8, 8}, {8, 4}, {4, 8}, {4, 4}, {2, 4}, {12, 8}, {16, 4}, {4, 16}, {0, 0}, {-4, 8}}
	huge := []int{math.MaxInt, math.MaxInt - 1, math.MaxInt / 2, math.MaxInt/2 + 1, math.MaxInt / 16, math.MaxInt / 31, math.MaxInt/32 - 1, math.MaxInt / 32, math.MaxInt/32 + 1, -1, math.MinInt}
	pickHuge := func() int { return huge[rng.Intn(len(huge))] }
	for i := 0; i < 300000; i++ {
		sz := sizesAll[rng.Intn(len(sizesAll))]
		w, h := sz[0], sz[1]
		n := rng.Intn(600)
		stride := rng.Intn(40) - 2
		off := rng.Intn(n+8) - 4
		if rng.Intn(4) == 0 && w > 0 && h > 0 && stride >= w { // the tight case
			off = n - ((h-1)*stride + w) + rng.Intn(3) - 1
		}
		switch rng.Intn(6) {
		case 0:
			stride = pickHuge()
		case 1:
			off = pickHuge()
		case 2:
			n = pickHuge()
		case 3:
			n, off, stride = pickHuge(), pickHuge(), pickHuge()
		}
		want := validSize(w, h) && stride >= w && boundstest.Fits(n, off, stride, 0, h-1, 0, w-1)
		if got := blockFits(n, off, stride, w, h); got != want {
			t.Fatalf("blockFits(n=%d off=%d stride=%d %dx%d) = %v, exact oracle %v", n, off, stride, w, h, got, want)
		}
		logWD := rng.Intn(11) - 2
		weight := []int{-129, -128, 0, 127, 128, 129, rng.Intn(300) - 150}[rng.Intn(7)]
		offset := []int{-129, -128, 0, 127, 128, rng.Intn(300) - 150}[rng.Intn(6)]
		if gate, checks := weightOK(logWD, weight, offset), !panics(func() { checkWeights(logWD, weight, offset) }); gate != checks {
			t.Fatalf("weightOK(%d, %d, %d) = %v, checkWeights %v", logWD, weight, offset, gate, checks)
		}
	}
}

// The hostile values of the security fix, on the inner gate itself: a stride or
// an offset of MaxInt/2+1 (1<<62 on 64-bit), MaxInt or -1 is refused whatever
// the slice length, and an accepted block's last index is the exact one.
func TestBlockLastHostileAndExact(t *testing.T) {
	for _, v := range []int{math.MaxInt/2 + 1, math.MaxInt, math.MaxInt - 1, -1, math.MinInt} {
		for _, sz := range sizes {
			w, h := sz[0], sz[1]
			for _, n := range []int{64, 1 << 16, math.MaxInt} {
				if _, ok := blockLast(n, 0, v, w, h); ok && boundstest.Fits(n, 0, v, 0, h-1, 0, w-1) == false {
					t.Errorf("stride %d n=%d %dx%d accepted", v, n, w, h)
				}
				if v < 0 || v > n {
					if _, ok := blockLast(n, 0, v, w, h); ok {
						t.Errorf("stride %d n=%d %dx%d accepted though it cannot fit", v, n, w, h)
					}
				}
				if _, ok := blockLast(n, v, 16, w, h); ok && !boundstest.Fits(n, v, 16, 0, h-1, 0, w-1) {
					t.Errorf("offset %d n=%d %dx%d accepted", v, n, w, h)
				}
			}
		}
	}
	for _, sz := range sizes {
		w, h := sz[0], sz[1]
		last, ok := blockLast(1000, 10, 40, w, h)
		if !ok || last != 10+(h-1)*40+w-1 {
			t.Errorf("%dx%d: last %d ok %v", w, h, last, ok)
		}
	}
}

// A call the gate refuses always panics, and the refusal helpers panic even
// when the detailed checks find nothing (a disagreement is still a refusal).
func TestRefuseAlwaysPanics(t *testing.T) {
	if !panics(func() { refuseUni(64, 0, 8, 8, 8, 1, 0, 0) }) {
		t.Error("refuseUni returned on a legal call")
	}
	if !panics(func() { refuseUni(64, 0, 8, 8, 8, 999, 0, 0) }) {
		t.Error("refuseUni returned on a bad weight")
	}
	if !panics(func() { refuseBi(64, 0, 8, 64, 0, 8, 8, 8, 1, 1, 0, 0, 0) }) {
		t.Error("refuseBi returned on a legal call")
	}
	if !panics(func() { refuseBi(64, 0, 8, 63, 0, 8, 8, 8, 1, 1, 0, 0, 0) }) {
		t.Error("refuseBi returned on a short source")
	}
}
