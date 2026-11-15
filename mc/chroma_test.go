package mc

import (
	"bytes"
	"math"
	"math/rand"
	"strconv"
	"testing"

	"github.com/go-videodsp/kernels/internal/boundstest"
)

var chromaSizes = [][2]int{{8, 8}, {8, 4}, {8, 2}, {4, 8}, {4, 4}, {4, 2}, {2, 8}, {2, 4}, {2, 2}}

func chromaPlane(r *rand.Rand, n int) []byte {
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

func chromaDiff(a, b []byte) int {
	for i := range a {
		if a[i] != b[i] {
			return i
		}
	}
	return -1
}

// Every kernel equals the portable reference, for all nine sizes, all 64
// fraction pairs, strides with a gap and one and two planes, and leaves the
// bytes around the block alone.
func TestPutChromaMatchesReference(t *testing.T) {
	r := rand.New(rand.NewSource(17))
	for _, sz := range chromaSizes {
		w, h := sz[0], sz[1]
		for xf := 0; xf < 8; xf++ {
			for yf := 0; yf < 8; yf++ {
				ss, ds := w+1+r.Intn(20), w+r.Intn(20)
				src0, src1 := chromaPlane(r, (h+2)*ss+8), chromaPlane(r, (h+2)*ss+8)
				d0, d1 := chromaPlane(r, (h+1)*ds+8), chromaPlane(r, (h+1)*ds+8)
				w0, w1 := append([]byte(nil), d0...), append([]byte(nil), d1...)
				so, do := r.Intn(5), r.Intn(5)
				PutChroma(d0, do, ds, src0, so, ss, w, h, xf, yf)
				putChromaRef(w0, do, ds, src0, so, ss, w, h, xf, yf)
				if !bytes.Equal(d0, w0) {
					t.Fatalf("PutChroma %dx%d frac (%d,%d): first difference at %d", w, h, xf, yf, chromaDiff(d0, w0))
				}
				PutChromaPair(d0, d1, do, ds, src0, src1, so, ss, w, h, xf, yf)
				putChromaRef(w0, do, ds, src0, so, ss, w, h, xf, yf)
				putChromaRef(w1, do, ds, src1, so, ss, w, h, xf, yf)
				if !bytes.Equal(d0, w0) || !bytes.Equal(d1, w1) {
					t.Fatalf("PutChromaPair %dx%d frac (%d,%d): planes differ", w, h, xf, yf)
				}
			}
		}
	}
}

// putChromaPairRef is what the portable build runs: one plane or two.
func TestPutChromaPairRef(t *testing.T) {
	r := rand.New(rand.NewSource(19))
	src0, src1 := chromaPlane(r, 100), chromaPlane(r, 100)
	for _, planes := range []int{1, 2} {
		d0, d1 := make([]byte, 64), make([]byte, 64)
		w0, w1 := make([]byte, 64), make([]byte, 64)
		putChromaPairRef(d0, d1, 0, 8, src0, src1, 0, 9, 4, 4, 3, 6, planes)
		putChromaRef(w0, 0, 8, src0, 0, 9, 4, 4, 3, 6)
		if planes == 2 {
			putChromaRef(w1, 0, 8, src1, 0, 9, 4, 4, 3, 6)
		}
		if !bytes.Equal(d0, w0) || !bytes.Equal(d1, w1) {
			t.Fatalf("planes %d: result differs", planes)
		}
	}
}

// The reference by hand: a fraction of zero copies, and the centre of four
// samples is their mean.
func TestChromaReferenceKnown(t *testing.T) {
	src := []byte{10, 20, 0, 30, 40, 0, 0, 0, 0}
	dst := make([]byte, 4)
	putChromaRef(dst, 0, 2, src, 0, 3, 2, 2, 0, 0)
	if !bytes.Equal(dst, []byte{10, 20, 30, 40}) {
		t.Fatalf("copy: %v", dst)
	}
	putChromaRef(dst, 0, 2, src, 0, 3, 1, 1, 4, 4)
	// (16*10 + 16*20 + 16*30 + 16*40 + 32) >> 6 = 1632 >> 6 = 25
	if dst[0] != 25 {
		t.Fatalf("centre: %v", dst)
	}
}

func TestChromaRefusals(t *testing.T) {
	buf := func() []byte { return make([]byte, 400) }
	for name, f := range map[string]func(){
		"width":      func() { PutChroma(buf(), 0, 16, buf(), 0, 16, 16, 8, 1, 1) },
		"height":     func() { PutChroma(buf(), 0, 16, buf(), 0, 16, 8, 3, 1, 1) },
		"frac low":   func() { PutChroma(buf(), 0, 16, buf(), 0, 16, 8, 8, -1, 1) },
		"frac high":  func() { PutChroma(buf(), 0, 16, buf(), 0, 16, 8, 8, 1, 8) },
		"dst stride": func() { PutChroma(buf(), 0, 7, buf(), 0, 16, 8, 8, 1, 1) },
		"src stride": func() { PutChroma(buf(), 0, 16, buf(), 0, 8, 8, 8, 1, 1) },
		"short src":  func() { PutChroma(buf(), 0, 16, make([]byte, 8*16+8), 0, 16, 8, 8, 1, 1) },
		"short dst":  func() { PutChroma(make([]byte, 100), 0, 16, buf(), 0, 16, 8, 8, 1, 1) },
		"overlap": func() {
			b := buf()
			PutChroma(b, 0, 16, b, 4, 16, 8, 8, 1, 1)
		},
		"pair planes overlap": func() {
			b := buf()
			PutChromaPair(b, b, 0, 16, buf(), buf(), 0, 16, 8, 8, 1, 1)
		},
		"pair dst on src": func() {
			b, c := buf(), buf()
			PutChromaPair(b, buf(), 0, 16, c, b, 0, 16, 8, 8, 1, 1)
		},
	} {
		if !refused(f) {
			t.Errorf("%s was accepted", name)
		}
	}
	for _, v := range hostile {
		for name, f := range map[string]func(){
			"dst stride": func() { PutChroma(buf(), 0, v, buf(), 0, 16, 8, 8, 1, 1) },
			"src stride": func() { PutChroma(buf(), 0, 16, buf(), 0, v, 8, 8, 1, 1) },
			"dst offset": func() { PutChroma(buf(), v, 16, buf(), 0, 16, 8, 8, 1, 1) },
			"src offset": func() { PutChroma(buf(), 0, 16, buf(), v, 16, 8, 8, 1, 1) },
			"pair":       func() { PutChromaPair(buf(), buf(), v, 16, buf(), buf(), v, 16, 4, 4, 1, 1) },
		} {
			if !refused(f) {
				t.Errorf("%s value %d was accepted", name, v)
			}
		}
	}
	// nothing is written on a refusal
	d := bytes.Repeat([]byte{7}, 64)
	if !refused(func() { PutChroma(d, 0, 8, make([]byte, 10), 0, 9, 8, 8, 1, 1) }) || !bytes.Equal(d, bytes.Repeat([]byte{7}, 64)) {
		t.Error("a refused call wrote")
	}
}

// FuzzChromaBoundsRefusal compares the outcome with the exact-integer oracle.
func FuzzChromaBoundsRefusal(f *testing.F) {
	f.Add(300, 300, 0, 16, 0, 16, uint8(0))
	f.Add(64, 64, 0, math.MaxInt/2+1, 8, 16, uint8(1))
	f.Add(64, 64, math.MaxInt, 16, math.MaxInt, 16, uint8(4))
	f.Fuzz(func(t *testing.T, dn, sn, dOff, dStride, sOff, sStride int, sizeIdx uint8) {
		if dn < 0 || sn < 0 || dn > 4096 || sn > 4096 {
			return
		}
		dst, src := make([]byte, dn), make([]byte, sn)
		sz := chromaSizes[int(sizeIdx)%len(chromaSizes)]
		w, h := sz[0], sz[1]
		fits := boundstest.Fits(dn, dOff, dStride, 0, h-1, 0, w-1) && dStride >= w &&
			boundstest.Fits(sn, sOff, sStride, 0, h, 0, w) && sStride >= w+1
		if got := !refused(func() { PutChroma(dst, dOff, dStride, src, sOff, sStride, w, h, 3, 5) }); got && !fits {
			t.Fatalf("PutChroma accepted a footprint the exact oracle rejects: dn=%d sn=%d dOff=%d dStride=%d sOff=%d sStride=%d %dx%d", dn, sn, dOff, dStride, sOff, sStride, w, h)
		}
	})
}

// FuzzPutChroma compares the dispatched kernel with the reference on
// arbitrary samples.
func FuzzPutChroma(f *testing.F) {
	f.Add([]byte("seed"), uint8(0), uint8(0), uint8(0))
	f.Add(bytes.Repeat([]byte{0xff, 0}, 300), uint8(63), uint8(7), uint8(8))
	f.Fuzz(func(t *testing.T, data []byte, xy, sizeIdx, extra uint8) {
		sz := chromaSizes[int(sizeIdx)%len(chromaSizes)]
		w, h := sz[0], sz[1]
		xf, yf := int(xy)&7, int(xy>>3)&7
		ss, ds := w+1+int(extra)%12, w+int(extra)%9
		src0, src1 := make([]byte, (h+1)*ss+w+3), make([]byte, (h+1)*ss+w+3)
		for i := range src0 {
			src0[i], src1[i] = fuzzByte(data, i), fuzzByte(data, 7*i+3)
		}
		d0, d1 := make([]byte, h*ds+w+1), make([]byte, h*ds+w+1)
		w0, w1 := make([]byte, len(d0)), make([]byte, len(d1))
		PutChromaPair(d0, d1, 1, ds, src0, src1, 2, ss, w, h, xf, yf)
		putChromaRef(w0, 1, ds, src0, 2, ss, w, h, xf, yf)
		putChromaRef(w1, 1, ds, src1, 2, ss, w, h, xf, yf)
		if !bytes.Equal(d0, w0) || !bytes.Equal(d1, w1) {
			t.Fatalf("%dx%d (%d,%d): difference", w, h, xf, yf)
		}
	})
}

func fuzzByte(data []byte, i int) byte {
	if len(data) == 0 {
		return 0
	}
	return data[i%len(data)]
}

func BenchmarkPutChroma(b *testing.B) {
	r := rand.New(rand.NewSource(3))
	src0, src1 := chromaPlane(r, 16*64), chromaPlane(r, 16*64)
	d0, d1 := make([]byte, 16*64), make([]byte, 16*64)
	for _, sz := range [][2]int{{8, 8}, {4, 4}, {2, 2}} {
		w, h := sz[0], sz[1]
		name := strconv.Itoa(w) + "x" + strconv.Itoa(h)
		b.Run("ref/"+name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				putChromaRef(d0, 0, 64, src0, 0, 64, w, h, 3, 5)
				putChromaRef(d1, 0, 64, src1, 0, 64, w, h, 3, 5)
			}
		})
		b.Run("pair/"+name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				PutChromaPair(d0, d1, 0, 64, src0, src1, 0, 64, w, h, 3, 5)
			}
		})
	}
}
