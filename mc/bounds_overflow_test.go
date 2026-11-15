package mc

import (
	"math"
	"testing"

	"github.com/go-videodsp/kernels/internal/boundstest"
)

// Strides and offsets that used to wrap the bounds proof (1<<62 times four rows
// is negative in int64) and the other hostile values. With 64-byte slices none
// of them can be a legal call, so every one must be REFUSED, by assertion.
var hostile = []int{math.MaxInt, math.MaxInt - 1, math.MaxInt/2 + 1, math.MaxInt/2 + 2, math.MaxInt/2 - 1, math.MaxInt / 4 * 3, -1, -2, math.MinInt, math.MinInt + 1}

func refused(f func()) (p bool) {
	defer func() { p = recover() != nil }()
	f()
	return
}

func TestHostileStridesAndOffsetsAreRefused(t *testing.T) {
	buf := func() []byte { return make([]byte, 64) }
	for _, v := range hostile {
		for _, sz := range sizes {
			w, h := sz[0], sz[1]
			cases := map[string]func(){
				"PutLuma src stride":   func() { PutLuma(buf(), 0, 16, buf(), 8, v, w, h, 0, 0) },
				"PutLuma src stride H": func() { PutLuma(buf(), 0, 16, buf(), 8, v, w, h, 2, 2) },
				"PutLuma dst stride":   func() { PutLuma(buf(), 0, v, buf(), 8, 16, w, h, 0, 0) },
				"PutLuma src offset":   func() { PutLuma(buf(), 0, 16, buf(), v, 16, w, h, 2, 2) },
				"PutLuma dst offset":   func() { PutLuma(buf(), v, 16, buf(), 8, 16, w, h, 0, 0) },
				"AvgLuma src stride":   func() { AvgLuma(buf(), 0, 16, buf(), 8, v, w, h, 1, 3) },
				"AvgLuma dst stride":   func() { AvgLuma(buf(), 0, v, buf(), 8, 16, w, h, 1, 3) },
				"AvgLuma offsets":      func() { AvgLuma(buf(), v, 16, buf(), v, 16, w, h, 1, 3) },
				"WeightUni stride":     func() { WeightUni(buf(), 0, v, w, h, 3, 1, 2) },
				"WeightUni offset":     func() { WeightUni(buf(), v, 8, w, h, 3, 1, 2) },
				"WeightBi dst stride":  func() { WeightBi(buf(), 0, v, buf(), 0, 16, w, h, 3, 5, 1, 1, 3) },
				"WeightBi src stride":  func() { WeightBi(buf(), 0, 16, buf(), 0, v, w, h, 3, 5, 1, 1, 3) },
				"WeightBi offsets":     func() { WeightBi(buf(), v, 16, buf(), v, 16, w, h, 3, 5, 1, 1, 3) },
			}
			for name, f := range cases {
				if !refused(f) {
					t.Errorf("%s: %dx%d value %d was accepted", name, w, h, v)
				}
			}
		}
	}
}

// FuzzBoundsRefusal throws unconstrained integers at every entry point of the
// package and compares the outcome with the exact-integer oracle: a call panics
// if and only if its footprint does not fit (or a non-bounds rule refuses it).
// A call that returns on a footprint the oracle rejects is the defect.
func FuzzBoundsRefusal(f *testing.F) {
	f.Add(64, 64, 0, 16, 8, 16, uint8(0), uint8(0))
	f.Add(64, 64, 0, math.MaxInt/2+1, 8, 16, uint8(1), uint8(3))
	f.Add(64, 64, math.MaxInt, 16, math.MaxInt, 16, uint8(6), uint8(2))
	f.Fuzz(func(t *testing.T, dn, sn, dOff, dStride, sOff, sStride int, sizeIdx, pos uint8) {
		if dn < 0 || sn < 0 || dn > 4096 || sn > 4096 {
			return // allocation size only; every other value is unconstrained
		}
		dst, src := make([]byte, dn), make([]byte, sn)
		sz := sizes[int(sizeIdx)%len(sizes)]
		w, h := sz[0], sz[1]
		fx, fy := int(pos)&3, int(pos>>2)&3
		l, r, u, d := margins(fx, fy)

		fits := boundstest.Fits(dn, dOff, dStride, 0, h-1, 0, w-1) && dStride >= w &&
			boundstest.Fits(sn, sOff, sStride, u, h-1+d, l, w-1+r) && sStride > 0
		if got := !refused(func() { PutLuma(dst, dOff, dStride, src, sOff, sStride, w, h, fx, fy) }); got && !fits {
			t.Fatalf("PutLuma accepted a footprint the exact oracle rejects: dn=%d sn=%d dOff=%d dStride=%d sOff=%d sStride=%d %dx%d", dn, sn, dOff, dStride, sOff, sStride, w, h)
		}
		if got := !refused(func() { AvgLuma(dst, dOff, dStride, src, sOff, sStride, w, h, fx, fy) }); got && !fits {
			t.Fatal("AvgLuma accepted a footprint the exact oracle rejects")
		}
		uniFits := boundstest.Fits(dn, dOff, dStride, 0, h-1, 0, w-1) && dStride >= w
		if got := !refused(func() { WeightUni(dst, dOff, dStride, w, h, 3, 1, 2) }); got && !uniFits {
			t.Fatal("WeightUni accepted a block the exact oracle rejects")
		}
		biFits := uniFits && boundstest.Fits(sn, sOff, sStride, 0, h-1, 0, w-1) && sStride >= w
		if got := !refused(func() { WeightBi(dst, dOff, dStride, src, sOff, sStride, w, h, 3, 5, 1, 1, 3) }); got && !biFits {
			t.Fatal("WeightBi accepted a block the exact oracle rejects")
		}
	})
}
