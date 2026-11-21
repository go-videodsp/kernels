package mc

import (
	"bytes"
	"errors"
	"testing"
)

// The sweeps that send refused blocks run in a child process (see
// plane_child_test.go), and a child's statements do not count in the parent's
// coverage. These tests run in the parent and hold the same branches with
// calls that are harmless even if a refusal were missing: a refused block they
// send never reaches the kernels as an unsafe one, because the refusal they use
// for the methods (planes that share memory) is one whose block is valid, and
// the others are asked of the check functions, which dispatch nothing.

func TestPlaneChecksNameTheirRefusal(t *testing.T) {
	_, d := planeBuf(1, 16*20)
	_, s := planeBuf(2, 24*20)
	dst, err := NewPlane(d, 16)
	if err != nil {
		t.Fatal(err)
	}
	src, err := NewPlane(s, 24)
	if err != nil {
		t.Fatal(err)
	}
	const dstOff, srcOff = 16*2 + 2, 24*6 + 6
	for _, c := range []struct {
		name                       string
		dstOff, srcOff, w, h, x, y int
		want                       error
	}{
		{"accepted", dstOff, srcOff, 8, 8, 1, 1, nil},
		{"size", dstOff, srcOff, 12, 8, 1, 1, errSize},
		{"fraction", dstOff, srcOff, 8, 8, 4, 1, errFrac},
		{"destination footprint", -1000, srcOff, 8, 8, 1, 1, errFootprint},
		{"source footprint", dstOff, -1000, 8, 8, 1, 1, errFootprint},
	} {
		if got := dst.checkLuma(c.dstOff, &src, c.srcOff, c.w, c.h, c.x, c.y); !errors.Is(got, c.want) {
			t.Errorf("luma %s: %v, want %v", c.name, got, c.want)
		}
	}
	// A valid block on planes that share memory: refused, and nothing written.
	shared := append([]byte(nil), d...)
	both, err := NewPlane(shared, 16)
	if err != nil {
		t.Fatal(err)
	}
	before := append([]byte(nil), shared...)
	if got := both.PutLuma(dstOff, &both, dstOff, 8, 8, 0, 0); !errors.Is(got, errShared) {
		t.Errorf("PutLuma on one plane: %v, want errShared", got)
	}
	if got := both.AvgLuma(dstOff, &both, dstOff, 8, 8, 0, 0); !errors.Is(got, errShared) {
		t.Errorf("AvgLuma on one plane: %v, want errShared", got)
	}
	if !bytes.Equal(shared, before) {
		t.Error("a refused call wrote the plane")
	}
	// An accepted block runs.
	if got := dst.PutLuma(dstOff, &src, srcOff, 8, 8, 1, 1); got != nil {
		t.Errorf("PutLuma: %v", got)
	}
	if got := dst.AvgLuma(dstOff, &src, srcOff, 8, 8, 1, 1); got != nil {
		t.Errorf("AvgLuma: %v", got)
	}
}

func TestChromaPairChecksNameTheirRefusal(t *testing.T) {
	_, cb := planeBuf(3, 16*12)
	_, cr := planeBuf(4, 16*12)
	_, scb := planeBuf(5, 16*12)
	_, scr := planeBuf(6, 16*12)
	dst, err := NewChromaPair(cb, cr, 16)
	if err != nil {
		t.Fatal(err)
	}
	src, err := NewChromaPair(scb, scr, 16)
	if err != nil {
		t.Fatal(err)
	}
	const off = 16*3 + 3
	for _, c := range []struct {
		name                       string
		dstOff, srcOff, w, h, x, y int
		want                       error
	}{
		{"accepted", off, off, 4, 4, 3, 5, nil},
		{"size", off, off, 3, 4, 3, 5, errSize},
		{"fraction", off, off, 4, 4, 8, 5, errFrac},
		{"destination footprint", -1000, off, 4, 4, 3, 5, errFootprint},
		{"source footprint", off, -1000, 4, 4, 3, 5, errFootprint},
	} {
		if got := dst.check(c.dstOff, &src, c.srcOff, c.w, c.h, c.x, c.y); !errors.Is(got, c.want) {
			t.Errorf("chroma %s: %v, want %v", c.name, got, c.want)
		}
	}
	// A valid block with the destination as its own source: refused, nothing written.
	cbBefore, crBefore := append([]byte(nil), cb...), append([]byte(nil), cr...)
	if got := dst.PutChroma(off, &dst, off, 4, 4, 0, 0); !errors.Is(got, errShared) {
		t.Errorf("PutChroma on one pair: %v, want errShared", got)
	}
	if got := dst.AvgChroma(off, &dst, off, 4, 4, 0, 0); !errors.Is(got, errShared) {
		t.Errorf("AvgChroma on one pair: %v, want errShared", got)
	}
	if !bytes.Equal(cb, cbBefore) || !bytes.Equal(cr, crBefore) {
		t.Error("a refused call wrote the planes")
	}
	if got := dst.PutChroma(off, &src, off, 4, 4, 3, 5); got != nil {
		t.Errorf("PutChroma: %v", got)
	}
	if got := dst.AvgChroma(off, &src, off, 4, 4, 3, 5); got != nil {
		t.Errorf("AvgChroma: %v", got)
	}
}
