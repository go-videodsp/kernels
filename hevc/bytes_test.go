package hevc

import (
	"errors"
	"math/rand"
	"testing"
	"unsafe"
)

// byteKernel is one of the six filters over the bytes of an 8-bit picture: k 0 is the
// horizontal pass into int16 (HPass), 1 the vertical filter into int32 (VPassU), 2 the horizontal
// filter into int32 (HPassW); luma has eight taps, chroma four.
type byteKernel struct {
	name   string
	k      int
	chroma bool
}

var byteKernels = []byteKernel{
	{"HPass8", 0, false}, {"HPass4", 0, true},
	{"VPassU8", 1, false}, {"VPassU4", 1, true},
	{"HPassW8", 2, false}, {"HPassW4", 2, true},
}

// geometry is the taps, the footprint before and after the block, and the width unit.
func (b byteKernel) geometry() (n, bef, aft, unit int) {
	if b.chroma {
		return 4, chromaBefore, chromaAfter, 2
	}
	return 8, before, after, 4
}

func (b byteKernel) horizontal() bool { return b.k != 1 }

// srcOff is the smallest valid offset of the block in a source of stride sS.
func (b byteKernel) srcOff(sS int) int {
	_, bef, _, _ := b.geometry()
	if b.horizontal() {
		return bef
	}
	return bef * sS
}

// srcLen is the length of a source holding exactly the footprint of a w x h block.
func (b byteKernel) srcLen(sS, w, h int) int {
	_, bef, aft, _ := b.geometry()
	if b.horizontal() {
		return (h-1)*sS + w + bef + aft
	}
	return (h-1+bef+aft)*sS + w
}

type byteDst struct {
	d16 []int16
	d32 []int32
}

func (b byteKernel) newDst(n int) byteDst {
	if b.k == 0 {
		d := make([]int16, n)
		for i := range d {
			d[i] = sentinel16
		}
		return byteDst{d16: d}
	}
	d := make([]int32, n)
	for i := range d {
		d[i] = sentinel16
	}
	return byteDst{d32: d}
}

func (d byteDst) len() int {
	if d.d16 != nil {
		return len(d.d16)
	}
	return len(d.d32)
}

func (d byteDst) at(i int) int32 {
	if d.d16 != nil {
		return int32(d.d16[i])
	}
	return d.d32[i]
}

// run calls the kernel; a nil c is a nil pointer.
func (b byteKernel) run(d byteDst, dOff, dS int, src []uint8, sOff, sS, w, h int, c []int16) error {
	var p8 *[8]int16
	var p4 *[4]int16
	if c != nil && b.chroma {
		p4 = (*[4]int16)(c)
	} else if c != nil {
		p8 = (*[8]int16)(c)
	}
	switch {
	case b.k == 0 && b.chroma:
		return TryHPass4Bytes(d.d16, dOff, dS, src, sOff, sS, w, h, p4)
	case b.k == 0:
		return TryHPass8Bytes(d.d16, dOff, dS, src, sOff, sS, w, h, p8)
	case b.k == 1 && b.chroma:
		return TryVPassU4Bytes(d.d32, dOff, dS, src, sOff, sS, w, h, p4)
	case b.k == 1:
		return TryVPassU8Bytes(d.d32, dOff, dS, src, sOff, sS, w, h, p8)
	case b.chroma:
		return TryHPassW4Bytes(d.d32, dOff, dS, src, sOff, sS, w, h, p4)
	}
	return TryHPassW8Bytes(d.d32, dOff, dS, src, sOff, sS, w, h, p8)
}

// model is the text's filter of one sample in 64 bits (shift 0 at 8 bits); the int16 pass
// keeps the low 16 bits.
func (b byteKernel) model(src []uint8, pos, sS int, c []int16) int32 {
	_, bef, _, _ := b.geometry()
	step := sS
	if b.horizontal() {
		step = 1
	}
	var s int64
	for k, ck := range c {
		s += int64(ck) * int64(src[pos+(k-bef)*step])
	}
	if b.k == 0 {
		return int32(int16(s))
	}
	return int32(s)
}

func randomBytes(rng *rand.Rand, n int, extreme bool) []uint8 {
	s := make([]uint8, n)
	for i := range s {
		if extreme {
			s[i] = [2]uint8{0, 255}[rng.Intn(2)]
		} else {
			s[i] = uint8(rng.Intn(256))
		}
	}
	return s
}

func checkBytesAgainstTheText(t *testing.T, seed int64, iters int) {
	t.Helper()
	rng := rand.New(rand.NewSource(seed))
	for iter := 0; iter < iters; iter++ {
		b := byteKernels[iter%len(byteKernels)]
		n, _, _, unit := b.geometry()
		w, h := unit*(1+rng.Intn(64/unit)), 1+rng.Intn(64)
		c := randomFilter(rng, n, iter/len(byteKernels))
		sS, dS := w+rng.Intn(9), w+rng.Intn(9)
		src := randomBytes(rng, b.srcLen(sS, w, h)+sS+rng.Intn(5), iter%7 == 0)
		d := b.newDst((h-1)*dS + w + rng.Intn(5))
		sOff := b.srcOff(sS)
		if !b.horizontal() {
			sOff += rng.Intn(sS - w + 1)
		}
		if err := b.run(d, 0, dS, src, sOff, sS, w, h, c); err != nil {
			t.Fatalf("iter %d %s: %v", iter, b.name, err)
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				want := b.model(src, sOff+y*sS+x, sS, c)
				if got := d.at(y*dS + x); got != want {
					t.Fatalf("iter %d %s %dx%d: sample (%d,%d) is %d, the text gives %d", iter, b.name, w, h, x, y, got, want)
				}
			}
		}
		for i := 0; i < d.len(); i++ {
			if y, x := i/dS, i%dS; (x >= w || y >= h) && d.at(i) != sentinel16 {
				t.Fatalf("iter %d %s: a sample outside the block was written", iter, b.name)
			}
		}
	}
}

func TestBytesAgainstTheText(t *testing.T) {
	checkBytesAgainstTheText(t, 71, 9000)
}

func TestBytesRefusals(t *testing.T) {
	for _, b := range byteKernels {
		n, _, _, unit := b.geometry()
		c := randomFilter(rand.New(rand.NewSource(5)), n, 0)
		src := make([]uint8, 8192)
		ok := b.srcOff(64)
		type tc struct {
			name                           string
			dOff, dS, sOff, sS, w, h, dLen int
			nilC                           bool
		}
		bad := 2*unit + 1 // not a multiple of the unit (odd for chroma, 9 for luma)
		if !b.chroma {
			bad = 6
		}
		for _, x := range []tc{
			{"width not a multiple of the unit", 0, 64, ok, 64, bad, 4, 4096, false},
			{"width 0", 0, 64, ok, 64, 0, 4, 4096, false},
			{"width past 64", 0, 128, ok, 128, 68, 4, 8192, false},
			{"height 0", 0, 64, ok, 64, 8, 0, 4096, false},
			{"height 65", 0, 64, ok, 64, 8, 65, 8192, false},
			{"no coefficients", 0, 64, ok, 64, 8, 4, 4096, true},
			{"destination stride", 0, 4, ok, 64, 8, 4, 4096, false},
			{"source stride 0", 0, 64, ok, 0, 8, 4, 4096, false},
			{"destination fit", 4090, 64, ok, 64, 8, 4, 4096, false},
			{"source fit before", 0, 64, ok - 1, 64, 8, 4, 4096, false},
			{"source fit after", 0, 64, 8192 - 100, 64, 8, 4, 4096, false},
			{"stride that wraps", 0, 64, ok, maxInt, 8, 4, 4096, false},
		} {
			d := b.newDst(x.dLen)
			cc := c
			if x.nilC {
				cc = nil
			}
			if err := b.run(d, x.dOff, x.dS, src, x.sOff, x.sS, x.w, x.h, cc); !errors.Is(err, ErrRefused) {
				t.Errorf("%s, %s: %v", b.name, x.name, err)
			}
			for i := 0; i < d.len(); i++ {
				if d.at(i) != sentinel16 {
					t.Fatalf("%s, %s: a refused call wrote dst[%d]", b.name, x.name, i)
				}
			}
		}
		// the destination sharing memory with the source footprint
		mem := make([]int32, 4096)
		var err error
		switch b.k {
		case 0:
			d16 := unsafe.Slice((*int16)(unsafe.Pointer(&mem[0])), 2*len(mem))
			s8 := unsafe.Slice((*uint8)(unsafe.Pointer(&mem[0])), 4*len(mem))
			err = b.run(byteDst{d16: d16}, 0, 64, s8, ok, 64, 8, 4, c)
		default:
			s8 := unsafe.Slice((*uint8)(unsafe.Pointer(&mem[0])), 4*len(mem))
			err = b.run(byteDst{d32: mem}, 0, 64, s8, ok, 64, 8, 4, c)
		}
		if !errors.Is(err, ErrRefused) {
			t.Errorf("%s overlap: %v", b.name, err)
		}
	}
}

func TestWidenBytes(t *testing.T) {
	rng := rand.New(rand.NewSource(72))
	for iter := 0; iter < 3000; iter++ {
		w, h, shift := 2*(1+rng.Intn(32)), 1+rng.Intn(64), rng.Intn(16)
		sS, dS := w+rng.Intn(9), w+rng.Intn(9)
		src := randomBytes(rng, (h-1)*sS+w+rng.Intn(5), iter%7 == 0)
		dst := make([]int32, (h-1)*dS+w+rng.Intn(5))
		for i := range dst {
			dst[i] = sentinel16
		}
		if err := TryWidenBytes(dst, 0, dS, src, 0, sS, w, h, shift); err != nil {
			t.Fatalf("iter %d: %v", iter, err)
		}
		for i := range dst {
			y, x := i/dS, i%dS
			want := int32(sentinel16)
			if x < w && y < h {
				want = int32(src[y*sS+x]) << uint(shift)
			}
			if dst[i] != want {
				t.Fatalf("iter %d %dx%d shift %d: element %d is %d, want %d", iter, w, h, shift, i, dst[i], want)
			}
		}
	}
	dst := make([]int32, 4096)
	src := make([]uint8, 4096)
	mem := make([]int32, 4096)
	for name, f := range map[string]func() error{
		"odd width":          func() error { return TryWidenBytes(dst, 0, 64, src, 0, 64, 5, 4, 0) },
		"width 0":            func() error { return TryWidenBytes(dst, 0, 64, src, 0, 64, 0, 4, 0) },
		"width past 64":      func() error { return TryWidenBytes(dst, 0, 128, src, 0, 128, 66, 4, 0) },
		"height 0":           func() error { return TryWidenBytes(dst, 0, 64, src, 0, 64, 8, 0, 0) },
		"height 65":          func() error { return TryWidenBytes(dst, 0, 64, src, 0, 64, 8, 65, 0) },
		"shift 16":           func() error { return TryWidenBytes(dst, 0, 64, src, 0, 64, 8, 4, 16) },
		"shift -1":           func() error { return TryWidenBytes(dst, 0, 64, src, 0, 64, 8, 4, -1) },
		"destination stride": func() error { return TryWidenBytes(dst, 0, 4, src, 0, 64, 8, 4, 0) },
		"source stride":      func() error { return TryWidenBytes(dst, 0, 64, src, 0, 4, 8, 4, 0) },
		"destination fit":    func() error { return TryWidenBytes(dst, 4090, 64, src, 0, 64, 8, 4, 0) },
		"source fit":         func() error { return TryWidenBytes(dst, 0, 64, src, 4090, 64, 8, 4, 0) },
		"stride that wraps":  func() error { return TryWidenBytes(dst, 0, 64, src, 64, maxInt, 8, 4, 0) },
		"overlap": func() error {
			return TryWidenBytes(mem, 0, 64, unsafe.Slice((*uint8)(unsafe.Pointer(&mem[0])), 4*len(mem)), 0, 64, 8, 4, 0)
		},
	} {
		for i := range dst {
			dst[i] = sentinel16
		}
		if err := f(); !errors.Is(err, ErrRefused) {
			t.Errorf("%s: %v", name, err)
		}
		for i := range dst {
			if dst[i] != sentinel16 {
				t.Fatalf("%s: a refused call wrote dst[%d]", name, i)
			}
		}
	}
}

// The portable references are the text's formulas over bytes too: they are what a target
// without a kernel runs.
func TestBytesRefsAgainstTheText(t *testing.T) {
	rng := rand.New(rand.NewSource(73))
	for iter := 0; iter < 400; iter++ {
		w, h := 4*(1+rng.Intn(4)), 1+rng.Intn(6)
		c8, c4 := randomFilter(rng, 8, iter), randomFilter(rng, 4, iter)
		src := randomBytes(rng, (h+8)*(w+8), iter%5 == 0)
		sS := w + 8
		g16, g32 := make([]int16, w*h), make([]int32, w*h)
		check := func(b byteKernel, off int, c []int16, get func(i int) int32) {
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					if want := b.model(src, off+y*sS+x, sS, c); get(y*w+x) != want {
						t.Fatalf("%s: (%d,%d) is %d, the text gives %d", b.name, x, y, get(y*w+x), want)
					}
				}
			}
		}
		g := func(i int) int32 { return g32[i] }
		g1 := func(i int) int32 { return int32(g16[i]) }
		hpassRef(g16, 0, w, src, 3+3*sS, sS, w, h, 0, (*[8]int16)(c8))
		check(byteKernels[0], 3+3*sS, c8, g1)
		hpass4Ref(g16, 0, w, src, 1+sS, sS, w, h, 0, (*[4]int16)(c4))
		check(byteKernels[1], 1+sS, c4, g1)
		vpassU8Ref(g32, 0, w, src, 3*sS+3, sS, w, h, 0, (*[8]int16)(c8))
		check(byteKernels[2], 3*sS+3, c8, g)
		vpassU4Ref(g32, 0, w, src, sS+1, sS, w, h, 0, (*[4]int16)(c4))
		check(byteKernels[3], sS+1, c4, g)
		hpassW8Ref(g32, 0, w, src, 3+3*sS, sS, w, h, 0, (*[8]int16)(c8))
		check(byteKernels[4], 3+3*sS, c8, g)
		hpassW4Ref(g32, 0, w, src, 1+sS, sS, w, h, 0, (*[4]int16)(c4))
		check(byteKernels[5], 1+sS, c4, g)
		widenRef(g32, 0, w, src, sS, sS, w, h, 3)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if want := int32(src[sS+y*sS+x]) << 3; g32[y*w+x] != want {
					t.Fatalf("widen: (%d,%d) is %d, want %d", x, y, g32[y*w+x], want)
				}
			}
		}
	}
}
