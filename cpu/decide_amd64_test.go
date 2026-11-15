//go:build amd64 && !purego

package cpu

import "testing"

func TestDecide(t *testing.T) {
	const avx2 = 1 << 5
	for _, c := range []struct {
		name                  string
		maxLeaf, c1, b7, xcr0 uint32
		wantAVX2, wantAVX512  bool
	}{
		{"avx2 host", 7, osxsaveBit | avxBit, avx2, 0x6, true, false},
		{"avx512 host", 7, osxsaveBit | avxBit, avx2 | avx512Bits, 0xe6, true, true},
		{"OSXSAVE clear (xcr0 not readable)", 7, avxBit, avx2, 0x6, false, false},
		{"OSXSAVE clear, stale xcr0", 7, avxBit, avx2 | avx512Bits, 0xe6, false, false},
		{"AVX bit clear", 7, osxsaveBit, avx2, 0x6, false, false},
		{"OS does not save YMM", 7, osxsaveBit | avxBit, avx2, 0x2, false, false},
		{"OS does not save ZMM", 7, osxsaveBit | avxBit, avx2 | avx512Bits, 0x6, true, false},
		{"no leaf 7", 6, osxsaveBit | avxBit, avx2 | avx512Bits, 0xe6, false, false},
		{"no AVX2 bit", 7, osxsaveBit | avxBit, 0, 0x6, false, false},
	} {
		a, b := decide(c.maxLeaf, c.c1, c.b7, c.xcr0)
		if a != c.wantAVX2 || b != c.wantAVX512 {
			t.Errorf("%s: got (%v, %v), want (%v, %v)", c.name, a, b, c.wantAVX2, c.wantAVX512)
		}
	}
}
