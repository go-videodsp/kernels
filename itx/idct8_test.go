package itx

import (
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/go-videodsp/kernels/internal/boundstest"
)

type blockKind int

const (
	kindDC       blockKind = iota // only the first coefficient
	kindSparse                    // a few coefficients
	kindDense                     // realistic magnitudes
	kindExtreme                   // the whole int32 range: wraps like the reference
	kindZero                      // nothing
	kindFirstRow                  // zero everywhere but the first row
	kindLastLane                  // only the last coefficient
	numKinds
)

func randomBlock(r *rand.Rand, k blockKind) (coef, scale [64]int32) {
	for i := range scale {
		scale[i] = int32(r.Intn(1600)) // LevelScale8x8 is at most 255 * 16 * 6
	}
	switch k {
	case kindDC:
		coef[0] = int32(r.Intn(4001) - 2000)
	case kindSparse:
		for n := 1 + r.Intn(6); n > 0; n-- {
			coef[r.Intn(64)] = int32(r.Intn(401) - 200)
		}
	case kindDense:
		for i := range coef {
			coef[i] = int32(r.Intn(201) - 100)
		}
	case kindExtreme:
		for i := range coef {
			coef[i] = int32(r.Uint32())
			scale[i] = int32(r.Uint32())
		}
	case kindFirstRow:
		for i := 0; i < 8; i++ {
			coef[i] = int32(r.Intn(401) - 200)
		}
	case kindLastLane:
		coef[63] = int32(r.Intn(401) - 200)
	}
	return coef, scale
}

func randomPlane(r *rand.Rand, n int) []byte {
	b := make([]byte, n)
	for i := range b {
		switch r.Intn(4) {
		case 0:
			b[i] = 0
		case 1:
			b[i] = 255
		default:
			b[i] = byte(r.Intn(256))
		}
	}
	return b
}

// Every kernel in the package must equal the portable reference on every
// block shape and every qpDiv, with strides that leave a gap, and must leave
// the bytes around the block alone.
func TestDequant8x8IDCTAddMatchesReference(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	for k := blockKind(0); k < numKinds; k++ {
		for qpDiv := 0; qpDiv <= 31; qpDiv++ {
			for _, stride := range []int{8, 9, 24, 64} {
				for n := 0; n < 6; n++ {
					coef, scale := randomBlock(r, k)
					size := 3*stride + 8*stride + 16
					got := randomPlane(r, size)
					want := append([]byte(nil), got...)
					off := 1 + r.Intn(stride+8)
					saved := coef
					Dequant8x8IDCTAdd(got, off, stride, &coef, &scale, qpDiv)
					dequant8x8IDCTAddRef(want, off, stride, &saved, &scale, qpDiv)
					if coef != saved {
						t.Fatal("coef was modified")
					}
					if !bytes.Equal(got, want) {
						t.Fatalf("kind %d qpDiv %d stride %d off %d: first difference at %d", k, qpDiv, stride, off, firstDiff(got, want))
					}
				}
			}
		}
	}
}

func firstDiff(a, b []byte) int {
	for i := range a {
		if a[i] != b[i] {
			return i
		}
	}
	return -1
}

// The reference itself against a different formulation: the transform as the
// matrix of Equation 8-338..8-364 applied to unit vectors must be linear in
// the sense the standard gives, and the DC-only shortcut is the closed form.
func TestReferenceDCOnly(t *testing.T) {
	var coef, scale [64]int32
	for i := range scale {
		scale[i] = 16
	}
	for _, c := range []int32{-5000, -64, -1, 0, 1, 3, 63, 64, 777, 5000} {
		for qpDiv := 0; qpDiv <= 12; qpDiv++ {
			coef[0] = c
			plane := bytes.Repeat([]byte{100}, 64)
			dequant8x8IDCTAddRef(plane, 0, 8, &coef, &scale, qpDiv)
			var d int32
			if qpDiv >= 6 {
				d = c * 16 << uint(qpDiv-6)
			} else {
				d = (c*16 + 1<<uint(5-qpDiv)) >> uint(6-qpDiv)
			}
			v := 100 + (d+32)>>6
			if v < 0 {
				v = 0
			}
			if v > 255 {
				v = 255
			}
			for i, p := range plane {
				if int32(p) != v {
					t.Fatalf("c=%d qpDiv=%d sample %d = %d, want %d", c, qpDiv, i, p, v)
				}
			}
		}
	}
}

func TestZeroBlockLeavesThePrediction(t *testing.T) {
	var coef, scale [64]int32
	plane := randomPlane(rand.New(rand.NewSource(1)), 64)
	want := append([]byte(nil), plane...)
	Dequant8x8IDCTAdd(plane, 0, 8, &coef, &scale, 5)
	if !bytes.Equal(plane, want) {
		t.Fatal("a zero block changed the prediction")
	}
}

func refused(f func()) (p bool) {
	defer func() { p = recover() != nil }()
	f()
	return
}

var hostile = []int{math.MaxInt, math.MaxInt - 1, math.MaxInt/2 + 1, math.MaxInt/2 + 2, math.MaxInt/2 - 1, math.MaxInt / 4 * 3, -1, -2, math.MinInt, math.MinInt + 1}

// Strides, offsets and shifts that used to wrap bounds proofs, and the other
// hostile values, against a 64-byte plane: none can be a legal call.
func TestHostileArgumentsAreRefused(t *testing.T) {
	var coef, scale [64]int32
	buf := func() []byte { return make([]byte, 64) }
	for _, v := range hostile {
		cases := map[string]func(){
			"stride": func() { Dequant8x8IDCTAdd(buf(), 0, v, &coef, &scale, 3) },
			"offset": func() { Dequant8x8IDCTAdd(buf(), v, 8, &coef, &scale, 3) },
			"qpDiv":  func() { Dequant8x8IDCTAdd(buf(), 0, 8, &coef, &scale, v) },
		}
		for name, f := range cases {
			if !refused(f) {
				t.Errorf("%s value %d was accepted", name, v)
			}
		}
	}
	for name, f := range map[string]func(){
		"short stride": func() { Dequant8x8IDCTAdd(buf(), 0, 7, &coef, &scale, 3) },
		"short plane":  func() { Dequant8x8IDCTAdd(make([]byte, 63), 0, 8, &coef, &scale, 3) },
		"empty plane":  func() { Dequant8x8IDCTAdd(nil, 0, 8, &coef, &scale, 3) },
		"qpDiv 32":     func() { Dequant8x8IDCTAdd(buf(), 0, 8, &coef, &scale, 32) },
	} {
		if !refused(f) {
			t.Errorf("%s was accepted", name)
		}
	}
	if refused(func() { Dequant8x8IDCTAdd(buf(), 0, 8, &coef, &scale, 31) }) {
		t.Error("a legal call was refused")
	}
}

// FuzzBoundsRefusal compares the outcome of a call with the exact-integer
// oracle: it panics if and only if the 8x8 footprint does not fit.
func FuzzBoundsRefusal(f *testing.F) {
	f.Add(64, 0, 8, 3)
	f.Add(64, 0, math.MaxInt/2+1, 3)
	f.Add(64, math.MaxInt, 16, 31)
	f.Add(200, 17, 23, 0)
	f.Fuzz(func(t *testing.T, n, off, stride, qpDiv int) {
		if n < 0 || n > 4096 {
			return
		}
		var coef, scale [64]int32
		dst := make([]byte, n)
		fits := boundstest.Fits(n, off, stride, 0, 7, 0, 7) && stride >= 8 && qpDiv >= 0 && qpDiv <= 31
		got := !refused(func() { Dequant8x8IDCTAdd(dst, off, stride, &coef, &scale, qpDiv) })
		if got != fits {
			t.Fatalf("n=%d off=%d stride=%d qpDiv=%d: accepted=%v, oracle fits=%v", n, off, stride, qpDiv, got, fits)
		}
	})
}

// FuzzDequant8x8IDCTAdd compares the dispatched kernel with the reference on
// arbitrary coefficient and scale bytes and prediction samples.
func FuzzDequant8x8IDCTAdd(f *testing.F) {
	f.Add([]byte("seed"), uint8(3), uint8(0))
	f.Add(bytes.Repeat([]byte{0xff}, 600), uint8(31), uint8(5))
	f.Fuzz(func(t *testing.T, data []byte, qp, strideExtra uint8) {
		var coef, scale [64]int32
		for i := range coef {
			coef[i] = int32(fuzzWord(data, 4*i))
			scale[i] = int32(fuzzWord(data, 256+4*i))
		}
		stride := 8 + int(strideExtra)%40
		plane := make([]byte, 8*stride+8)
		for i := range plane {
			plane[i] = byte(fuzzWord(data, 512+i))
		}
		want := append([]byte(nil), plane...)
		qpDiv := int(qp) % 32
		Dequant8x8IDCTAdd(plane, 3, stride, &coef, &scale, qpDiv)
		dequant8x8IDCTAddRef(want, 3, stride, &coef, &scale, qpDiv)
		if !bytes.Equal(plane, want) {
			t.Fatalf("first difference at %d", firstDiff(plane, want))
		}
	})
}

func fuzzWord(data []byte, i int) uint32 {
	var w uint32
	for k := 0; k < 4; k++ {
		if len(data) > 0 {
			w |= uint32(data[(i+k)%len(data)]) << (8 * uint(k))
		}
	}
	return w
}

// The reference on a block whose answer is known by hand: a single
// coefficient 64 at (0, 0) with scale 16 and qpDiv 6 is d = 1024, every
// sample (1024 + 32) >> 6 = 16.
func TestReferenceKnownValue(t *testing.T) {
	var coef, scale [64]int32
	coef[0] = 64
	for i := range scale {
		scale[i] = 16
	}
	plane := make([]byte, 64)
	dequant8x8IDCTAddRef(plane, 0, 8, &coef, &scale, 6)
	for i, p := range plane {
		if p != 16 {
			t.Fatal(fmt.Sprint("sample ", i, " = ", p))
		}
	}
}
