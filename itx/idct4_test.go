package itx

import (
	"bytes"
	"math"
	"math/rand"
	"testing"

	"github.com/go-videodsp/kernels/internal/boundstest"
)

func randomBlocks(r *rand.Rand, k blockKind) (coef [16][16]int32, scale [16]int32) {
	for i := range scale {
		scale[i] = int32(r.Intn(1600))
	}
	for b := range coef {
		switch k {
		case kindDC:
			coef[b][0] = int32(r.Intn(4001) - 2000)
		case kindSparse:
			for n := 1 + r.Intn(4); n > 0; n-- {
				coef[b][r.Intn(16)] = int32(r.Intn(401) - 200)
			}
		case kindDense:
			for i := range coef[b] {
				coef[b][i] = int32(r.Intn(201) - 100)
			}
		case kindExtreme:
			for i := range coef[b] {
				coef[b][i] = int32(r.Uint32())
			}
			for i := range scale {
				scale[i] = int32(r.Uint32())
			}
		case kindLastLane:
			coef[b][15] = int32(r.Intn(401) - 200)
		}
	}
	return coef, scale
}

func TestDequant4x4IDCTAddBlocksMatchesReference(t *testing.T) {
	r := rand.New(rand.NewSource(4))
	for k := blockKind(0); k < numKinds; k++ {
		for qpDiv := 0; qpDiv <= 31; qpDiv++ {
			for _, n := range []int{4, 16} {
				for _, skip := range []bool{false, true} {
					size := 16
					if n == 4 {
						size = 8
					}
					stride := size + r.Intn(40)
					for rep := 0; rep < 3; rep++ {
						coef, scale := randomBlocks(r, k)
						saved, savedScale := coef, scale
						mask := uint16(r.Uint32())
						if rep == 0 {
							mask = 0xffff
						}
						plane := randomPlane(r, size*stride+64)
						want := append([]byte(nil), plane...)
						off := r.Intn(30)
						Dequant4x4IDCTAddBlocks(plane, off, stride, n, &coef, mask, &scale, qpDiv, skip)
						dequant4x4IDCTAddBlocksRef(want, off, stride, &saved, mask&(1<<uint(n)-1), &savedScale, qpDiv, skip)
						if coef != saved || scale != savedScale {
							t.Fatal("an input was modified")
						}
						if !bytes.Equal(plane, want) {
							t.Fatalf("kind %d qpDiv %d n %d skip %v mask %#x stride %d: first difference at %d", k, qpDiv, n, skip, mask, stride, firstDiff(plane, want))
						}
					}
				}
			}
		}
	}
}

func TestLumaDCDequantMatchesReference(t *testing.T) {
	r := rand.New(rand.NewSource(6))
	for qpDiv := 0; qpDiv <= 31; qpDiv++ {
		for n := 0; n < 200; n++ {
			var got, want [16]int32
			for i := range got {
				switch n % 3 {
				case 0:
					got[i] = int32(r.Intn(2001) - 1000)
				case 1:
					got[i] = int32(r.Uint32())
				}
			}
			ls0 := int32(r.Intn(400))
			if n%7 == 0 {
				ls0 = int32(r.Uint32())
			}
			want = got
			LumaDCDequant(&got, ls0, qpDiv)
			lumaDCDequantRef(&want, ls0, qpDiv)
			if got != want {
				t.Fatalf("qpDiv %d: %v, want %v", qpDiv, got, want)
			}
		}
	}
}

// The reference by hand: a flat DC block of value v transforms to
// 16*v*scale in the first Hadamard output.
func TestLumaDCReferenceKnown(t *testing.T) {
	var dc [16]int32
	dc[0] = 3
	lumaDCDequantRef(&dc, 10, 6)
	for i, v := range dc {
		if v != 30 {
			t.Fatalf("dc[%d] = %d, want 30", i, v)
		}
	}
}

func TestBlocksZeroMaskAndRefusals(t *testing.T) {
	var coef [16][16]int32
	var scale [16]int32
	plane := randomPlane(rand.New(rand.NewSource(2)), 16*16)
	want := append([]byte(nil), plane...)
	Dequant4x4IDCTAddBlocks(plane, 0, 16, 16, &coef, 0, &scale, 3, false)
	Dequant4x4IDCTAddBlocks(plane, 0, 16, 4, &coef, 0xfff0, &scale, 3, false) // all bits at or above 4 ignored
	if !bytes.Equal(plane, want) {
		t.Fatal("an empty mask changed the plane")
	}
	buf := func() []byte { return make([]byte, 16*16) }
	for name, f := range map[string]func(){
		"n":            func() { Dequant4x4IDCTAddBlocks(buf(), 0, 16, 8, &coef, 1, &scale, 3, false) },
		"qpDiv low":    func() { Dequant4x4IDCTAddBlocks(buf(), 0, 16, 16, &coef, 1, &scale, -1, false) },
		"qpDiv high":   func() { Dequant4x4IDCTAddBlocks(buf(), 0, 16, 16, &coef, 1, &scale, 32, false) },
		"stride 16":    func() { Dequant4x4IDCTAddBlocks(buf(), 0, 15, 16, &coef, 1, &scale, 3, false) },
		"stride 8":     func() { Dequant4x4IDCTAddBlocks(buf(), 0, 7, 4, &coef, 1, &scale, 3, false) },
		"short plane":  func() { Dequant4x4IDCTAddBlocks(make([]byte, 255), 0, 16, 16, &coef, 1, &scale, 3, false) },
		"dc qpDiv":     func() { LumaDCDequant(&[16]int32{}, 1, 32) },
		"dc qpDiv low": func() { LumaDCDequant(&[16]int32{}, 1, -1) },
	} {
		if !refused(f) {
			t.Errorf("%s was accepted", name)
		}
	}
	for _, v := range hostile {
		for name, f := range map[string]func(){
			"stride": func() { Dequant4x4IDCTAddBlocks(buf(), 0, v, 16, &coef, 1, &scale, 3, false) },
			"offset": func() { Dequant4x4IDCTAddBlocks(buf(), v, 16, 4, &coef, 1, &scale, 3, false) },
		} {
			if !refused(f) {
				t.Errorf("%s value %d was accepted", name, v)
			}
		}
	}
}

// FuzzBlocksBoundsRefusal compares the outcome with the exact-integer oracle.
func FuzzBlocksBoundsRefusal(f *testing.F) {
	f.Add(300, 0, 16, 16, 3)
	f.Add(64, 0, math.MaxInt/2+1, 4, 3)
	f.Add(200, math.MaxInt, 16, 16, 31)
	f.Fuzz(func(t *testing.T, n, off, stride, blocks, qpDiv int) {
		if n < 0 || n > 4096 {
			return
		}
		var coef [16][16]int32
		var scale [16]int32
		dst := make([]byte, n)
		size := 16
		if blocks == 4 {
			size = 8
		}
		fits := (blocks == 4 || blocks == 16) && qpDiv >= 0 && qpDiv <= 31 && stride >= size &&
			boundstest.Fits(n, off, stride, 0, size-1, 0, size-1)
		got := !refused(func() { Dequant4x4IDCTAddBlocks(dst, off, stride, blocks, &coef, 1, &scale, qpDiv, false) })
		if got != fits {
			t.Fatalf("n=%d off=%d stride=%d blocks=%d qpDiv=%d: accepted=%v, oracle=%v", n, off, stride, blocks, qpDiv, got, fits)
		}
	})
}

// FuzzDequant4x4IDCTAddBlocks compares the dispatched kernel with the
// reference on arbitrary data.
func FuzzDequant4x4IDCTAddBlocks(f *testing.F) {
	f.Add([]byte("seed"), uint8(3), uint16(0xffff), uint8(0))
	f.Add(bytes.Repeat([]byte{0xff}, 2000), uint8(31), uint16(0x5a5a), uint8(5))
	f.Fuzz(func(t *testing.T, data []byte, qp uint8, mask uint16, flags uint8) {
		var coef [16][16]int32
		var scale [16]int32
		for b := range coef {
			for i := range coef[b] {
				coef[b][i] = int32(fuzzWord(data, 64*b+4*i))
			}
		}
		for i := range scale {
			scale[i] = int32(fuzzWord(data, 1024+4*i))
		}
		n := 16
		if flags&1 != 0 {
			n = 4
		}
		stride := 16 + int(flags>>2)%20
		plane := make([]byte, 16*stride+16)
		for i := range plane {
			plane[i] = byte(fuzzWord(data, 1100+i))
		}
		want := append([]byte(nil), plane...)
		qpDiv := int(qp) % 32
		Dequant4x4IDCTAddBlocks(plane, 2, stride, n, &coef, mask, &scale, qpDiv, flags&2 != 0)
		dequant4x4IDCTAddBlocksRef(want, 2, stride, &coef, mask&(1<<uint(n)-1), &scale, qpDiv, flags&2 != 0)
		if !bytes.Equal(plane, want) {
			t.Fatalf("first difference at %d", firstDiff(plane, want))
		}
	})
}

func BenchmarkIDCT4Blocks(b *testing.B) {
	r := rand.New(rand.NewSource(5))
	coef, scale := randomBlocks(r, kindSparse)
	plane := randomPlane(r, 16*64)
	for _, c := range []struct {
		name string
		f    func(dst []byte, off, stride int, coef *[16][16]int32, mask uint16, scale *[16]int32, qpDiv int, skipDC bool)
	}{
		{"ref", dequant4x4IDCTAddBlocksRef},
		{"kernel", func(dst []byte, off, stride int, coef *[16][16]int32, mask uint16, scale *[16]int32, qpDiv int, skipDC bool) {
			Dequant4x4IDCTAddBlocks(dst, off, stride, 16, coef, mask, scale, qpDiv, skipDC)
		}},
	} {
		b.Run("mb16/"+c.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				c.f(plane, 0, 64, &coef, 0xffff, &scale, 4, false)
			}
		})
		b.Run("one/"+c.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				c.f(plane, 0, 64, &coef, 1, &scale, 4, false)
			}
		})
	}
}
