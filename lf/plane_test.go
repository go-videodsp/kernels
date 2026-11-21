package lf

import (
	"bytes"
	"math"
	"testing"
)

// The contract of Plane is containment plus equality: a method accepts exactly
// the offsets the plain function accepts, writes exactly what the plain
// function writes, and never touches a byte outside the slice. These tests
// check all three over every offset around a plane (the first and last rows,
// where an edge macroblock of a padded picture sits, and well outside), in a
// buffer with guard bytes on both sides of the slice.

func didPanic(f func()) (p bool) {
	defer func() {
		if recover() != nil {
			p = true
		}
	}()
	f()
	return
}

const padN = 96

// cheapRand is a xorshift generator: the sweeps below build tens of thousands
// of buffers, and math/rand's source costs more to seed than the call it feeds
// (the sweeps run under emulation on the architecture lanes).
type cheapRand uint64

func newCheap(seed int64) *cheapRand {
	r := cheapRand(uint64(seed)*0x9E3779B97F4A7C15 + 0x2545F4914F6CDD1D)
	return &r
}

func (r *cheapRand) Intn(n int) int {
	x := uint64(*r)
	x ^= x << 13
	x ^= x >> 7
	x ^= x << 17
	*r = cheapRand(x)
	return int((x >> 11) % uint64(n))
}

// smooth fills b with values close enough for every filter to act.
func smoothFill(rng *cheapRand, b []byte) {
	for i := range b {
		b[i] = byte(100 + rng.Intn(6))
	}
}

// guarded returns a backing array of n+2*guard bytes and the n bytes between
// the guards, with the capacity cut so an append cannot reach a guard.
func padBuf(rng *cheapRand, n int) (back, mid []byte) {
	back = make([]byte, n+2*padN)
	smoothFill(rng, back)
	return back, back[padN : padN+n : padN+n]
}

type edgeOp struct {
	name  string
	plane func(p *Plane, off, ia, ib int, bS [4]uint8)
	plain func(pix []byte, off, stride, ia, ib int, bS [4]uint8)
}

var edgeOps = []edgeOp{
	{"LumaV", func(p *Plane, off, ia, ib int, bS [4]uint8) { p.LumaV(off, ia, ib, bS) },
		func(pix []byte, off, stride, ia, ib int, bS [4]uint8) { DeblockLumaV(pix, off, stride, ia, ib, bS) }},
	{"LumaH", func(p *Plane, off, ia, ib int, bS [4]uint8) { p.LumaH(off, ia, ib, bS) },
		func(pix []byte, off, stride, ia, ib int, bS [4]uint8) { DeblockLumaH(pix, off, stride, ia, ib, bS) }},
	{"LumaIntraV", func(p *Plane, off, ia, ib int, _ [4]uint8) { p.LumaIntraV(off, ia, ib) },
		func(pix []byte, off, stride, ia, ib int, _ [4]uint8) { DeblockLumaIntraV(pix, off, stride, ia, ib) }},
	{"LumaIntraH", func(p *Plane, off, ia, ib int, _ [4]uint8) { p.LumaIntraH(off, ia, ib) },
		func(pix []byte, off, stride, ia, ib int, _ [4]uint8) { DeblockLumaIntraH(pix, off, stride, ia, ib) }},
	{"ChromaV", func(p *Plane, off, ia, ib int, bS [4]uint8) { p.ChromaV(off, ia, ib, bS) },
		func(pix []byte, off, stride, ia, ib int, bS [4]uint8) { DeblockChromaV(pix, off, stride, ia, ib, bS) }},
	{"ChromaH", func(p *Plane, off, ia, ib int, bS [4]uint8) { p.ChromaH(off, ia, ib, bS) },
		func(pix []byte, off, stride, ia, ib int, bS [4]uint8) { DeblockChromaH(pix, off, stride, ia, ib, bS) }},
	{"ChromaIntraV", func(p *Plane, off, ia, ib int, _ [4]uint8) { p.ChromaIntraV(off, ia, ib) },
		func(pix []byte, off, stride, ia, ib int, _ [4]uint8) { DeblockChromaIntraV(pix, off, stride, ia, ib) }},
	{"ChromaIntraH", func(p *Plane, off, ia, ib int, _ [4]uint8) { p.ChromaIntraH(off, ia, ib) },
		func(pix []byte, off, stride, ia, ib int, _ [4]uint8) { DeblockChromaIntraH(pix, off, stride, ia, ib) }},
}

// checkEdgeOp runs op at off on two identical guarded buffers, through the
// Plane and through the plain function, and reports whether it was accepted.
func checkEdgeOp(t *testing.T, op edgeOp, n, stride, off, ia, ib int, bS [4]uint8, seed int64) bool {
	t.Helper()
	backA, midA := padBuf(newCheap(seed), n)
	backB, midB := padBuf(newCheap(seed), n)
	p, err := NewPlane(midA, stride)
	if err != nil {
		t.Fatalf("NewPlane(n=%d, stride=%d): %v", n, stride, err)
	}
	gotPanic := didPanic(func() { op.plane(&p, off, ia, ib, bS) })
	wantPanic := didPanic(func() { op.plain(midB, off, stride, ia, ib, bS) })
	if gotPanic != wantPanic {
		t.Fatalf("%s n=%d stride=%d off=%d: Plane panicked %v, the plain function %v", op.name, n, stride, off, gotPanic, wantPanic)
	}
	if !bytes.Equal(backA, backB) {
		t.Fatalf("%s n=%d stride=%d off=%d: the Plane left different bytes than the plain function (or touched a guard)", op.name, n, stride, off)
	}
	return !gotPanic
}

func TestPlaneEqualsPlainEverywhere(t *testing.T) {
	for _, stride := range []int{16, 17, 32} {
		for _, rows := range []int{9, 22} {
			n := stride * rows
			for _, op := range edgeOps {
				accepted, refused := 0, 0
				for off := -(5*stride + 24); off <= n+5*stride+24; off++ {
					rng := newCheap(int64(off)*7 + int64(stride))
					bS := [4]uint8{uint8(rng.Intn(4)), uint8(rng.Intn(4)), uint8(rng.Intn(4)), 3}
					if checkEdgeOp(t, op, n, stride, off, 20+rng.Intn(32), 20+rng.Intn(32), bS, int64(off)) {
						accepted++
					} else {
						refused++
					}
				}
				if refused == 0 {
					t.Errorf("%s n=%d stride=%d: no offset was refused", op.name, n, stride)
				}
				if rows == 22 && accepted == 0 {
					t.Errorf("%s n=%d stride=%d: no offset was accepted", op.name, n, stride)
				}
			}
		}
	}
}

// The extreme offsets: a wrapped subtraction must not let one through.
func TestPlaneRefusesExtremeOffsets(t *testing.T) {
	const n, stride = 16 * 24, 16
	for _, op := range edgeOps {
		for _, off := range []int{math.MinInt, math.MinInt + 1, math.MaxInt, math.MaxInt - 1, math.MinInt / 2, math.MaxInt/2 + 1, math.MinInt + 4, -n, 2 * n} {
			if checkEdgeOp(t, op, n, stride, off, 30, 30, [4]uint8{1, 1, 1, 1}, 1) {
				t.Errorf("%s accepted offset %d", op.name, off)
			}
		}
	}
}

func TestPlaneIndexesAndStrengthsStillRefused(t *testing.T) {
	back, mid := padBuf(newCheap(1), 16*24)
	p, err := NewPlane(mid, 16)
	if err != nil {
		t.Fatal(err)
	}
	off := 16*8 + 8
	for _, op := range edgeOps {
		for _, c := range []struct{ ia, ib int }{{-1, 30}, {52, 30}, {30, -1}, {30, 52}} {
			if !didPanic(func() { op.plane(&p, off, c.ia, c.ib, [4]uint8{}) }) {
				t.Errorf("%s accepted indexes %d, %d", op.name, c.ia, c.ib)
			}
		}
	}
	for _, name := range []string{"LumaV", "LumaH", "ChromaV", "ChromaH"} {
		for _, op := range edgeOps {
			if op.name != name {
				continue
			}
			for k := 0; k < 4; k++ {
				bS := [4]uint8{}
				bS[k] = 4
				if !didPanic(func() { op.plane(&p, off, 30, 30, bS) }) {
					t.Errorf("%s accepted bS[%d] = 4", name, k)
				}
			}
		}
	}
	_ = back
}

func TestZeroPlaneRefusesEverything(t *testing.T) {
	var p Plane
	for _, op := range edgeOps {
		for off := -3; off < 40; off++ {
			if !didPanic(func() { op.plane(&p, off, 30, 30, [4]uint8{}) }) {
				t.Errorf("%s: the zero Plane accepted offset %d", op.name, off)
			}
		}
	}
	var q PlanePair
	off := 0
	for _, f := range []func(){
		func() { q.ChromaPairV(off, 30, 30, 30, 30, [4]uint8{}) },
		func() { q.ChromaPairH(off, 30, 30, 30, 30, [4]uint8{}) },
		func() { q.ChromaPairIntraV(off, 30, 30, 30, 30) },
		func() { q.ChromaPairIntraH(off, 30, 30, 30, 30) },
	} {
		for off = -3; off < 40; off++ {
			if !didPanic(f) {
				t.Errorf("the zero PlanePair accepted offset %d", off)
			}
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

func TestPlanePairEqualsPlainEverywhere(t *testing.T) {
	type pairOp struct {
		name  string
		pair  func(q *PlanePair, off int, ia, ib, ja, jb int, bS [4]uint8)
		plain func(cb, cr []byte, off, stride, ia, ib, ja, jb int, bS [4]uint8)
	}
	ops := []pairOp{
		{"V", func(q *PlanePair, off int, ia, ib, ja, jb int, bS [4]uint8) { q.ChromaPairV(off, ia, ib, ja, jb, bS) },
			func(cb, cr []byte, off, stride, ia, ib, ja, jb int, bS [4]uint8) {
				DeblockChromaPairV(cb, cr, off, stride, ia, ib, ja, jb, bS)
			}},
		{"H", func(q *PlanePair, off int, ia, ib, ja, jb int, bS [4]uint8) { q.ChromaPairH(off, ia, ib, ja, jb, bS) },
			func(cb, cr []byte, off, stride, ia, ib, ja, jb int, bS [4]uint8) {
				DeblockChromaPairH(cb, cr, off, stride, ia, ib, ja, jb, bS)
			}},
		{"IntraV", func(q *PlanePair, off int, ia, ib, ja, jb int, _ [4]uint8) { q.ChromaPairIntraV(off, ia, ib, ja, jb) },
			func(cb, cr []byte, off, stride, ia, ib, ja, jb int, _ [4]uint8) {
				DeblockChromaPairIntraV(cb, cr, off, stride, ia, ib, ja, jb)
			}},
		{"IntraH", func(q *PlanePair, off int, ia, ib, ja, jb int, _ [4]uint8) { q.ChromaPairIntraH(off, ia, ib, ja, jb) },
			func(cb, cr []byte, off, stride, ia, ib, ja, jb int, _ [4]uint8) {
				DeblockChromaPairIntraH(cb, cr, off, stride, ia, ib, ja, jb)
			}},
	}
	for _, stride := range []int{16, 24} {
		n := stride * 14
		for _, op := range ops {
			accepted := 0
			offs := []int{math.MinInt, math.MaxInt, math.MinInt / 2, math.MaxInt/2 + 1}
			for off := -(4*stride + 16); off <= n+4*stride+16; off++ {
				offs = append(offs, off)
			}
			for _, off := range offs {
				rng := newCheap(int64(off))
				ia, ib, ja, jb := 20+rng.Intn(32), 20+rng.Intn(32), 20+rng.Intn(32), 20+rng.Intn(32)
				bS := [4]uint8{uint8(rng.Intn(4)), 3, uint8(rng.Intn(4)), 0}
				ba, ma := padBuf(newCheap(int64(off)+1), n)
				bb, mb := padBuf(newCheap(int64(off)+2), n)
				ca, na := padBuf(newCheap(int64(off)+1), n)
				cb, nb := padBuf(newCheap(int64(off)+2), n)
				q, err := NewPlanePair(ma, mb, stride)
				if err != nil {
					t.Fatal(err)
				}
				got := didPanic(func() { op.pair(&q, off, ia, ib, ja, jb, bS) })
				want := didPanic(func() { op.plain(na, nb, off, stride, ia, ib, ja, jb, bS) })
				if got != want {
					t.Fatalf("%s stride=%d off=%d: pair panicked %v, plain %v", op.name, stride, off, got, want)
				}
				if !bytes.Equal(ba, ca) || !bytes.Equal(bb, cb) {
					t.Fatalf("%s stride=%d off=%d: bytes differ from the plain function (or a guard was touched)", op.name, stride, off)
				}
				if !got {
					accepted++
				}
			}
			if accepted == 0 {
				t.Errorf("%s stride=%d: nothing accepted", op.name, stride)
			}
		}
	}
}

func TestNewPlanePairRefusals(t *testing.T) {
	arr := make([]byte, 1024)
	const n = 256
	if _, err := NewPlanePair(arr[:n], arr[n:2*n], 16); err != nil {
		t.Errorf("adjacent planes: %v", err)
	}
	if _, err := NewPlanePair(arr[n:2*n], arr[:n], 16); err != nil {
		t.Errorf("adjacent planes, Cr first: %v", err)
	}
	if _, err := NewPlanePair(arr[:n], arr[n-1:2*n-1], 16); err == nil {
		t.Error("planes overlapping by one byte were accepted")
	}
	if _, err := NewPlanePair(arr[n-1:2*n-1], arr[:n], 16); err == nil {
		t.Error("planes overlapping by one byte (Cr first) were accepted")
	}
	if _, err := NewPlanePair(arr[:n], arr[:n], 16); err == nil {
		t.Error("the same plane twice was accepted")
	}
	if _, err := NewPlanePair(arr[:n], arr[n:2*n-1], 16); err == nil {
		t.Error("planes of different lengths were accepted")
	}
	if _, err := NewPlanePair(arr[:n], arr[n:2*n], 15); err == nil {
		t.Error("a Cb plane with stride 15 was accepted")
	}
	if _, err := NewPlanePair(arr[:n], arr[n:2*n], 17); err != nil {
		t.Errorf("stride 17: %v", err)
	}
	if _, err := NewPlanePair(arr[:n], arr[n:n+n], math.MaxInt/2+1); err == nil {
		t.Error("a stride of half the int range was accepted")
	}
}

func FuzzPlane(f *testing.F) {
	f.Add(uint16(352), uint16(16), int64(4), uint8(0), uint8(30), uint8(30), uint32(0x01010101))
	f.Add(uint16(352), uint16(16), int64(64), uint8(1), uint8(40), uint8(40), uint32(0x03020100))
	f.Add(uint16(400), uint16(40), int64(-1), uint8(4), uint8(51), uint8(51), uint32(0))
	f.Add(uint16(400), uint16(40), int64(math.MinInt64), uint8(7), uint8(0), uint8(0), uint32(0))
	f.Fuzz(func(t *testing.T, n, stride uint16, off int64, op, ia, ib uint8, bsBits uint32) {
		o := edgeOps[int(op)%len(edgeOps)]
		bS := [4]uint8{uint8(bsBits), uint8(bsBits >> 8), uint8(bsBits >> 16), uint8(bsBits >> 24)}
		if int64(int(off)) != off {
			return // not representable on this target
		}
		nn, ss := int(n), int(stride)
		if _, err := NewPlane(make([]byte, nn), ss); err != nil {
			return
		}
		checkEdgeOp(t, o, nn, ss, int(off), int(ia)%64-4, int(ib)%64-4, bS, int64(off))
	})
}
