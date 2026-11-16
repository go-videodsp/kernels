//go:build unix

package hevc

import (
	"math/rand"
	"runtime/debug"
	"testing"
	"unsafe"
)

// The averaging kernel with exactly its footprint in memory that ends (or starts) at a
// PROT_NONE page: a read or write outside the block faults. Every even width up to 64
// (groups of sixteen, eight, four and two).
func TestAverageGuardPages(t *testing.T) {
	old := debug.SetPanicOnFault(true)
	defer debug.SetPanicOnFault(old)
	rng := rand.New(rand.NewSource(42))
	for _, atEnd := range []bool{true, false} {
		for w := 2; w <= 64; w += 2 {
			for _, h := range []int{1, 3, 8, 64} {
				aS, bS, dS := w+rng.Intn(3), w+rng.Intn(3), w+rng.Intn(3)
				na, nb, nd := (h-1)*aS+w, (h-1)*bS+w, (h-1)*dS+w
				a := unsafe.Slice((*uint16)(guarded(t, na, 2, atEnd)), na)
				b := unsafe.Slice((*uint16)(guarded(t, nb, 2, atEnd)), nb)
				dst := unsafe.Slice((*uint16)(guarded(t, nd, 2, atEnd)), nd)
				for i := range a {
					a[i] = uint16(rng.Intn(65536))
				}
				for i := range b {
					b[i] = uint16(rng.Intn(65536))
				}
				if err := TryAverage(dst, 0, dS, a, 0, aS, b, 0, bS, w, h); err != nil {
					t.Fatalf("%dx%d: %v", w, h, err)
				}
			}
		}
	}
}
