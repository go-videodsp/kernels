package mc

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
)

// Oracles: a literal transcription of ITU-T H.264 (V16) (06/2026), clause
// 8.4.2.3.2, Equations 8-274, 8-275 and 8-276, one sample at a time. They do
// not share the reference's merged rounding term: the logWD >= 1 and
// logWD == 0 forms of 8-274 are separate branches here.

func clipSample(v int) byte {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return byte(v)
}

// weightedUni is 8-274 (or 8-275, the same with list 1's variables).
func weightedUni(p, w, o, logWD int) byte {
	if logWD >= 1 {
		return clipSample(((p*w + 1<<(logWD-1)) >> logWD) + o)
	}
	return clipSample(p*w + o)
}

// weightedBi is 8-276.
func weightedBi(p0, p1, w0, w1, o0, o1, logWD int) byte {
	return clipSample(((p0*w0 + p1*w1 + 1<<logWD) >> (logWD + 1)) + ((o0 + o1 + 1) >> 1))
}

type wlayout struct{ dOff, dStride, dLen, sOff, sStride, sLen int }

func randomWLayout(r *rand.Rand, w, h int) wlayout {
	var l wlayout
	l.dStride = w + r.Intn(20)
	l.dOff = r.Intn(2 * l.dStride)
	l.dLen = l.dOff + (h-1)*l.dStride + w + r.Intn(5)
	l.sStride = w + r.Intn(20)
	l.sOff = r.Intn(2 * l.sStride)
	l.sLen = l.sOff + (h-1)*l.sStride + w + r.Intn(5)
	return l
}

func wplane(r *rand.Rand, n, kind int) []byte {
	b := make([]byte, n)
	switch kind {
	case 0:
		fill(r, b, fillRandom, 1)
	case 1:
		fill(r, b, fillZero, 1)
	case 2:
		fill(r, b, fillMax, 1)
	default:
		fill(r, b, fillChecker, 7)
	}
	return b
}

var weightGrid = []int{-128, -127, -64, -33, -32, -17, -2, -1, 0, 1, 2, 5, 16, 31, 32, 33, 63, 64, 65, 100, 127, 128}
var offsetGrid = []int{-128, -127, -64, -3, -2, -1, 0, 1, 2, 3, 63, 126, 127}

func TestWeightUniExhaustive(t *testing.T) {
	r := rand.New(rand.NewSource(11))
	for _, sz := range sizes {
		w, h := sz[0], sz[1]
		t.Run(fmt.Sprintf("%dx%d", w, h), func(t *testing.T) {
			for logWD := 0; logWD <= 7; logWD++ {
				for weight := -128; weight <= 128; weight++ {
					for _, offset := range offsetGrid {
						l := randomWLayout(r, w, h)
						plane := wplane(r, l.dLen, (weight+256+offset)%4)
						want := append([]byte(nil), plane...)
						for y := 0; y < h; y++ {
							for x := 0; x < w; x++ {
								i := l.dOff + y*l.dStride + x
								want[i] = weightedUni(int(plane[i]), weight, offset, logWD)
							}
						}
						got := append([]byte(nil), plane...)
						WeightUni(got, l.dOff, l.dStride, w, h, weight, offset, logWD)
						if !bytes.Equal(got, want) {
							t.Fatalf("logWD %d weight %d offset %d: first difference at %d", logWD, weight, offset, firstDiff(got, want))
						}
						ref := append([]byte(nil), plane...)
						weightUniRef(ref, l.dOff, l.dStride, w, h, weight, offset, logWD)
						if !bytes.Equal(ref, want) {
							t.Fatalf("portable logWD %d weight %d offset %d: first difference at %d", logWD, weight, offset, firstDiff(ref, want))
						}
					}
				}
			}
		})
	}
}

func checkBi(t *testing.T, r *rand.Rand, w, h, w0, w1, o0, o1, logWD, kind int) {
	t.Helper()
	l := randomWLayout(r, w, h)
	d := wplane(r, l.dLen, kind)
	s := wplane(r, l.sLen, (kind+1)%4)
	want := append([]byte(nil), d...)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			want[l.dOff+y*l.dStride+x] = weightedBi(int(d[l.dOff+y*l.dStride+x]), int(s[l.sOff+y*l.sStride+x]), w0, w1, o0, o1, logWD)
		}
	}
	got := append([]byte(nil), d...)
	WeightBi(got, l.dOff, l.dStride, s, l.sOff, l.sStride, w, h, w0, w1, o0, o1, logWD)
	if !bytes.Equal(got, want) {
		t.Fatalf("%dx%d logWD %d w (%d,%d) o (%d,%d): first difference at %d", w, h, logWD, w0, w1, o0, o1, firstDiff(got, want))
	}
	ref := append([]byte(nil), d...)
	weightBiRef(ref, l.dOff, l.dStride, s, l.sOff, l.sStride, w, h, w0, w1, o0, o1, logWD)
	if !bytes.Equal(ref, want) {
		t.Fatalf("portable %dx%d logWD %d w (%d,%d) o (%d,%d): first difference at %d", w, h, logWD, w0, w1, o0, o1, firstDiff(ref, want))
	}
}

func TestWeightBiExhaustive(t *testing.T) {
	r := rand.New(rand.NewSource(12))
	for _, sz := range sizes {
		w, h := sz[0], sz[1]
		t.Run(fmt.Sprintf("%dx%d", w, h), func(t *testing.T) {
			for logWD := 0; logWD <= 7; logWD++ {
				pow := 1 << logWD
				ws := append([]int{pow, pow - 1, pow + 1, 128 - pow}, weightGrid...)
				for _, w0 := range ws {
					for _, w1 := range ws {
						if w0 < -128 || w0 > 128 || w1 < -128 || w1 > 128 {
							continue
						}
						for _, o := range [][2]int{{0, 0}, {-1, 0}, {0, -1}, {1, 0}, {127, 127}, {-128, -128}, {-128, 127}, {5, -9}} {
							checkBi(t, r, w, h, w0, w1, o[0], o[1], logWD, (w0+w1+logWD+257)%4)
						}
					}
				}
			}
			// Random, over the whole parameter space.
			for i := 0; i < 2000; i++ {
				checkBi(t, r, w, h, r.Intn(257)-128, r.Intn(257)-128, r.Intn(256)-128, r.Intn(256)-128, r.Intn(8), i%4)
			}
		})
	}
}

// With w0 = w1 = 2^logWD and a zero rounded offset, WeightBi is the default
// average of 8-273 (the implicit (32, 32) case among them): one fused kernel,
// the same bytes.
func TestWeightBiIsTheAverage(t *testing.T) {
	r := rand.New(rand.NewSource(13))
	for _, sz := range sizes {
		w, h := sz[0], sz[1]
		for logWD := 0; logWD <= 7; logWD++ {
			l := randomWLayout(r, w, h)
			d := wplane(r, l.dLen, 0)
			s := wplane(r, l.sLen, 0)
			want := append([]byte(nil), d...)
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					i := l.dOff + y*l.dStride + x
					want[i] = byte((int(d[i]) + int(s[l.sOff+y*l.sStride+x]) + 1) >> 1)
				}
			}
			got := append([]byte(nil), d...)
			WeightBi(got, l.dOff, l.dStride, s, l.sOff, l.sStride, w, h, 1<<logWD, 1<<logWD, 0, 0, logWD)
			if !bytes.Equal(got, want) {
				t.Fatalf("%dx%d logWD %d: first difference at %d", w, h, logWD, firstDiff(got, want))
			}
		}
	}
}

func TestWeightRefused(t *testing.T) {
	d := make([]byte, 64*64)
	s := make([]byte, 64*64)
	cases := []struct {
		name string
		call func()
	}{
		{"uni size", func() { WeightUni(d, 0, 64, 12, 8, 1, 0, 0) }},
		{"uni stride below width", func() { WeightUni(d, 0, 7, 8, 8, 1, 0, 0) }},
		{"uni negative offset", func() { WeightUni(d, -1, 64, 8, 8, 1, 0, 0) }},
		{"uni past end", func() { WeightUni(d, 64*64-7*64-7, 64, 8, 8, 1, 0, 0) }},
		{"uni logWD 8", func() { WeightUni(d, 0, 64, 8, 8, 1, 0, 8) }},
		{"uni logWD -1", func() { WeightUni(d, 0, 64, 8, 8, 1, 0, -1) }},
		{"uni weight 129", func() { WeightUni(d, 0, 64, 8, 8, 129, 0, 0) }},
		{"uni weight -129", func() { WeightUni(d, 0, 64, 8, 8, -129, 0, 0) }},
		{"uni offset 128", func() { WeightUni(d, 0, 64, 8, 8, 1, 128, 0) }},
		{"uni offset -129", func() { WeightUni(d, 0, 64, 8, 8, 1, -129, 0) }},
		{"bi dst past end", func() { WeightBi(d, 64*64-7*64-7, 64, s, 0, 64, 8, 8, 1, 1, 0, 0, 0) }},
		{"bi src past end", func() { WeightBi(d, 0, 64, s, 64*64-7*64-7, 64, 8, 8, 1, 1, 0, 0, 0) }},
		{"bi src stride below width", func() { WeightBi(d, 0, 64, s, 0, 3, 4, 4, 1, 1, 0, 0, 0) }},
		{"bi w0 129", func() { WeightBi(d, 0, 64, s, 0, 64, 8, 8, 129, 1, 0, 0, 0) }},
		{"bi w1 -129", func() { WeightBi(d, 0, 64, s, 0, 64, 8, 8, 1, -129, 0, 0, 0) }},
		{"bi o0 128", func() { WeightBi(d, 0, 64, s, 0, 64, 8, 8, 1, 1, 128, 0, 0) }},
		{"bi o1 -129", func() { WeightBi(d, 0, 64, s, 0, 64, 8, 8, 1, 1, 0, -129, 0) }},
		{"bi logWD 8", func() { WeightBi(d, 0, 64, s, 0, 64, 8, 8, 1, 1, 0, 0, 8) }},
		{"empty", func() { WeightBi(nil, 0, 4, nil, 0, 4, 4, 4, 1, 1, 0, 0, 0) }},
	}
	for _, c := range cases {
		mustPanic(t, c.name, c.call)
	}
	// Exactly fitting blocks are fine.
	WeightUni(d, 64*64-7*64-8, 64, 8, 8, 1, 0, 0)
	WeightBi(d, 64*64-7*64-8, 64, s, 64*64-7*64-8, 64, 8, 8, 1, 1, 0, 0, 0)
}

// Fuzz the parameters of both entry points against the oracles: the call
// either panics because a parameter or an extent is outside what the
// standard and the signature allow (decided here independently), or equals
// the oracle exactly with nothing outside the block touched.
func FuzzWeight(f *testing.F) {
	r := rand.New(rand.NewSource(14))
	plane := make([]byte, 40*40)
	fill(r, plane, fillRandom, 40)
	f.Add(plane, uint16(5), uint16(40), uint16(40), uint8(0), int16(32), int16(32), int16(0), int16(0), uint8(5), true)
	f.Add(plane, uint16(0), uint16(8), uint16(40), uint8(5), int16(-3), int16(77), int16(-128), int16(127), uint8(0), true)
	f.Add(plane, uint16(0), uint16(4), uint16(40), uint8(6), int16(128), int16(0), int16(9), int16(3), uint8(7), false)
	f.Add(plane[:100], uint16(90), uint16(16), uint16(16), uint8(1), int16(1), int16(1), int16(0), int16(0), uint8(1), true)

	f.Fuzz(func(t *testing.T, srcPlane []byte, off, dStride, sStride uint16, sizeIdx uint8, w0, w1, o0, o1 int16, logWD uint8, bi bool) {
		sz := sizes[int(sizeIdx)%len(sizes)]
		w, h := sz[0], sz[1]
		dst := make([]byte, 1024)
		for i := range dst {
			dst[i] = byte(i*13 + 5)
		}
		want := append([]byte(nil), dst...)
		do, ds, ss := int(off), int(dStride), int(sStride)
		sOff := int(off) / 2

		paramsOK := int(logWD) <= 7 && inRange(int(w0), -128, 128) && inRange(int(o0), -128, 127)
		fits := ds >= w && do+(h-1)*ds+w-1 < len(dst)
		if bi {
			paramsOK = paramsOK && inRange(int(w1), -128, 128) && inRange(int(o1), -128, 127)
			fits = fits && ss >= w && sOff+(h-1)*ss+w-1 < len(srcPlane)
		}

		var panicked bool
		func() {
			defer func() { panicked = recover() != nil }()
			if bi {
				WeightBi(dst, do, ds, srcPlane, sOff, ss, w, h, int(w0), int(w1), int(o0), int(o1), int(logWD))
			} else {
				WeightUni(dst, do, ds, w, h, int(w0), int(o0), int(logWD))
			}
		}()
		if panicked == (paramsOK && fits) {
			t.Fatalf("paramsOK=%v fits=%v panicked=%v", paramsOK, fits, panicked)
		}
		if panicked {
			if !bytes.Equal(dst, want) {
				t.Fatal("a refused call wrote to dst")
			}
			return
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				i := do + y*ds + x
				if bi {
					want[i] = weightedBi(int(want[i]), int(srcPlane[sOff+y*ss+x]), int(w0), int(w1), int(o0), int(o1), int(logWD))
				} else {
					want[i] = weightedUni(int(want[i]), int(w0), int(o0), int(logWD))
				}
			}
		}
		if !bytes.Equal(dst, want) {
			t.Fatalf("bi=%v %dx%d: first difference at %d", bi, w, h, firstDiff(dst, want))
		}
	})
}

func inRange(v, lo, hi int) bool { return v >= lo && v <= hi }

func BenchmarkWeight(b *testing.B) {
	const stride = 64
	d := make([]byte, stride*20)
	s := make([]byte, stride*20)
	for i := range s {
		s[i] = byte(i * 7)
	}
	for _, impl := range []string{"dispatch", "portable"} {
		for _, sz := range sizes {
			w, h := sz[0], sz[1]
			b.Run(fmt.Sprintf("%s/uni/%dx%d", impl, w, h), func(b *testing.B) {
				b.SetBytes(int64(w * h))
				for i := 0; i < b.N; i++ {
					if impl == "portable" {
						weightUniRef(d, 0, stride, w, h, 45, 3, 5)
					} else {
						WeightUni(d, 0, stride, w, h, 45, 3, 5)
					}
				}
			})
			b.Run(fmt.Sprintf("%s/bi/%dx%d", impl, w, h), func(b *testing.B) {
				b.SetBytes(int64(w * h))
				for i := 0; i < b.N; i++ {
					if impl == "portable" {
						weightBiRef(d, 0, stride, s, 0, stride, w, h, 21, 43, 2, 3, 5)
					} else {
						WeightBi(d, 0, stride, s, 0, stride, w, h, 21, 43, 2, 3, 5)
					}
				}
			})
		}
	}
}
