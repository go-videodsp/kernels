//go:build unix

package hevc

import (
	"math/rand"
	"runtime/debug"
	"testing"
	"unsafe"
)

// The SAO kernels with exactly their footprint in memory that ends (or starts) at a
// PROT_NONE page: a read or write outside the footprint or the block faults. Every width up
// to 64 (the groups of sixteen, eight, and the tail in Go) and every class.
func TestSAOGuardPages(t *testing.T) {
	old := debug.SetPanicOnFault(true)
	defer debug.SetPanicOnFault(old)
	rng := rand.New(rand.NewSource(23))
	off := [4]int16{-3, -1, 2, 5}
	for _, atEnd := range []bool{true, false} {
		for w := 1; w <= 64; w++ {
			for _, h := range []int{1, 2, 7, 64} {
				dStride := w + rng.Intn(3)
				nd := (h-1)*dStride + w
				// band: the footprint is the block
				sStride := w + rng.Intn(3)
				ns := (h-1)*sStride + w
				src := unsafe.Slice((*uint16)(guarded(t, ns, 2, atEnd)), ns)
				dst := unsafe.Slice((*uint16)(guarded(t, nd, 2, atEnd)), nd)
				for i := range src {
					src[i] = uint16(rng.Intn(4096))
				}
				if err := TrySAOBand(dst, 0, dStride, src, 0, sStride, w, h, 12, rng.Intn(32), &off); err != nil {
					t.Fatalf("band %dx%d: %v", w, h, err)
				}
				// edge: the footprint is the block and the margin of the class
				for class := 0; class < 4; class++ {
					rows, cols := saoMargins(class)
					es := w + 2*cols + rng.Intn(3)
					en := (h-1+2*rows)*es + w + 2*cols
					esrc := unsafe.Slice((*uint16)(guarded(t, en, 2, atEnd)), en)
					edst := unsafe.Slice((*uint16)(guarded(t, nd, 2, atEnd)), nd)
					for i := range esrc {
						esrc[i] = uint16(rng.Intn(4096))
					}
					if err := TrySAOEdge(edst, 0, dStride, esrc, rows*es+cols, es, w, h, 12, class, &off); err != nil {
						t.Fatalf("edge class %d %dx%d: %v", class, w, h, err)
					}
				}
			}
		}
	}
}
