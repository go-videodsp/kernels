package mc

import (
	"math/rand"
	"testing"
)

func panics(f func()) (p bool) {
	defer func() { p = recover() != nil }()
	f()
	return
}

// The fast gate of WeightUni and WeightBi (blockFits, weightOK) must accept
// exactly what the detailed checks accept: a gate looser than the checks would
// let an unproved block reach the assembly, a stricter one would refuse a legal
// call. Compared on a dense grid of sizes, offsets, strides and parameters,
// including the edges of every range.
func TestFastGateEqualsChecks(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	sizesAll := [][2]int{{16, 16}, {16, 8}, {8, 16}, {8, 8}, {8, 4}, {4, 8}, {4, 4}, {2, 4}, {12, 8}, {16, 4}, {4, 16}, {0, 0}, {-4, 8}}
	for i := 0; i < 200000; i++ {
		sz := sizesAll[rng.Intn(len(sizesAll))]
		w, h := sz[0], sz[1]
		n := rng.Intn(600)
		stride := rng.Intn(40) - 2
		off := rng.Intn(n+8) - 4
		if rng.Intn(4) == 0 && w > 0 && h > 0 && stride >= w { // the tight case
			off = n - ((h-1)*stride + w) + rng.Intn(3) - 1
		}
		if rng.Intn(50) == 0 {
			stride = 1<<62 + rng.Intn(3) // would overflow a plain int product
		}
		logWD := rng.Intn(11) - 2
		weight := []int{-129, -128, 0, 127, 128, 129, rng.Intn(300) - 150}[rng.Intn(7)]
		offset := []int{-129, -128, 0, 127, 128, rng.Intn(300) - 150}[rng.Intn(6)]

		gate := blockFits(n, off, stride, w, h) && weightOK(logWD, weight, offset)
		checks := !panics(func() {
			checkBlock("destination", n, off, stride, w, h)
			checkWeights(logWD, weight, offset)
		})
		if gate != checks {
			t.Fatalf("n=%d off=%d stride=%d %dx%d logWD=%d w=%d o=%d: gate %v, checks %v", n, off, stride, w, h, logWD, weight, offset, gate, checks)
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
