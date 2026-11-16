//go:build unix

package hevc

import (
	"math/rand"
	"runtime/debug"
	"testing"
	"unsafe"
)

// The weighting kernels with exactly their footprint in memory that ends (or starts) at a
// PROT_NONE page: a read or write outside the block faults. Every even width up to 64
// (groups of eight, four and two), one and two lists.
func TestWeightGuardPages(t *testing.T) {
	old := debug.SetPanicOnFault(true)
	defer debug.SetPanicOnFault(old)
	rng := rand.New(rand.NewSource(32))
	for _, atEnd := range []bool{true, false} {
		for w := 2; w <= 64; w += 2 {
			for _, h := range []int{1, 3, 8, 64} {
				pStride, dStride := w+rng.Intn(3), w+rng.Intn(3)
				np, nd := (h-1)*pStride+w, (h-1)*dStride+w
				p0 := unsafe.Slice((*int32)(guarded(t, np, 4, atEnd)), np)
				p1 := unsafe.Slice((*int32)(guarded(t, np, 4, atEnd)), np)
				dst := unsafe.Slice((*uint16)(guarded(t, nd, 2, atEnd)), nd)
				for i := range p0 {
					p0[i], p1[i] = int32(rng.Intn(30000)), int32(rng.Intn(30000))
				}
				if err := TryWeightUni(dst, 0, dStride, p0, 0, pStride, w, h, 12, 2+rng.Intn(12), 7, -5); err != nil {
					t.Fatalf("uni %dx%d: %v", w, h, err)
				}
				if err := TryWeightBi(dst, 0, dStride, p0, p1, 0, pStride, w, h, 12, 2+rng.Intn(12), 7, 9, -5, 3); err != nil {
					t.Fatalf("bi %dx%d: %v", w, h, err)
				}
			}
		}
	}
}
