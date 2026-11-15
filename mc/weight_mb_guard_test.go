//go:build unix

package mc

import (
	"bytes"
	"math/rand"
	"runtime/debug"
	"testing"
)

// The macroblock entries with the destination and the source placed so that
// the proved window ends (or starts) exactly at an inaccessible page: any access
// outside the 16x16 window, whatever the partition layout, faults.
func TestWeightMBGuardPages(t *testing.T) {
	old := debug.SetPanicOnFault(true)
	defer debug.SetPanicOnFault(old)
	r := rand.New(rand.NewSource(41))
	for _, atEnd := range []bool{true, false} {
		for _, stride := range []int{16, 17, 64} {
			n := 15*stride + 16 // the tight window: first sample to last sample
			for round := 0; round < 60; round++ {
				logWD := r.Intn(8)
				initDst := make([]byte, n)
				initSrc := make([]byte, n)
				fill(r, initDst, fillRandom, stride)
				fill(r, initSrc, fillRandom, stride)

				dst := guarded(t, n, atEnd)
				copy(dst, initDst)
				uni := uniParts(r, mbLayout(r))
				wantU := append([]byte(nil), initDst...)
				for _, p := range uni {
					for y := 0; y < p.H; y++ {
						for x := 0; x < p.W; x++ {
							j := (p.Y+y)*stride + p.X + x
							wantU[j] = weightedUni(int(wantU[j]), p.Weight, p.Offset, logWD)
						}
					}
				}
				WeightUniMB(dst, 0, stride, uni, logWD)
				if !bytes.Equal(dst, wantU) {
					t.Fatalf("uni atEnd=%v stride=%d: first difference at %d", atEnd, stride, firstDiff(dst, wantU))
				}

				dst2, src := guarded(t, n, atEnd), guarded(t, n, atEnd)
				copy(dst2, initDst)
				copy(src, initSrc)
				bi := biParts(r, mbLayout(r))
				wantB := append([]byte(nil), initDst...)
				for _, p := range bi {
					for y := 0; y < p.H; y++ {
						for x := 0; x < p.W; x++ {
							j := (p.Y+y)*stride + p.X + x
							wantB[j] = weightedBi(int(wantB[j]), int(initSrc[j]), p.W0, p.W1, p.O0, p.O1, logWD)
						}
					}
				}
				WeightBiMB(dst2, 0, stride, src, 0, stride, bi, logWD)
				if !bytes.Equal(dst2, wantB) {
					t.Fatalf("bi atEnd=%v stride=%d: first difference at %d", atEnd, stride, firstDiff(dst2, wantB))
				}
			}
		}
	}
}
