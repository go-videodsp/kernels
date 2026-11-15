//go:build unix

package mc

import (
	"bytes"
	"fmt"
	"math/rand"
	"runtime/debug"
	"testing"
)

// PutChroma and PutChromaPair with the planes placed so that they end (or
// start) exactly at a PROT_NONE page: any access outside the (w+1) x (h+1)
// source footprint or the w x h destination block faults.
func TestChromaGuardPages(t *testing.T) {
	old := debug.SetPanicOnFault(true)
	defer debug.SetPanicOnFault(old)
	r := rand.New(rand.NewSource(23))
	for _, atEnd := range []bool{true, false} {
		for _, sz := range chromaSizes {
			w, h := sz[0], sz[1]
			t.Run(fmt.Sprintf("end=%v/%dx%d", atEnd, w, h), func(t *testing.T) {
				for _, extra := range []int{0, 5} {
					ss, ds := w+1+extra, w+extra
					sn, dn := h*ss+w+1, (h-1)*ds+w
					for _, f := range [][2]int{{0, 0}, {3, 5}, {7, 7}, {0, 4}, {6, 0}} {
						sInit0, sInit1 := chromaPlane(r, sn), chromaPlane(r, sn)
						dInit0, dInit1 := chromaPlane(r, dn), chromaPlane(r, dn)
						s0, s1 := guarded(t, sn, atEnd), guarded(t, sn, atEnd)
						d0, d1 := guarded(t, dn, atEnd), guarded(t, dn, atEnd)
						copy(s0, sInit0)
						copy(s1, sInit1)
						copy(d0, dInit0)
						copy(d1, dInit1)
						PutChromaPair(d0, d1, 0, ds, s0, s1, 0, ss, w, h, f[0], f[1])
						w0, w1 := append([]byte(nil), dInit0...), append([]byte(nil), dInit1...)
						putChromaRef(w0, 0, ds, sInit0, 0, ss, w, h, f[0], f[1])
						putChromaRef(w1, 0, ds, sInit1, 0, ss, w, h, f[0], f[1])
						if !bytes.Equal(d0, w0) || !bytes.Equal(d1, w1) {
							t.Fatalf("pair frac %v extra %d: planes differ", f, extra)
						}
						d := guarded(t, dn, atEnd)
						copy(d, dInit0)
						PutChroma(d, 0, ds, s0, 0, ss, w, h, f[0], f[1])
						if !bytes.Equal(d, w0) {
							t.Fatalf("single frac %v extra %d: first difference at %d", f, extra, chromaDiff(d, w0))
						}
					}
				}
			})
		}
	}
}
