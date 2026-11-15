//go:build amd64 && !purego

package mc

import (
	"bytes"
	"math/rand"
	"os"
	"testing"

	"github.com/go-videodsp/kernels/cpu"
)

// The direct AVX2 partition loops and the per-block dispatch loops must give the
// same macroblock; the direct ones are what runs where AVX2 exists, the
// dispatch ones everywhere else on amd64 and on the CI lanes this proves them.
// On a GitHub runner (all have AVX2) a missing AVX2 is a failure, never a skip.
func TestWeightMBDirectLoopsEqualDispatchLoops(t *testing.T) {
	required := os.Getenv("GITHUB_ACTIONS") == "true" || os.Getenv("VIDEODSP_REQUIRE_AVX2") == "1"
	if !cpu.HasAVX2 {
		if required {
			t.Fatal("AVX2 is required here and the cpu probe says no")
		}
		t.Skip("no AVX2 on this host")
	}
	r := rand.New(rand.NewSource(51))
	for i := 0; i < 2000; i++ {
		logWD := []int{5, r.Intn(8)}[r.Intn(2)]
		ds, ss := 16+r.Intn(30), 16+r.Intn(30)
		dOff, sOff := r.Intn(2*ds), r.Intn(2*ss)
		init := make([]byte, dOff+15*ds+16)
		src := make([]byte, sOff+15*ss+16)
		fill(r, init, fillRandom, ds)
		fill(r, src, fillRandom, ss)
		uni, bi := uniParts(r, mbLayout(r)), biParts(r, mbLayout(r))

		a, b := append([]byte(nil), init...), append([]byte(nil), init...)
		weightUniPartsAVX2(a, dOff, ds, uni, logWD)
		weightUniPartsDispatch(b, dOff, ds, uni, logWD)
		if !bytes.Equal(a, b) {
			t.Fatalf("uni: direct and dispatch loops differ at %d", firstDiff(a, b))
		}
		a, b = append([]byte(nil), init...), append([]byte(nil), init...)
		weightBiPartsAVX2(a, dOff, ds, src, sOff, ss, bi, logWD)
		weightBiPartsDispatch(b, dOff, ds, src, sOff, ss, bi, logWD)
		if !bytes.Equal(a, b) {
			t.Fatalf("bi: direct and dispatch loops differ at %d", firstDiff(a, b))
		}
	}
	// No partitions: nothing to do, nothing touched.
	z := []byte{1, 2, 3}
	weightUniPartsAVX2(z, 0, 16, nil, 2)
	weightBiPartsAVX2(z, 0, 16, z, 0, 16, nil, 2)
	if !bytes.Equal(z, []byte{1, 2, 3}) {
		t.Error("an empty list wrote")
	}
}
