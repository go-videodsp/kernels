//go:build unix

package hevc

import (
	"math/rand"
	"runtime/debug"
	"testing"
	"unsafe"
)

// The horizontal filter into int32 and the widening copy with exactly their footprint in memory
// that ends (or starts) at a PROT_NONE page: a read or write outside the block or the taps'
// columns faults. Every width (luma multiples of four, chroma and widening even).
func TestHPassWAndWidenGuardPages(t *testing.T) {
	old := debug.SetPanicOnFault(true)
	defer debug.SetPanicOnFault(old)
	rng := rand.New(rand.NewSource(74))
	for _, atEnd := range []bool{true, false} {
		for w := 2; w <= 64; w += 2 {
			for _, h := range []int{1, 3, 8, 64} {
				for kind := 0; kind < 3; kind++ { // luma, chroma, widen
					if kind == 0 && w%4 != 0 {
						continue
					}
					bef, aft := before, after
					switch kind {
					case 1:
						bef, aft = chromaBefore, chromaAfter
					case 2:
						bef, aft = 0, 0
					}
					sS, dS := w+bef+aft+rng.Intn(3), w+rng.Intn(3)
					ns, nd := (h-1)*sS+w+bef+aft, (h-1)*dS+w
					src := unsafe.Slice((*uint16)(guarded(t, ns, 2, atEnd)), ns)
					dst := unsafe.Slice((*int32)(guarded(t, nd, 4, atEnd)), nd)
					for i := range src {
						src[i] = uint16(rng.Intn(4096))
					}
					var err error
					switch kind {
					case 0:
						err = TryHPassW8(dst, 0, dS, src, bef, sS, w, h, 12, &lumaFilters[rng.Intn(3)])
					case 1:
						err = TryHPassW4(dst, 0, dS, src, bef, sS, w, h, 12, &chromaFilters[rng.Intn(7)])
					default:
						err = TryWiden(dst, 0, dS, src, 0, sS, w, h, 2+rng.Intn(5))
					}
					if err != nil {
						t.Fatalf("kind %d %dx%d: %v", kind, w, h, err)
					}
				}
			}
		}
	}
}
