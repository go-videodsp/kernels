//go:build unix

package hevc

import (
	"math/rand"
	"runtime/debug"
	"testing"
	"unsafe"
)

// The chroma passes with exactly their footprint in memory that ends (or starts) at a
// PROT_NONE page: a read or write outside the footprint or the block faults. Every even width,
// including the ones that end in a group of two.
func TestChromaPassesGuardPages(t *testing.T) {
	old := debug.SetPanicOnFault(true)
	defer debug.SetPanicOnFault(old)
	rng := rand.New(rand.NewSource(9))
	c := chromaFilters[2]
	for _, atEnd := range []bool{true, false} {
		for w := 2; w <= 64; w += 2 {
			for _, h := range []int{1, 3, 8, 64} {
				dStride, sStride := w+rng.Intn(3), w+3+rng.Intn(3)
				ns, nd := (h-1)*sStride+w+3, (h-1)*dStride+w
				src := unsafe.Slice((*uint16)(guarded(t, ns, 2, atEnd)), ns)
				dst := unsafe.Slice((*int16)(guarded(t, nd, 2, atEnd)), nd)
				for i := range src {
					src[i] = uint16(rng.Intn(4096))
				}
				if err := TryHPass4(dst, 0, dStride, src, 1, sStride, w, h, 12, &c); err != nil {
					t.Fatalf("horizontal %dx%d: %v", w, h, err)
				}
				sStride = w + rng.Intn(3)
				ns, nd = (h+3-1)*sStride+w, (h-1)*dStride+w
				mid := unsafe.Slice((*int16)(guarded(t, ns, 2, atEnd)), ns)
				out := unsafe.Slice((*int32)(guarded(t, nd, 4, atEnd)), nd)
				for i := range mid {
					mid[i] = int16(rng.Intn(20000))
				}
				if err := TryVPass4(out, 0, dStride, mid, sStride, sStride, w, h, &c); err != nil {
					t.Fatalf("vertical %dx%d: %v", w, h, err)
				}
			}
		}
	}
}
