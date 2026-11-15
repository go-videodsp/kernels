//go:build unix

package mc

import (
	"bytes"
	"fmt"
	"math/rand"
	"runtime/debug"
	"testing"
)

// WeightUni and WeightBi with the planes placed so that they end (or start)
// exactly at a PROT_NONE page: any access outside the w x h block faults.
// Parameters cover the single-list path, the two-list path and the averaging
// shortcut.
func TestWeightGuardPages(t *testing.T) {
	old := debug.SetPanicOnFault(true)
	defer debug.SetPanicOnFault(old)
	r := rand.New(rand.NewSource(21))
	params := []struct{ w0, w1, o0, o1, logWD int }{
		{45, 0, 3, 0, 5}, {-128, 0, 127, 0, 0}, {128, 0, -128, 0, 7}, // uni
		{21, 43, 2, 3, 5}, {-64, 128, 0, -9, 6}, {127, -128, 127, 127, 0}, // bi
		{32, 32, 0, 0, 5}, {1, 1, 0, 0, 0}, {128, 128, 0, -1, 7}, // the average (32,32 among them)
	}
	for _, atEnd := range []bool{true, false} {
		for _, sz := range sizes {
			w, h := sz[0], sz[1]
			t.Run(fmt.Sprintf("end=%v/%dx%d", atEnd, w, h), func(t *testing.T) {
				for _, extra := range []int{0, 5} {
					stride := w + extra
					n := (h-1)*stride + w
					dInit := make([]byte, n)
					sInit := make([]byte, n)
					fill(r, dInit, fillRandom, 1)
					fill(r, sInit, fillRandom, 1)
					for _, p := range params {
						// Single list, then two lists.
						d := guarded(t, n, atEnd)
						copy(d, dInit)
						WeightUni(d, 0, stride, w, h, p.w0, p.o0, p.logWD)
						want := append([]byte(nil), dInit...)
						for y := 0; y < h; y++ {
							for x := 0; x < w; x++ {
								want[y*stride+x] = weightedUni(int(dInit[y*stride+x]), p.w0, p.o0, p.logWD)
							}
						}
						if !bytes.Equal(d, want) {
							t.Fatalf("uni %+v extra %d: first difference at %d", p, extra, firstDiff(d, want))
						}

						d = guarded(t, n, atEnd)
						copy(d, dInit)
						s := guarded(t, n, atEnd)
						copy(s, sInit)
						WeightBi(d, 0, stride, s, 0, stride, w, h, p.w0, p.w1, p.o0, p.o1, p.logWD)
						for y := 0; y < h; y++ {
							for x := 0; x < w; x++ {
								i := y*stride + x
								want[i] = weightedBi(int(dInit[i]), int(sInit[i]), p.w0, p.w1, p.o0, p.o1, p.logWD)
							}
						}
						if !bytes.Equal(d, want) {
							t.Fatalf("bi %+v extra %d: first difference at %d", p, extra, firstDiff(d, want))
						}
					}
				}
			})
		}
	}
}
