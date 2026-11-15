package itx

import (
	"math/rand"
	"testing"
)

// A block of realistic content: a few low-frequency coefficients (the common
// case) and a dense one.
func benchBlock(b *testing.B, k blockKind, f func(dst []byte, off, stride int, coef, scale *[64]int32, qpDiv int)) {
	r := rand.New(rand.NewSource(5))
	coef, scale := randomBlock(r, k)
	plane := randomPlane(r, 8*64)
	b.SetBytes(64)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f(plane, 0, 64, &coef, &scale, 4)
	}
}

func BenchmarkIDCT8(b *testing.B) {
	for _, c := range []struct {
		name string
		k    blockKind
	}{{"dc", kindDC}, {"sparse", kindSparse}, {"dense", kindDense}} {
		b.Run(c.name+"/ref", func(b *testing.B) { benchBlock(b, c.k, dequant8x8IDCTAddRef) })
		b.Run(c.name+"/kernel", func(b *testing.B) { benchBlock(b, c.k, Dequant8x8IDCTAdd) })
	}
}
