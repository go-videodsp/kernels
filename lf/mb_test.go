package lf

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/go-videodsp/kernels/internal/boundstest"
)

// composeMB is DeblockMB written as the per-edge public calls, in the order of
// clause 8.7: the oracle of the result and of the bounds proof.
func composeMB(y []byte, yOff, yStride int, cb, cr []byte, cOff, cStride int, e *MBEdges) {
	for d := 0; d < 2; d++ {
		vertical := d == 0
		for k := 0; k < 4; k++ {
			bs := e.BS[d][k]
			if bs == [4]uint8{} {
				continue
			}
			th := e.Luma[d][min(k, 1)]
			off := yOff + 4*k
			if !vertical {
				off = yOff + 4*k*yStride
			}
			ia, ib := int(th.IndexA), int(th.IndexB)
			switch {
			case bs == [4]uint8{4, 4, 4, 4} && vertical:
				DeblockLumaIntraV(y, off, yStride, ia, ib)
			case bs == [4]uint8{4, 4, 4, 4}:
				DeblockLumaIntraH(y, off, yStride, ia, ib)
			case vertical:
				DeblockLumaV(y, off, yStride, ia, ib, bs)
			default:
				DeblockLumaH(y, off, yStride, ia, ib, bs)
			}
		}
		for k := 0; k < 4; k += 2 {
			bs := e.BS[d][k]
			if bs == [4]uint8{} {
				continue
			}
			a, b := e.Chroma[0][d][min(k, 1)], e.Chroma[1][d][min(k, 1)]
			off := cOff + 2*k
			if !vertical {
				off = cOff + 2*k*cStride
			}
			ia0, ib0, ia1, ib1 := int(a.IndexA), int(a.IndexB), int(b.IndexA), int(b.IndexB)
			switch {
			case bs == [4]uint8{4, 4, 4, 4} && vertical:
				DeblockChromaPairIntraV(cb, cr, off, cStride, ia0, ib0, ia1, ib1)
			case bs == [4]uint8{4, 4, 4, 4}:
				DeblockChromaPairIntraH(cb, cr, off, cStride, ia0, ib0, ia1, ib1)
			case vertical:
				DeblockChromaPairV(cb, cr, off, cStride, ia0, ib0, ia1, ib1, bs)
			default:
				DeblockChromaPairH(cb, cr, off, cStride, ia0, ib0, ia1, ib1, bs)
			}
		}
	}
}

func randomEdges(r *rand.Rand) *MBEdges {
	e := &MBEdges{}
	for d := 0; d < 2; d++ {
		for k := 0; k < 4; k++ {
			switch r.Intn(5) {
			case 0: // not filtered
			case 1:
				if k == 0 {
					e.BS[d][k] = [4]uint8{4, 4, 4, 4}
					continue
				}
				fallthrough
			default:
				for g := range e.BS[d][k] {
					e.BS[d][k][g] = uint8(r.Intn(4))
				}
			}
		}
		for j := 0; j < 2; j++ {
			e.Luma[d][j] = Thresholds{uint8(18 + r.Intn(34)), uint8(18 + r.Intn(34))}
			for p := 0; p < 2; p++ {
				e.Chroma[p][d][j] = Thresholds{uint8(18 + r.Intn(34)), uint8(18 + r.Intn(34))}
			}
		}
	}
	return e
}

// smooth content so that the filters' strict tests pass often: a flat base
// with small steps, and some strong steps.
func smoothPlane(r *rand.Rand, n int) []byte {
	b := make([]byte, n)
	v := 120
	for i := range b {
		if r.Intn(8) == 0 {
			v += r.Intn(21) - 10
		}
		if v < 0 {
			v = 0
		}
		if v > 255 {
			v = 255
		}
		b[i] = byte(v + r.Intn(3))
	}
	return b
}

// DeblockMB equals the per-edge calls in the order of 8.7 on every mix of
// edges, strengths and thresholds, at several offsets and strides, through the
// implementation this build selects.
func TestDeblockMBMatchesPerEdgeCalls(t *testing.T) {
	r := rand.New(rand.NewSource(33))
	filtered := 0
	for n := 0; n < 3000; n++ {
		yStride, cStride := 16+r.Intn(24), 8+r.Intn(12)
		yOff := 4*yStride + 4 + r.Intn(5)
		cOff := 2*cStride + 2 + r.Intn(5)
		y := smoothPlane(r, 24*yStride+40)
		cb, cr := smoothPlane(r, 14*cStride+30), smoothPlane(r, 14*cStride+30)
		e := randomEdges(r)
		y2, cb2, cr2 := append([]byte(nil), y...), append([]byte(nil), cb...), append([]byte(nil), cr...)
		DeblockMB(y, yOff, yStride, cb, cr, cOff, cStride, e)
		composeMB(y2, yOff, yStride, cb2, cr2, cOff, cStride, e)
		if !bytes.Equal(y, y2) || !bytes.Equal(cb, cb2) || !bytes.Equal(cr, cr2) {
			t.Fatalf("case %d: DeblockMB differs from the per-edge calls (strengths %v)", n, e.BS)
		}
		filtered++
	}
	if filtered == 0 {
		t.Fatal("nothing ran")
	}
}

// The filters must have changed something in these tests, or the comparison
// above could pass on two no-ops.
func TestDeblockMBChangesSamples(t *testing.T) {
	r := rand.New(rand.NewSource(8))
	changed := 0
	for n := 0; n < 200; n++ {
		y := smoothPlane(r, 24*32+40)
		cb, cr := smoothPlane(r, 14*16+30), smoothPlane(r, 14*16+30)
		y0 := append([]byte(nil), y...)
		DeblockMB(y, 4*32+4, 32, cb, cr, 2*16+2, 16, randomEdges(r))
		if !bytes.Equal(y, y0) {
			changed++
		}
	}
	if changed < 100 {
		t.Fatalf("only %d of 200 macroblocks changed a luma sample", changed)
	}
}

func mbPanics(f func()) (p bool) {
	defer func() { p = recover() != nil }()
	f()
	return
}

// The hoisted proof is at least as strong as the per-edge proofs it replaces:
// over random geometries, including offsets at the edges of the slices and
// strides that are too small, whenever the per-edge calls refuse, DeblockMB
// refuses, and it refuses before writing anything. Its window is exactly the
// one the exact-integer oracle computes from the filtered edges.
func TestDeblockMBProofIsAtLeastAsStrongAsPerEdge(t *testing.T) {
	r := rand.New(rand.NewSource(77))
	refusedByComposition, acceptedByBoth := 0, 0
	for n := 0; n < 20000; n++ {
		// geometries around the edge of what fits: about a macroblock window with
		// its borders, with strides, lengths and offsets nudged either way
		yStride, cStride := 16+r.Intn(24), 8+r.Intn(12)
		if r.Intn(10) == 0 {
			yStride, cStride = 8+r.Intn(30), 4+r.Intn(14)
		}
		yLen, cLen := (19+r.Intn(3))*yStride+r.Intn(40), (9+r.Intn(3))*cStride+r.Intn(20)
		yOff, cOff := 4*yStride+4+r.Intn(9)-4, 2*cStride+2+r.Intn(9)-4
		if r.Intn(8) == 0 {
			yOff = hostile[r.Intn(len(hostile))]
		}
		if r.Intn(8) == 0 {
			yStride = hostile[r.Intn(len(hostile))]
		}
		if r.Intn(8) == 0 {
			cOff = hostile[r.Intn(len(hostile))]
		}
		e := randomEdges(r)
		y := smoothPlane(r, yLen)
		cb, cr := smoothPlane(r, cLen), smoothPlane(r, cLen)
		y1, cb1, cr1 := append([]byte(nil), y...), append([]byte(nil), cb...), append([]byte(nil), cr...)
		y2, cb2, cr2 := append([]byte(nil), y...), append([]byte(nil), cb...), append([]byte(nil), cr...)
		compRefused := mbPanics(func() { composeMB(y1, yOff, yStride, cb1, cr1, cOff, cStride, e) })
		mbRefused := mbPanics(func() { DeblockMB(y2, yOff, yStride, cb2, cr2, cOff, cStride, e) })
		if compRefused && !mbRefused {
			t.Fatalf("case %d: the per-edge calls refuse and DeblockMB does not (yOff %d yStride %d cOff %d cStride %d)", n, yOff, yStride, cOff, cStride)
		}
		if mbRefused {
			if !bytes.Equal(y2, y) || !bytes.Equal(cb2, cb) || !bytes.Equal(cr2, cr) {
				t.Fatalf("case %d: DeblockMB refused after writing", n)
			}
		}
		// the exact oracle for the luma window and the chroma windows
		lumaActive, chromaActive := false, false
		top, left, ctop, cleft := 0, 0, 0, 0
		for d := 0; d < 2; d++ {
			for k := 0; k < 4; k++ {
				if e.BS[d][k] != [4]uint8{} {
					lumaActive = true
					if k%2 == 0 {
						chromaActive = true
					}
				}
			}
		}
		if e.BS[1][0] != [4]uint8{} {
			top, ctop = 4, 2
		}
		if e.BS[0][0] != [4]uint8{} {
			left, cleft = 4, 2
		}
		fits := true
		if lumaActive {
			fits = fits && yStride >= 16 && boundstest.Fits(len(y), yOff, yStride, top, 15, left, 15)
		}
		if chromaActive {
			fits = fits && cStride >= 8 && boundstest.Fits(len(cb), cOff, cStride, ctop, 7, cleft, 7) &&
				boundstest.Fits(len(cr), cOff, cStride, ctop, 7, cleft, 7)
		}
		// the planes here are distinct slices, so sharing cannot be the reason
		if mbRefused && fits {
			t.Fatalf("case %d: DeblockMB refused a window the oracle accepts", n)
		}
		if !mbRefused && !fits {
			t.Fatalf("case %d: DeblockMB accepted a window the oracle rejects", n)
		}
		if compRefused {
			refusedByComposition++
		} else {
			acceptedByBoth++
		}
	}
	if refusedByComposition < 2000 || acceptedByBoth < 2000 {
		t.Fatalf("the test needs both outcomes: %d refused, %d accepted", refusedByComposition, acceptedByBoth)
	}
}

// Where the region proof must refuse although a single edge fits: only the top
// edge is filtered, the slice holds that edge's own footprint (4 rows above, 3
// below) and nothing more, and the macroblock's window is larger.
func TestDeblockMBRegionProofRefuses(t *testing.T) {
	const stride = 16
	off := 4*stride + 0
	var e MBEdges
	e.BS[1][0] = [4]uint8{1, 1, 1, 1}
	e.Luma[1][0] = Thresholds{30, 30}
	short := make([]byte, off+3*stride+16) // rows -4..3 of the edge, and no more
	one := append([]byte(nil), short...)
	if mbPanics(func() { DeblockLumaH(one, off, stride, 30, 30, [4]uint8{1, 1, 1, 1}) }) {
		t.Fatal("the single edge must fit this slice")
	}
	before := append([]byte(nil), short...)
	if !mbPanics(func() { DeblockMB(short, off, stride, nil, nil, 0, 8, &e) }) {
		t.Fatal("the macroblock window does not fit and must be refused")
	}
	if !bytes.Equal(short, before) {
		t.Fatal("a refused call wrote")
	}
}

// Other refusals: the Cb and Cr windows sharing memory, strengths that
// DeblockMB does not take, indexes and strides out of range.
func TestDeblockMBRefusals(t *testing.T) {
	y := make([]byte, 24*32)
	c := make([]byte, 4096)
	mk := func(f func(*MBEdges)) *MBEdges {
		e := &MBEdges{}
		e.BS[0][1] = [4]uint8{1, 2, 3, 1}
		e.BS[1][0] = [4]uint8{4, 4, 4, 4}
		for d := 0; d < 2; d++ {
			e.Luma[d] = [2]Thresholds{{30, 30}, {30, 30}}
			for p := 0; p < 2; p++ {
				e.Chroma[p][d] = [2]Thresholds{{30, 30}, {30, 30}}
			}
		}
		f(e)
		return e
	}
	ok := mk(func(*MBEdges) {})
	if mbPanics(func() { DeblockMB(y, 4*32+4, 32, c[:2048], c[2048:], 2*32+2, 32, ok) }) {
		t.Fatal("the base case was refused")
	}
	for name, e := range map[string]*MBEdges{
		"strength 5":         mk(func(e *MBEdges) { e.BS[0][1][0] = 5 }),
		"4 on an inner edge": mk(func(e *MBEdges) { e.BS[0][2] = [4]uint8{4, 4, 4, 4} }),
		"4 beside others":    mk(func(e *MBEdges) { e.BS[1][0] = [4]uint8{4, 4, 3, 4} }),
		"4 on edge 0, not 4": mk(func(e *MBEdges) { e.BS[0][0] = [4]uint8{4, 0, 0, 0} }),
		"luma index":         mk(func(e *MBEdges) { e.Luma[0][1].IndexA = 52 }),
		"luma index B":       mk(func(e *MBEdges) { e.Luma[1][0].IndexB = 99 }),
		"chroma index":       mk(func(e *MBEdges) { e.Chroma[1][0][0].IndexA = 52 }),
		"chroma index inner": mk(func(e *MBEdges) { e.BS[0][0] = [4]uint8{1, 1, 1, 1}; e.Chroma[0][0][1].IndexB = 60 }),
	} {
		if !mbPanics(func() { DeblockMB(y, 4*32+4, 32, c[:2048], c[2048:], 2*32+2, 32, e) }) {
			t.Errorf("%s was accepted", name)
		}
	}
	if !mbPanics(func() { DeblockMB(y, 4*32+4, 15, c[:2048], c[2048:], 2*32+2, 32, ok) }) {
		t.Error("a luma stride below 16 was accepted")
	}
	if !mbPanics(func() { DeblockMB(y, 4*32+4, 32, c[:2048], c[2048:], 2*32+2, 7, ok) }) {
		t.Error("a chroma stride below 8 was accepted")
	}
	if !mbPanics(func() { DeblockMB(y, 4*32+4, 32, c, c, 2*32+2, 32, ok) }) {
		t.Error("Cb and Cr sharing one slice were accepted")
	}
	// an unfiltered plane is not looked at: only luma edges, no chroma edge 0 or 2
	only := &MBEdges{}
	only.BS[0][1] = [4]uint8{1, 1, 1, 1}
	only.Luma[0][1] = Thresholds{30, 30}
	if mbPanics(func() { DeblockMB(y, 4*32+4, 32, nil, nil, 0, 0, only) }) {
		t.Error("inner luma edges alone must not need chroma planes")
	}
	for _, v := range hostile {
		for name, f := range map[string]func(){
			"luma stride":   func() { DeblockMB(y, 4*32+4, v, c[:2048], c[2048:], 2*32+2, 32, ok) },
			"luma offset":   func() { DeblockMB(y, v, 32, c[:2048], c[2048:], 2*32+2, 32, ok) },
			"chroma stride": func() { DeblockMB(y, 4*32+4, 32, c[:2048], c[2048:], 2*32+2, v, ok) },
			"chroma offset": func() { DeblockMB(y, 4*32+4, 32, c[:2048], c[2048:], v, 32, ok) },
		} {
			if !mbPanics(f) {
				t.Errorf("%s value %d was accepted", name, v)
			}
		}
	}
}

func BenchmarkDeblockMB(b *testing.B) {
	r := rand.New(rand.NewSource(2))
	const yStride, cStride = 64, 32
	y, cb, cr := smoothPlane(r, 24*yStride), smoothPlane(r, 14*cStride), smoothPlane(r, 14*cStride)
	e := randomEdges(r)
	for d := 0; d < 2; d++ {
		for k := 1; k < 4; k++ {
			e.BS[d][k] = [4]uint8{1, 1, 2, 1}
		}
		e.BS[d][0] = [4]uint8{2, 2, 2, 2}
	}
	b.Run("per-edge", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			composeMB(y, 4*yStride+4, yStride, cb, cr, 2*cStride+2, cStride, e)
		}
	})
	b.Run("mb", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			DeblockMB(y, 4*yStride+4, yStride, cb, cr, 2*cStride+2, cStride, e)
		}
	})
}
