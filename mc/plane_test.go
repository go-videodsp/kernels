package mc

import (
	"bytes"
	"math"
	"testing"
)

// The contract of Plane and ChromaPair is containment plus equality: a method
// accepts exactly the blocks the plain function accepts (when the two planes
// are different memory), writes exactly what it writes and never touches a byte
// outside the destination slice. The tests check all three over every offset
// around a plane, edge blocks and the extreme motion-vector range included
// (the offset of a block displaced by the largest vector, before and after the
// picture, with and without padding), in buffers with guard bytes on both sides.

const planePadN = 96

func planeDidPanic(f func()) (p bool) {
	defer func() {
		if recover() != nil {
			p = true
		}
	}()
	f()
	return
}

// planeMustNotPanic fails the test, by an assertion, when a Plane method panics:
// a refusal is an error value, and a panic is the dispatch running on a block
// that was not refused.
func planeMustNotPanic(t *testing.T, call any, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("%+v: the Plane method panicked instead of returning an error: %v", call, r)
		}
	}()
	f()
}

func planeBuf(seed int64, n int) (back, mid []byte) {
	// xorshift, not math/rand: the sweeps build tens of thousands of buffers
	// and math/rand's source costs more to seed than the call it feeds.
	x := uint64(seed)*0x9E3779B97F4A7C15 + 0x2545F4914F6CDD1D
	back = make([]byte, n+2*planePadN)
	for i := range back {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		back[i] = byte(x >> 24)
	}
	return back, back[planePadN : planePadN+n : planePadN+n]
}

// sweepOffsets lists every offset from well before the plane to well after it,
// the offsets of the largest motion vectors (a vector of +-2048 quarter samples
// is 512 rows of the plane away) and the extremes of int.
func sweepOffsets(n, stride int) []int {
	var offs []int
	for off := -(8*stride + 8); off <= n+8*stride+8; off++ {
		offs = append(offs, off)
	}
	for _, mv := range []int{512, 2048, 1 << 20} {
		offs = append(offs, -mv*stride, mv*stride, -mv*stride-mv, n+mv*stride+mv)
	}
	return append(offs, math.MinInt, math.MinInt+1, math.MaxInt, math.MaxInt-1, math.MinInt/2, math.MaxInt/2+1)
}

type lumaCall struct {
	dstOff, srcOff, w, h, xFrac, yFrac int
	avg                                bool
}

// runLuma runs the call through Plane and through the plain function on
// identical buffers and reports whether it was accepted.
func runLuma(t *testing.T, c lumaCall, dn, sn, dstStride, srcStride int, seed int64) bool {
	t.Helper()
	dBackA, dA := planeBuf(seed, dn)
	dBackB, dB := planeBuf(seed, dn)
	sBackA, sA := planeBuf(seed+1, sn)
	sBackB, sB := planeBuf(seed+1, sn)
	dp, err := NewPlane(dA, dstStride)
	if err != nil {
		t.Fatal(err)
	}
	sp, err := NewPlane(sA, srcStride)
	if err != nil {
		t.Fatal(err)
	}
	var gotErr error
	planeMustNotPanic(t, c, func() {
		if c.avg {
			gotErr = dp.AvgLuma(c.dstOff, &sp, c.srcOff, c.w, c.h, c.xFrac, c.yFrac)
		} else {
			gotErr = dp.PutLuma(c.dstOff, &sp, c.srcOff, c.w, c.h, c.xFrac, c.yFrac)
		}
	})
	wantPanic := planeDidPanic(func() {
		if c.avg {
			AvgLuma(dB, c.dstOff, dstStride, sB, c.srcOff, srcStride, c.w, c.h, c.xFrac, c.yFrac)
		} else {
			PutLuma(dB, c.dstOff, dstStride, sB, c.srcOff, srcStride, c.w, c.h, c.xFrac, c.yFrac)
		}
	})
	if (gotErr != nil) != wantPanic {
		t.Fatalf("%+v dn=%d sn=%d: Plane error %v, the plain function panicked %v", c, dn, sn, gotErr, wantPanic)
	}
	if !bytes.Equal(dBackA, dBackB) || !bytes.Equal(sBackA, sBackB) {
		t.Fatalf("%+v dn=%d sn=%d: bytes differ from the plain function (or a guard was touched)", c, dn, sn)
	}
	return gotErr == nil
}

var lumaSizes = [][2]int{{16, 16}, {16, 8}, {8, 16}, {8, 8}, {8, 4}, {4, 8}, {4, 4}}

func TestPlaneLumaEqualsPlainEverywhere(t *testing.T) {
	if !inPlaneChild(t) {
		return
	}
	const dstStride, srcStride, dn, sn = 16, 24, 16 * 20, 24 * 30
	for _, avg := range []bool{false, true} {
		for _, sz := range lumaSizes {
			w, h := sz[0], sz[1]
			for xf := 0; xf < 4; xf++ {
				for yf := 0; yf < 4; yf++ {
					acceptedSrc, refusedSrc, acceptedDst, refusedDst := 0, 0, 0, 0
					for _, off := range sweepOffsets(sn, srcStride) {
						if runLuma(t, lumaCall{dstOff: 16*2 + 1, srcOff: off, w: w, h: h, xFrac: xf, yFrac: yf, avg: avg}, dn, sn, dstStride, srcStride, int64(off)) {
							acceptedSrc++
						} else {
							refusedSrc++
						}
					}
					for _, off := range sweepOffsets(dn, dstStride) {
						if runLuma(t, lumaCall{dstOff: off, srcOff: 24*2 + 2, w: w, h: h, xFrac: xf, yFrac: yf, avg: avg}, dn, sn, dstStride, srcStride, int64(off)) {
							acceptedDst++
						} else {
							refusedDst++
						}
					}
					if acceptedSrc == 0 || refusedSrc == 0 || acceptedDst == 0 || refusedDst == 0 {
						t.Fatalf("avg=%v %dx%d frac (%d,%d): the sweep did not cross both limits: %d %d %d %d", avg, w, h, xf, yf, acceptedSrc, refusedSrc, acceptedDst, refusedDst)
					}
				}
			}
		}
	}
}

func TestPlaneLumaRefusesBadSizesAndFractions(t *testing.T) {
	if !inPlaneChild(t) {
		return
	}
	const dstStride, srcStride, dn, sn = 16, 24, 16 * 20, 24 * 20
	for _, sz := range [][2]int{{0, 0}, {16, 4}, {4, 16}, {2, 2}, {12, 8}, {8, 12}, {32, 32}, {-8, 8}, {8, -8}} {
		if runLuma(t, lumaCall{dstOff: 40, srcOff: 24*6 + 6, w: sz[0], h: sz[1]}, dn, sn, dstStride, srcStride, 1) {
			t.Errorf("size %dx%d accepted", sz[0], sz[1])
		}
	}
	for _, f := range [][2]int{{-1, 0}, {0, -1}, {4, 0}, {0, 4}, {math.MinInt, 0}, {0, math.MaxInt}} {
		if runLuma(t, lumaCall{dstOff: 40, srcOff: 24*6 + 6, w: 8, h: 8, xFrac: f[0], yFrac: f[1]}, dn, sn, dstStride, srcStride, 1) {
			t.Errorf("fractions %v accepted", f)
		}
	}
}

type chromaCall struct {
	dstOff, srcOff, w, h, xFrac, yFrac int
	avg                                bool
}

func runChroma(t *testing.T, c chromaCall, dn, sn, dstStride, srcStride int, seed int64) bool {
	t.Helper()
	dBackA, dA := planeBuf(seed, dn)
	dBackB, dB := planeBuf(seed, dn)
	eBackA, eA := planeBuf(seed+3, dn)
	eBackB, eB := planeBuf(seed+3, dn)
	sBackA, sA := planeBuf(seed+1, sn)
	sBackB, sB := planeBuf(seed+1, sn)
	tBackA, tA := planeBuf(seed+2, sn)
	tBackB, tB := planeBuf(seed+2, sn)
	dp, err := NewChromaPair(dA, eA, dstStride)
	if err != nil {
		t.Fatal(err)
	}
	sp, err := NewChromaPair(sA, tA, srcStride)
	if err != nil {
		t.Fatal(err)
	}
	var gotErr, wantErr error
	planeMustNotPanic(t, c, func() {
		if c.avg {
			gotErr = dp.AvgChroma(c.dstOff, &sp, c.srcOff, c.w, c.h, c.xFrac, c.yFrac)
		} else {
			gotErr = dp.PutChroma(c.dstOff, &sp, c.srcOff, c.w, c.h, c.xFrac, c.yFrac)
		}
	})
	if c.avg {
		wantErr = TryAvgChromaPair(dB, eB, c.dstOff, dstStride, sB, tB, c.srcOff, srcStride, c.w, c.h, c.xFrac, c.yFrac)
	} else {
		wantErr = TryPutChromaPair(dB, eB, c.dstOff, dstStride, sB, tB, c.srcOff, srcStride, c.w, c.h, c.xFrac, c.yFrac)
	}
	if (gotErr != nil) != (wantErr != nil) {
		t.Fatalf("%+v dn=%d sn=%d: ChromaPair error %v, the plain function %v", c, dn, sn, gotErr, wantErr)
	}
	for _, p := range [][2][]byte{{dBackA, dBackB}, {eBackA, eBackB}, {sBackA, sBackB}, {tBackA, tBackB}} {
		if !bytes.Equal(p[0], p[1]) {
			t.Fatalf("%+v dn=%d sn=%d: bytes differ from the plain function (or a guard was touched)", c, dn, sn)
		}
	}
	return gotErr == nil
}

func TestChromaPairEqualsPlainEverywhere(t *testing.T) {
	if !inPlaneChild(t) {
		return
	}
	const dstStride, srcStride, dn, sn = 16, 24, 16 * 14, 24 * 14
	for _, avg := range []bool{false, true} {
		for _, w := range []int{2, 4, 8} {
			for _, h := range []int{2, 4, 8} {
				for _, f := range [][2]int{{0, 0}, {3, 5}, {7, 7}, {0, 7}} {
					accSrc, refSrc, accDst, refDst := 0, 0, 0, 0
					for _, off := range sweepOffsets(sn, srcStride) {
						if runChroma(t, chromaCall{dstOff: 16 + 1, srcOff: off, w: w, h: h, xFrac: f[0], yFrac: f[1], avg: avg}, dn, sn, dstStride, srcStride, int64(off)) {
							accSrc++
						} else {
							refSrc++
						}
					}
					for _, off := range sweepOffsets(dn, dstStride) {
						if runChroma(t, chromaCall{dstOff: off, srcOff: 24*3 + 3, w: w, h: h, xFrac: f[0], yFrac: f[1], avg: avg}, dn, sn, dstStride, srcStride, int64(off)) {
							accDst++
						} else {
							refDst++
						}
					}
					if accSrc == 0 || refSrc == 0 || accDst == 0 || refDst == 0 {
						t.Fatalf("avg=%v %dx%d frac %v: the sweep did not cross both limits: %d %d %d %d", avg, w, h, f, accSrc, refSrc, accDst, refDst)
					}
				}
			}
		}
	}
}

func TestChromaPairRefusesBadSizesAndFractions(t *testing.T) {
	if !inPlaneChild(t) {
		return
	}
	const dstStride, srcStride, dn, sn = 16, 24, 16 * 14, 24 * 14
	for _, sz := range [][2]int{{0, 0}, {16, 8}, {8, 16}, {3, 4}, {4, 3}, {1, 1}, {-8, 8}, {8, -8}} {
		if runChroma(t, chromaCall{dstOff: 40, srcOff: 24*3 + 3, w: sz[0], h: sz[1]}, dn, sn, dstStride, srcStride, 1) {
			t.Errorf("size %dx%d accepted", sz[0], sz[1])
		}
	}
	for _, f := range [][2]int{{-1, 0}, {0, -1}, {8, 0}, {0, 8}, {math.MinInt, 0}, {0, math.MaxInt}} {
		if runChroma(t, chromaCall{dstOff: 40, srcOff: 24*3 + 3, w: 8, h: 8, xFrac: f[0], yFrac: f[1]}, dn, sn, dstStride, srcStride, 1) {
			t.Errorf("fractions %v accepted", f)
		}
	}
}

func TestPlanesThatShareMemoryAreRefused(t *testing.T) {
	if !inPlaneChild(t) {
		return
	}
	arr := make([]byte, 4096)
	const n = 1024
	cases := []struct {
		name     string
		dst, src []byte
		ok       bool
	}{
		{"adjacent, dst first", arr[:n], arr[n : 2*n], true},
		{"adjacent, src first", arr[n : 2*n], arr[:n], true},
		{"overlapping by one byte", arr[:n], arr[n-1 : 2*n-1], false},
		{"overlapping by one byte, src first", arr[n-1 : 2*n-1], arr[:n], false},
		{"the same plane", arr[:n], arr[:n], false},
		{"nested", arr[:n], arr[10 : n-10], false},
	}
	for _, c := range cases {
		dp, err := NewPlane(c.dst, 32)
		if err != nil {
			t.Fatal(err)
		}
		sp, err := NewPlane(c.src, 32)
		if err != nil {
			t.Fatal(err)
		}
		if got := dp.PutLuma(32*4, &sp, 32*8+8, 8, 8, 1, 1) == nil; got != c.ok {
			t.Errorf("PutLuma %s: accepted %v, want %v", c.name, got, c.ok)
		}
		if got := dp.AvgLuma(32*4, &sp, 32*8+8, 8, 8, 1, 1) == nil; got != c.ok {
			t.Errorf("AvgLuma %s: accepted %v, want %v", c.name, got, c.ok)
		}
	}
	// A chroma pair: the four planes must be four different memories.
	quarter := func(i int) []byte { return arr[i*n : (i+1)*n] }
	pair := func(a, b []byte) ChromaPair {
		p, err := NewChromaPair(a, b, 32)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	d, s := pair(quarter(0), quarter(1)), pair(quarter(2), quarter(3))
	if err := d.PutChroma(32*2, &s, 32*3+3, 4, 4, 3, 3); err != nil {
		t.Errorf("four disjoint planes: %v", err)
	}
	for _, c := range []struct {
		name string
		s    ChromaPair
	}{
		{"src Cb is dst Cb", pair(quarter(0), quarter(3))},
		{"src Cb is dst Cr", pair(quarter(1), quarter(3))},
		{"src Cr is dst Cb", pair(quarter(2), quarter(0))},
		{"src Cr is dst Cr", pair(quarter(2), quarter(1))},
	} {
		if d.PutChroma(32*2, &c.s, 32*3+3, 4, 4, 3, 3) == nil {
			t.Errorf("PutChroma: %s was accepted", c.name)
		}
		if d.AvgChroma(32*2, &c.s, 32*3+3, 4, 4, 3, 3) == nil {
			t.Errorf("AvgChroma: %s was accepted", c.name)
		}
	}
}

func TestZeroPlanesRefuseEverything(t *testing.T) {
	var z, y Plane
	var zc, yc ChromaPair
	for off := -3; off < 40; off++ {
		if z.PutLuma(off, &y, off, 4, 4, 0, 0) == nil || z.AvgLuma(off, &y, off, 4, 4, 0, 0) == nil {
			t.Errorf("the zero Plane accepted offset %d", off)
		}
		if zc.PutChroma(off, &yc, off, 2, 2, 0, 0) == nil || zc.AvgChroma(off, &yc, off, 2, 2, 0, 0) == nil {
			t.Errorf("the zero ChromaPair accepted offset %d", off)
		}
	}
}

func TestNewPlaneRefusals(t *testing.T) {
	pix := make([]byte, 100)
	for _, c := range []struct {
		name   string
		pix    []byte
		stride int
		max    int
		ok     bool
	}{
		{"stride 15", pix, 15, 100, false},
		{"stride 16", pix, 16, 100, true},
		{"stride 0", pix, 0, 100, false},
		{"stride -16", pix, -16, 100, false},
		{"stride equals the length", pix, 100, 100, true},
		{"stride beyond the length", pix, 101, 100, false},
		{"stride half of int", pix, math.MaxInt/2 + 1, 100, false},
		{"length at the cap", pix, 16, 100, true},
		{"length over the cap", pix, 16, 99, false},
		{"empty", nil, 16, 100, false},
	} {
		_, err := newPlane(c.pix, c.stride, c.max)
		if (err == nil) != c.ok {
			t.Errorf("%s: err = %v, want ok = %v", c.name, err, c.ok)
		}
	}
	if _, err := NewPlane(pix, 16); err != nil {
		t.Errorf("NewPlane(100 bytes, stride 16): %v", err)
	}
}

func TestNewChromaPairRefusals(t *testing.T) {
	arr := make([]byte, 1024)
	const n = 256
	if _, err := NewChromaPair(arr[:n], arr[n:2*n], 16); err != nil {
		t.Errorf("adjacent planes: %v", err)
	}
	if _, err := NewChromaPair(arr[n:2*n], arr[:n], 16); err != nil {
		t.Errorf("adjacent planes, Cr first: %v", err)
	}
	if _, err := NewChromaPair(arr[:n], arr[n-1:2*n-1], 16); err == nil {
		t.Error("planes overlapping by one byte were accepted")
	}
	if _, err := NewChromaPair(arr[n-1:2*n-1], arr[:n], 16); err == nil {
		t.Error("planes overlapping by one byte (Cr first) were accepted")
	}
	if _, err := NewChromaPair(arr[:n], arr[:n], 16); err == nil {
		t.Error("the same plane twice was accepted")
	}
	if _, err := NewChromaPair(arr[:n], arr[n:2*n-1], 16); err == nil {
		t.Error("planes of different lengths were accepted")
	}
	if _, err := NewChromaPair(arr[:n], arr[n:2*n], 15); err == nil {
		t.Error("stride 15 was accepted")
	}
	if _, err := NewChromaPair(arr[n:2*n], arr[:n], 15); err == nil {
		t.Error("stride 15 (Cr first) was accepted")
	}
}

func FuzzPlaneLuma(f *testing.F) {
	if !inPlaneChild(f) {
		f.Skip("judged in the child process")
		return
	}
	f.Add(uint16(480), uint16(16), uint16(480), uint16(24), int64(0), int64(24*6+6), uint8(3), uint8(0x11))
	f.Add(uint16(480), uint16(16), uint16(480), uint16(24), int64(16*3), int64(-1), uint8(0), uint8(0x22))
	f.Add(uint16(300), uint16(20), uint16(300), uint16(20), int64(math.MinInt64), int64(math.MaxInt64), uint8(9), uint8(0x33))
	f.Add(uint16(300), uint16(32), uint16(300), uint16(16), int64(5), int64(-(1 << 40)), uint8(255), uint8(0x01))
	f.Fuzz(func(t *testing.T, dn, dstStride, sn, srcStride uint16, dstOff, srcOff int64, sel, fr uint8) {
		if int64(int(dstOff)) != dstOff || int64(int(srcOff)) != srcOff {
			return
		}
		sz := lumaSizes[int(sel)%len(lumaSizes)]
		c := lumaCall{dstOff: int(dstOff), srcOff: int(srcOff), w: sz[0], h: sz[1], xFrac: int(fr & 3), yFrac: int(fr>>2) & 3, avg: sel&0x80 != 0}
		if int(sel)%11 == 10 {
			c.xFrac, c.yFrac = int(fr>>4)-4, int(fr&15)-4 // out-of-range fractions too
		}
		if _, err := NewPlane(make([]byte, dn), int(dstStride)); err != nil {
			return
		}
		if _, err := NewPlane(make([]byte, sn), int(srcStride)); err != nil {
			return
		}
		runLuma(t, c, int(dn), int(sn), int(dstStride), int(srcStride), dstOff^srcOff)
	})
}

func FuzzChromaPair(f *testing.F) {
	if !inPlaneChild(f) {
		f.Skip("judged in the child process")
		return
	}
	f.Add(uint16(224), uint16(16), uint16(336), uint16(24), int64(17), int64(24*3+3), uint8(0x44), uint8(0x35))
	f.Add(uint16(224), uint16(16), uint16(336), uint16(24), int64(-1), int64(0), uint8(0x01), uint8(0x77))
	f.Add(uint16(300), uint16(20), uint16(300), uint16(20), int64(math.MinInt64), int64(math.MaxInt64), uint8(0), uint8(0))
	f.Add(uint16(300), uint16(16), uint16(300), uint16(16), int64(3), int64(1<<50), uint8(0x80), uint8(0xff))
	f.Fuzz(func(t *testing.T, dn, dstStride, sn, srcStride uint16, dstOff, srcOff int64, sel, fr uint8) {
		if int64(int(dstOff)) != dstOff || int64(int(srcOff)) != srcOff {
			return
		}
		sizes := []int{2, 4, 8, 3, 16}
		c := chromaCall{dstOff: int(dstOff), srcOff: int(srcOff), w: sizes[int(sel&7)%len(sizes)], h: sizes[int(sel>>3&7)%len(sizes)],
			xFrac: int(fr>>4) % 10, yFrac: int(fr&15) % 10, avg: sel&0x80 != 0}
		if _, err := NewChromaPair(make([]byte, dn), make([]byte, dn), int(dstStride)); err != nil {
			return
		}
		if _, err := NewChromaPair(make([]byte, sn), make([]byte, sn), int(srcStride)); err != nil {
			return
		}
		runChroma(t, c, int(dn), int(sn), int(dstStride), int(srcStride), dstOff^srcOff)
	})
}
