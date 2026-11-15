package mc

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
)

// The oracle below is a per-sample transcription of ITU-T H.264 (V16)
// (06/2026) clause 8.4.2.2.1: Table 8-11 gives the offsets of the labelled
// samples of Figure 8-4, Equations 8-241 to 8-261 the values, Table 8-12 the
// assignment. It shares no code with the kernels and reads j both ways
// (Equations 8-245 and 8-246), which the standard says agree.

// table811 is Table 8-11: (xDZ, yDZ) of each full-sample letter.
var table811 = map[byte][2]int{
	'A': {0, -2}, 'B': {1, -2}, 'C': {0, -1}, 'D': {1, -1},
	'E': {-2, 0}, 'F': {-1, 0}, 'G': {0, 0}, 'H': {1, 0}, 'I': {2, 0}, 'J': {3, 0},
	'K': {-2, 1}, 'L': {-1, 1}, 'M': {0, 1}, 'N': {1, 1}, 'P': {2, 1}, 'Q': {3, 1},
	'R': {0, 2}, 'S': {1, 2}, 'T': {0, 3}, 'U': {1, 3},
}

// table812 is Table 8-12, indexed [xFrac][yFrac].
var table812 = [4][4]byte{
	{'G', 'd', 'h', 'n'},
	{'a', 'e', 'i', 'p'},
	{'b', 'f', 'j', 'q'},
	{'c', 'g', 'k', 'r'},
}

type spec struct {
	src    []byte
	stride int
	p      int // index of G
}

// at is the full sample at (dx, dy) from G.
func (s spec) at(dx, dy int) int { return int(s.src[s.p+dx+dy*s.stride]) }

// Z is the sample of Table 8-11 letter z.
func (s spec) Z(z byte) int { o := table811[z]; return s.at(o[0], o[1]) }

func clip1y(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

// b1 at row dy: 8-241 applied to the row's E F G H I J.
func (s spec) b1(dy int) int {
	return s.at(-2, dy) - 5*s.at(-1, dy) + 20*s.at(0, dy) + 20*s.at(1, dy) - 5*s.at(2, dy) + s.at(3, dy)
}

// h1 at column dx: 8-242 applied to the column's A C G M R T.
func (s spec) h1(dx int) int {
	return s.at(dx, -2) - 5*s.at(dx, -1) + 20*s.at(dx, 0) + 20*s.at(dx, 1) - 5*s.at(dx, 2) + s.at(dx, 3)
}

func (s spec) j1() int {
	// 8-245: cc dd h1 m1 ee ff are the vertical intermediates of the columns
	// -2 -1 0 1 2 3.
	viaColumns := s.h1(-2) - 5*s.h1(-1) + 20*s.h1(0) + 20*s.h1(1) - 5*s.h1(2) + s.h1(3)
	// 8-246: aa bb b1 s1 gg hh are the horizontal intermediates of the rows
	// -2 -1 0 1 2 3.
	viaRows := s.b1(-2) - 5*s.b1(-1) + 20*s.b1(0) + 20*s.b1(1) - 5*s.b1(2) + s.b1(3)
	if viaColumns != viaRows {
		panic("8-245 and 8-246 disagree")
	}
	return viaRows
}

// predict is predPartLX[x, y] for the fractional position (xFrac, yFrac) of
// the block sample whose G is spec.p.
func (s spec) predict(xFrac, yFrac int) byte {
	// Lazy: only the samples a position uses are inside its footprint.
	b := func() int { return clip1y((s.b1(0) + 16) >> 5) }  // 8-243
	h := func() int { return clip1y((s.h1(0) + 16) >> 5) }  // 8-244
	j := func() int { return clip1y((s.j1() + 512) >> 10) } // 8-247
	sv := func() int { return clip1y((s.b1(1) + 16) >> 5) } // 8-248
	m := func() int { return clip1y((s.h1(1) + 16) >> 5) }  // 8-249
	var v int
	switch table812[xFrac][yFrac] {
	case 'G':
		v = s.Z('G')
	case 'a':
		v = (s.Z('G') + b() + 1) >> 1 // 8-250
	case 'c':
		v = (s.Z('H') + b() + 1) >> 1 // 8-251
	case 'd':
		v = (s.Z('G') + h() + 1) >> 1 // 8-252
	case 'n':
		v = (s.Z('M') + h() + 1) >> 1 // 8-253
	case 'f':
		v = (b() + j() + 1) >> 1 // 8-254
	case 'i':
		v = (h() + j() + 1) >> 1 // 8-255
	case 'k':
		v = (j() + m() + 1) >> 1 // 8-256
	case 'q':
		v = (j() + sv() + 1) >> 1 // 8-257
	case 'e':
		v = (b() + h() + 1) >> 1 // 8-258
	case 'g':
		v = (b() + m() + 1) >> 1 // 8-259
	case 'p':
		v = (h() + sv() + 1) >> 1 // 8-260
	case 'r':
		v = (m() + sv() + 1) >> 1 // 8-261
	case 'b':
		v = b()
	case 'h':
		v = h()
	case 'j':
		v = j()
	}
	return byte(v)
}

// oracle fills want like the kernel must: put, or averaged (8-273).
func oracle(want []byte, dOff, dStride int, src []byte, sOff, sStride, w, h, fx, fy int, avg bool) {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := int(spec{src, sStride, sOff + y*sStride + x}.predict(fx, fy))
			d := dOff + y*dStride + x
			if avg {
				v = (int(want[d]) + v + 1) >> 1
			}
			want[d] = byte(v)
		}
	}
}

var sizes = [][2]int{{16, 16}, {16, 8}, {8, 16}, {8, 8}, {8, 4}, {4, 8}, {4, 4}}

type fillKind int

const (
	fillRandom fillKind = iota
	fillZero
	fillMax
	fillChecker // 0 / 255 alternating: largest filter overshoots
	fillInvChecker
	fillColumns
	fillCount
)

func fill(r *rand.Rand, b []byte, kind fillKind, stride int) {
	for i := range b {
		switch kind {
		case fillRandom:
			b[i] = byte(r.Intn(256))
		case fillZero:
			b[i] = 0
		case fillMax:
			b[i] = 255
		case fillChecker:
			b[i] = byte(255 * ((i + i/stride) & 1))
		case fillInvChecker:
			b[i] = byte(255 * (1 - (i+i/stride)&1))
		case fillColumns:
			b[i] = byte(255 * ((i % stride / 2) & 1))
		}
	}
}

// footprint returns the margins the position needs.
func margins(fx, fy int) (left, right, up, down int) {
	if fx != 0 {
		left, right = 2, 3
	}
	if fy != 0 {
		up, down = 2, 3
	}
	return
}

type layout struct {
	srcOff, srcStride, srcLen int
	dstOff, dstStride, dstLen int
}

func randomLayout(r *rand.Rand, w, h, fx, fy int) layout {
	left, right, up, down := margins(fx, fy)
	var l layout
	l.srcStride = w + left + right + r.Intn(40)
	l.srcOff = up*l.srcStride + left + r.Intn(3*l.srcStride)
	l.srcLen = l.srcOff + (h-1+down)*l.srcStride + w + right + r.Intn(8)
	l.dstStride = w + r.Intn(30)
	l.dstOff = r.Intn(2 * l.dstStride)
	l.dstLen = l.dstOff + (h-1)*l.dstStride + w + r.Intn(8)
	return l
}

// check runs both entry points and the portable reference on one case and
// compares each with the oracle, over the whole destination slice: samples
// outside the block must not move.
func check(t *testing.T, src, dstInit []byte, l layout, w, h, fx, fy int) {
	t.Helper()
	for _, avg := range []bool{false, true} {
		want := append([]byte(nil), dstInit...)
		oracle(want, l.dstOff, l.dstStride, src, l.srcOff, l.srcStride, w, h, fx, fy, avg)

		got := append([]byte(nil), dstInit...)
		if avg {
			AvgLuma(got, l.dstOff, l.dstStride, src, l.srcOff, l.srcStride, w, h, fx, fy)
		} else {
			PutLuma(got, l.dstOff, l.dstStride, src, l.srcOff, l.srcStride, w, h, fx, fy)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("API avg=%v %dx%d frac (%d,%d) layout %+v: first difference at %d", avg, w, h, fx, fy, l, firstDiff(got, want))
		}

		ref := append([]byte(nil), dstInit...)
		lumaRef(ref, l.dstOff, l.dstStride, src, l.srcOff, l.srcStride, w, h, fx, fy, avg)
		if !bytes.Equal(ref, want) {
			t.Fatalf("portable avg=%v %dx%d frac (%d,%d) layout %+v: first difference at %d", avg, w, h, fx, fy, l, firstDiff(ref, want))
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

// Every position, every block size, put and average, on random planes and on
// the planes that drive the filter to its extremes.
func TestExhaustivePositionsAndSizes(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for _, sz := range sizes {
		w, h := sz[0], sz[1]
		for fy := 0; fy < 4; fy++ {
			for fx := 0; fx < 4; fx++ {
				t.Run(fmt.Sprintf("%dx%d/%c", w, h, table812[fx][fy]), func(t *testing.T) {
					for kind := fillKind(0); kind < fillCount; kind++ {
						rounds := 1
						if kind == fillRandom {
							rounds = 24
						}
						for i := 0; i < rounds; i++ {
							l := randomLayout(r, w, h, fx, fy)
							src := make([]byte, l.srcLen)
							fill(r, src, kind, l.srcStride)
							dst := make([]byte, l.dstLen)
							fill(r, dst, fillRandom, 1)
							check(t, src, dst, l, w, h, fx, fy)
						}
					}
				})
			}
		}
	}
}

// The tightest legal planes: the first and the last index the footprint
// touches are the plane's ends. One sample less panics.
func TestTightFootprint(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	for _, sz := range sizes {
		w, h := sz[0], sz[1]
		for fy := 0; fy < 4; fy++ {
			for fx := 0; fx < 4; fx++ {
				left, right, up, down := margins(fx, fy)
				stride := w + left + right
				l := layout{
					srcOff: up*stride + left, srcStride: stride,
					srcLen: (up+h+down)*stride - 0,
					dstOff: 0, dstStride: w, dstLen: w * h,
				}
				l.srcLen = l.srcOff + (h-1+down)*stride + w + right
				src := make([]byte, l.srcLen)
				fill(r, src, fillRandom, stride)
				dst := make([]byte, l.dstLen)
				fill(r, dst, fillRandom, 1)
				check(t, src, dst, l, w, h, fx, fy)

				mustPanic(t, "short source", func() {
					PutLuma(dst, 0, w, src[:len(src)-1], l.srcOff, stride, w, h, fx, fy)
				})
				mustPanic(t, "source offset one too small", func() {
					PutLuma(dst, 0, w, src, l.srcOff-1, stride, w, h, fx, fy)
				})
			}
		}
	}
}

func mustPanic(t *testing.T, what string, f func()) {
	t.Helper()
	defer func() {
		t.Helper()
		if recover() == nil {
			t.Errorf("%s: no panic", what)
		}
	}()
	f()
}

func TestBoundsRefused(t *testing.T) {
	src := make([]byte, 64*64)
	dst := make([]byte, 64*64)
	ok := func(f func(dst []byte, dstOff, dstStride int, src []byte, srcOff, srcStride, w, h, xFrac, yFrac int)) {
		f(dst, 0, 64, src, 3*64+3, 64, 8, 8, 2, 2) // the base case does not panic
		f(dst, 0, 64, src, 3893, 64, 8, 4, 1, 0)   // last index of the footprint is the last of src
	}
	for _, f := range []func(dst []byte, dstOff, dstStride int, src []byte, srcOff, srcStride, w, h, xFrac, yFrac int){PutLuma, AvgLuma} {
		ok(f)
		cases := []struct {
			name string
			call func()
		}{
			{"width 2", func() { f(dst, 0, 64, src, 200, 64, 2, 8, 0, 0) }},
			{"width 12", func() { f(dst, 0, 64, src, 200, 64, 12, 8, 0, 0) }},
			{"16x4", func() { f(dst, 0, 64, src, 200, 64, 16, 4, 0, 0) }},
			{"4x16", func() { f(dst, 0, 64, src, 200, 64, 4, 16, 0, 0) }},
			{"height 0", func() { f(dst, 0, 64, src, 200, 64, 8, 0, 0, 0) }},
			{"xFrac -1", func() { f(dst, 0, 64, src, 200, 64, 8, 8, -1, 0) }},
			{"xFrac 4", func() { f(dst, 0, 64, src, 200, 64, 8, 8, 4, 0) }},
			{"yFrac -1", func() { f(dst, 0, 64, src, 200, 64, 8, 8, 0, -1) }},
			{"yFrac 4", func() { f(dst, 0, 64, src, 200, 64, 8, 8, 0, 4) }},
			{"zero dst stride", func() { f(dst, 0, 0, src, 200, 64, 8, 8, 0, 0) }},
			{"dst stride below width", func() { f(dst, 0, 7, src, 200, 64, 8, 8, 0, 0) }},
			{"dst stride one below 16", func() { f(dst, 0, 15, src, 200, 64, 16, 8, 0, 0) }},
			{"negative src stride", func() { f(dst, 0, 64, src, 200, -64, 8, 8, 0, 0) }},
			{"negative dst offset", func() { f(dst, -1, 64, src, 200, 64, 8, 8, 0, 0) }},
			{"dst too short", func() { f(dst, 64*64-7*64, 64, src, 200, 64, 8, 8, 0, 0) }},
			{"dst offset past end", func() { f(dst, 64*64, 64, src, 200, 64, 8, 8, 0, 0) }},
			{"src above plane", func() { f(dst, 0, 64, src, 64+3, 64, 8, 8, 2, 2) }},
			{"src left of plane", func() { f(dst, 0, 64, src, 1, 64, 8, 8, 2, 0) }},
			{"src below plane", func() { f(dst, 0, 64, src, 60*64, 64, 8, 8, 0, 1) }},
			{"src right edge past end", func() { f(dst, 0, 64, src, 3894, 64, 8, 4, 1, 0) }},
			{"huge stride", func() { f(dst, 0, 64, src, 200, 1<<31-1, 8, 8, 0, 0) }},
			{"empty planes", func() { f(nil, 0, 8, nil, 0, 8, 4, 4, 0, 0) }},
		}
		for _, c := range cases {
			mustPanic(t, c.name, c.call)
		}
	}
}

// Fuzz the bounds proof and the kernels together: whatever the plane,
// strides, offsets, size and position, the call either panics because the
// footprint (computed here from Table 8-11's extreme offsets) does not fit,
// or equals the oracle exactly, with no byte outside the block touched.
func FuzzLuma(f *testing.F) {
	r := rand.New(rand.NewSource(3))
	plane := make([]byte, 40*40)
	fill(r, plane, fillRandom, 40)
	f.Add(plane, uint16(2*40+2), uint16(40), uint16(0), uint16(24), uint8(0), uint8(10), false)
	f.Add(plane, uint16(2*40+2), uint16(40), uint16(5), uint16(16), uint8(3), uint8(5), true)
	f.Add(plane, uint16(0), uint16(40), uint16(0), uint16(8), uint8(6), uint8(15), true)
	fill(r, plane, fillChecker, 40)
	f.Add(plane, uint16(3*40+3), uint16(40), uint16(1), uint16(20), uint8(1), uint8(6), false)
	f.Add(plane[:300], uint16(250), uint16(17), uint16(0), uint16(8), uint8(4), uint8(1), false)

	f.Fuzz(func(t *testing.T, src []byte, srcOff, srcStride, dstOff, dstStride uint16, sizeIdx, pos uint8, avg bool) {
		sz := sizes[int(sizeIdx)%len(sizes)]
		w, h := sz[0], sz[1]
		fx, fy := int(pos)&3, int(pos>>2)&3
		so, ss, do, ds := int(srcOff), int(srcStride), int(dstOff), int(dstStride)

		dst := make([]byte, 1024)
		for i := range dst {
			dst[i] = byte(i*7 + 3)
		}
		want := append([]byte(nil), dst...)

		left, right, up, down := margins(fx, fy)
		fits := ss > 0 && ds >= w &&
			so-up*ss-left >= 0 && so+(h-1+down)*ss+w-1+right < len(src) &&
			do+(h-1)*ds+w-1 < len(dst)

		var panicked bool
		func() {
			defer func() { panicked = recover() != nil }()
			if avg {
				AvgLuma(dst, do, ds, src, so, ss, w, h, fx, fy)
			} else {
				PutLuma(dst, do, ds, src, so, ss, w, h, fx, fy)
			}
		}()
		if panicked == fits {
			t.Fatalf("fits=%v panicked=%v: %dx%d frac (%d,%d) src %d/%d dst %d/%d len %d", fits, panicked, w, h, fx, fy, so, ss, do, ds, len(src))
		}
		if panicked {
			if !bytes.Equal(dst, want) {
				t.Fatal("a refused call wrote to dst")
			}
			return
		}
		oracle(want, do, ds, src, so, ss, w, h, fx, fy, avg)
		if !bytes.Equal(dst, want) {
			t.Fatalf("avg=%v %dx%d frac (%d,%d): first difference at %d", avg, w, h, fx, fy, firstDiff(dst, want))
		}
	})
}
