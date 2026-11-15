package mc

import (
	"bytes"
	"math/rand"
	"testing"
)

// The Try forms refuse exactly when the plain forms panic, return the refusal
// as an error with nothing written, and on success write what the plain forms
// write. Random geometries around the edge of what fits, hostile strides and
// offsets, sizes outside the supported set, fractions outside 0..7, and
// destinations that share memory with sources or with each other.
func TestTryChromaAgreesWithThePanickingForms(t *testing.T) {
	r := rand.New(rand.NewSource(61))
	sizes := [][2]int{{8, 8}, {8, 2}, {2, 8}, {4, 4}, {2, 2}, {6, 4}, {4, 6}, {16, 8}, {8, 3}}
	nRefused, nAccepted := 0, 0
	for n := 0; n < 6000; n++ {
		sz := sizes[r.Intn(len(sizes))]
		w, h := sz[0], sz[1]
		ds, ss := w+r.Intn(6)-1, w+1+r.Intn(6)-1
		if r.Intn(10) == 0 {
			ds = hostile[r.Intn(len(hostile))]
		}
		dn, sn := (h+r.Intn(3)-1)*ds+w+r.Intn(4), (h+1+r.Intn(3)-1)*ss+w+1+r.Intn(4)
		// lengths are for allocation only: a hostile stride keeps a small plane
		if dn < 1 || dn > 2000 {
			dn = 16 + r.Intn(40)
		}
		if sn < 1 || sn > 2000 {
			sn = 16 + r.Intn(40)
		}
		dOff, sOff := r.Intn(4), r.Intn(4)
		if r.Intn(10) == 0 {
			sOff = hostile[r.Intn(len(hostile))]
		}
		xf, yf := r.Intn(8), r.Intn(8)
		if r.Intn(12) == 0 {
			xf = 8 + r.Intn(3)
		}
		cb, cr := chromaPlane(r, dn), chromaPlane(r, dn)
		sb, sr := chromaPlane(r, sn), chromaPlane(r, sn)
		switch r.Intn(6) {
		case 0:
			cr = cb // the planes share memory
		case 1:
			sb = cb[:min(len(cb), sn)] // a source on a destination
		}
		for _, avg := range []bool{false, true} {
			// pair
			a0, a1 := append([]byte(nil), cb...), append([]byte(nil), cr...)
			b0, b1 := append([]byte(nil), cb...), append([]byte(nil), cr...)
			if &cr[0] == &cb[0] {
				b1 = b0
				a1 = a0
			}
			var err error
			plain := func() {
				if avg {
					AvgChromaPair(a0, a1, dOff, ds, sb, sr, sOff, ss, w, h, xf, yf)
				} else {
					PutChromaPair(a0, a1, dOff, ds, sb, sr, sOff, ss, w, h, xf, yf)
				}
			}
			panicked := refused(plain)
			if avg {
				err = TryAvgChromaPair(b0, b1, dOff, ds, sb, sr, sOff, ss, w, h, xf, yf)
			} else {
				err = TryPutChromaPair(b0, b1, dOff, ds, sb, sr, sOff, ss, w, h, xf, yf)
			}
			if panicked != (err != nil) {
				t.Fatalf("pair avg=%v %dx%d: panicked=%v, error=%v", avg, w, h, panicked, err)
			}
			if err != nil {
				nRefused++
				if !bytes.Equal(b0, cb) && &cr[0] != &cb[0] {
					t.Fatal("a refused TryChromaPair wrote")
				}
			} else {
				nAccepted++
				if !bytes.Equal(a0, b0) || !bytes.Equal(a1, b1) {
					t.Fatalf("pair avg=%v %dx%d: the forms wrote different bytes", avg, w, h)
				}
			}

			// single plane
			c0, c1 := append([]byte(nil), cb...), append([]byte(nil), cb...)
			panicked = refused(func() {
				if avg {
					AvgChroma(c0, dOff, ds, sb, sOff, ss, w, h, xf, yf)
				} else {
					PutChroma(c0, dOff, ds, sb, sOff, ss, w, h, xf, yf)
				}
			})
			if avg {
				err = TryAvgChroma(c1, dOff, ds, sb, sOff, ss, w, h, xf, yf)
			} else {
				err = TryPutChroma(c1, dOff, ds, sb, sOff, ss, w, h, xf, yf)
			}
			if panicked != (err != nil) {
				t.Fatalf("single avg=%v %dx%d: panicked=%v, error=%v", avg, w, h, panicked, err)
			}
			if err != nil && !bytes.Equal(c1, cb) && &sb[0] != &cb[0] {
				t.Fatal("a refused TryChroma wrote")
			}
			if err == nil && !bytes.Equal(c0, c1) {
				t.Fatalf("single avg=%v %dx%d: the forms wrote different bytes", avg, w, h)
			}
		}
	}
	if nRefused < 1000 || nAccepted < 1000 {
		t.Fatalf("the test needs both outcomes: %d refused, %d accepted", nRefused, nAccepted)
	}
}
