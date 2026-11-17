//go:build unix

package hevc

import (
	"math/rand"
	"runtime/debug"
	"testing"
	"unsafe"
)

// The byte-source kernels with exactly their footprint in memory that ends (or starts) at a
// PROT_NONE page: a read or write outside the block or its taps faults. Every width (luma
// multiples of four, chroma even) in groups of eight, four and two, so the two-byte and
// four-byte loads of the tails are held to the footprint too.
func TestBytesGuardPages(t *testing.T) {
	old := debug.SetPanicOnFault(true)
	defer debug.SetPanicOnFault(old)
	rng := rand.New(rand.NewSource(74))
	for _, atEnd := range []bool{true, false} {
		for _, b := range byteKernels {
			n, _, _, unit := b.geometry()
			for w := unit; w <= 64; w += unit {
				for _, h := range []int{1, 3, 8, 64} {
					sS, dS := w+rng.Intn(3), w+rng.Intn(3)
					ns, nd := b.srcLen(sS, w, h), (h-1)*dS+w
					src := unsafe.Slice((*uint8)(guarded(t, ns, 1, atEnd)), ns)
					for i := range src {
						src[i] = uint8(rng.Intn(256))
					}
					var d byteDst
					if b.k == 0 {
						d.d16 = unsafe.Slice((*int16)(guarded(t, nd, 2, atEnd)), nd)
					} else {
						d.d32 = unsafe.Slice((*int32)(guarded(t, nd, 4, atEnd)), nd)
					}
					if err := b.run(d, 0, dS, src, b.srcOff(sS), sS, w, h, randomFilter(rng, n, w)); err != nil {
						t.Fatalf("%s %dx%d: %v", b.name, w, h, err)
					}
				}
			}
		}
		for w := 2; w <= 64; w += 2 {
			for _, h := range []int{1, 3, 64} {
				sS, dS := w+rng.Intn(3), w+rng.Intn(3)
				ns, nd := (h-1)*sS+w, (h-1)*dS+w
				src := unsafe.Slice((*uint8)(guarded(t, ns, 1, atEnd)), ns)
				dst := unsafe.Slice((*int32)(guarded(t, nd, 4, atEnd)), nd)
				if err := TryWidenBytes(dst, 0, dS, src, 0, sS, w, h, rng.Intn(16)); err != nil {
					t.Fatalf("widen %dx%d: %v", w, h, err)
				}
			}
		}
	}
}
