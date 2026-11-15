package lf

import (
	"math"
	"testing"

	"github.com/go-videodsp/kernels/internal/boundstest"
)

// Strides and offsets that used to wrap the bounds proof (a stride of 1<<62
// times four rows is negative in int64) and the other hostile values. With
// small slices none can be a legal call: every one must be REFUSED.
var hostile = []int{math.MaxInt, math.MaxInt - 1, math.MaxInt/2 + 1, math.MaxInt/2 + 2, math.MaxInt/2 - 1, math.MaxInt / 4 * 3, -1, -2, math.MinInt, math.MinInt + 1}

func refused(f func()) (p bool) {
	defer func() { p = recover() != nil }()
	f()
	return
}

type entry struct {
	name string
	call func(pix, cr []byte, off, stride int)
}

var bS = [4]uint8{1, 2, 3, 0}

var entries = []entry{
	{"DeblockLumaV", func(p, _ []byte, o, s int) { DeblockLumaV(p, o, s, 30, 30, bS) }},
	{"DeblockLumaH", func(p, _ []byte, o, s int) { DeblockLumaH(p, o, s, 30, 30, bS) }},
	{"DeblockLumaIntraV", func(p, _ []byte, o, s int) { DeblockLumaIntraV(p, o, s, 30, 30) }},
	{"DeblockLumaIntraH", func(p, _ []byte, o, s int) { DeblockLumaIntraH(p, o, s, 30, 30) }},
	{"DeblockChromaV", func(p, _ []byte, o, s int) { DeblockChromaV(p, o, s, 30, 30, bS) }},
	{"DeblockChromaH", func(p, _ []byte, o, s int) { DeblockChromaH(p, o, s, 30, 30, bS) }},
	{"DeblockChromaIntraV", func(p, _ []byte, o, s int) { DeblockChromaIntraV(p, o, s, 30, 30) }},
	{"DeblockChromaIntraH", func(p, _ []byte, o, s int) { DeblockChromaIntraH(p, o, s, 30, 30) }},
	{"DeblockChromaPairV", func(p, c []byte, o, s int) { DeblockChromaPairV(p, c, o, s, 30, 30, 30, 30, bS) }},
	{"DeblockChromaPairH", func(p, c []byte, o, s int) { DeblockChromaPairH(p, c, o, s, 30, 30, 30, 30, bS) }},
	{"DeblockChromaPairIntraV", func(p, c []byte, o, s int) { DeblockChromaPairIntraV(p, c, o, s, 30, 30, 30, 30) }},
	{"DeblockChromaPairIntraH", func(p, c []byte, o, s int) { DeblockChromaPairIntraH(p, c, o, s, 30, 30, 30, 30) }},
}

func TestHostileStridesAndOffsetsAreRefused(t *testing.T) {
	for _, e := range entries {
		for _, v := range hostile {
			if !refused(func() { e.call(make([]byte, 256), make([]byte, 256), 64, v) }) {
				t.Errorf("%s: stride %d was accepted", e.name, v)
			}
			if !refused(func() { e.call(make([]byte, 256), make([]byte, 256), v, 32) }) {
				t.Errorf("%s: offset %d was accepted", e.name, v)
			}
		}
	}
}

// FuzzBoundsRefusal throws unconstrained integers at every entry point and
// compares the outcome with the exact-integer oracle: a call that returns on a
// footprint the oracle rejects is the defect.
func FuzzBoundsRefusal(f *testing.F) {
	f.Add(256, 64, 32)
	f.Add(256, 64, math.MaxInt/2+1)
	f.Add(256, math.MaxInt, 32)
	f.Fuzz(func(t *testing.T, n, off, stride int) {
		if n < 0 || n > 4096 {
			return
		}
		for i, e := range entries {
			var fits bool
			switch {
			case i < 4: // luma: 16 lines, columns -4..3 (V) or rows -4..3 (H)
				vertical := i%2 == 0
				if vertical {
					fits = boundstest.Fits(n, off, stride, 0, 15, 4, 3)
				} else {
					fits = boundstest.Fits(n, off, stride, 4, 3, 0, 15)
				}
				fits = fits && stride >= 16
			default: // chroma and its pair: 8 lines, columns/rows -2..1
				vertical := i%2 == 0
				if vertical {
					fits = boundstest.Fits(n, off, stride, 0, 7, 2, 1)
				} else {
					fits = boundstest.Fits(n, off, stride, 2, 1, 0, 7)
				}
				fits = fits && stride >= 8
			}
			if accepted := !refused(func() { e.call(make([]byte, n), make([]byte, n), off, stride) }); accepted && !fits {
				t.Fatalf("%s accepted a footprint the exact oracle rejects: n=%d off=%d stride=%d", e.name, n, off, stride)
			}
		}
	})
}
