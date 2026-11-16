//go:build unix

package hevc

import (
	"math/rand"
	"runtime/debug"
	"testing"
	"unsafe"
)

// The vertical filter with exactly its footprint in memory that ends (or starts) at a
// PROT_NONE page: a read or write outside the block or its rows of taps faults. Every
// width (luma multiples of four, chroma even) in groups of eight, four and two.
func TestVPassUGuardPages(t *testing.T) {
	old := debug.SetPanicOnFault(true)
	defer debug.SetPanicOnFault(old)
	rng := rand.New(rand.NewSource(63))
	for _, atEnd := range []bool{true, false} {
		for w := 2; w <= 64; w += 2 {
			for _, h := range []int{1, 3, 8, 64} {
				for _, chroma := range []bool{false, true} {
					if !chroma && w%4 != 0 {
						continue
					}
					bef, aft := before, after
					if chroma {
						bef, aft = chromaBefore, chromaAfter
					}
					sS, dS := w+rng.Intn(3), w+rng.Intn(3)
					ns, nd := (h-1+bef+aft)*sS+w, (h-1)*dS+w
					src := unsafe.Slice((*uint16)(guarded(t, ns, 2, atEnd)), ns)
					dst := unsafe.Slice((*int32)(guarded(t, nd, 4, atEnd)), nd)
					for i := range src {
						src[i] = uint16(rng.Intn(4096))
					}
					var err error
					if chroma {
						err = TryVPassU4(dst, 0, dS, src, bef*sS, sS, w, h, 12, &chromaFilters[rng.Intn(7)])
					} else {
						err = TryVPassU8(dst, 0, dS, src, bef*sS, sS, w, h, 12, &lumaFilters[rng.Intn(3)])
					}
					if err != nil {
						t.Fatalf("chroma %v %dx%d: %v", chroma, w, h, err)
					}
				}
			}
		}
	}
}
