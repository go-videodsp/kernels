//go:build unix

package hevc

import (
	"math/rand"
	"runtime/debug"
	"testing"
	"unsafe"
)

// The byte-destination kernels with exactly their footprint in memory that ends (or starts) at a
// PROT_NONE page: a read or write outside the block, or outside the neighbours an edge class
// names, faults. Every width in the groups of 32, 16, 8, 4 and 2 (SAO: the groups of 8 and 16,
// the rest of a row in Go).
func TestBytesDstGuardPages(t *testing.T) {
	old := debug.SetPanicOnFault(true)
	defer debug.SetPanicOnFault(old)
	rng := rand.New(rand.NewSource(85))
	u8 := func(n int, atEnd bool) []uint8 {
		s := unsafe.Slice((*uint8)(guarded(t, n, 1, atEnd)), n)
		for i := range s {
			s[i] = uint8(rng.Intn(256))
		}
		return s
	}
	i32 := func(n int, atEnd bool) []int32 {
		s := unsafe.Slice((*int32)(guarded(t, n, 4, atEnd)), n)
		for i := range s {
			s[i] = int32(rng.Intn(1<<17) - 1<<16)
		}
		return s
	}
	off := [4]int16{3, -2, 1, -4}
	for _, atEnd := range []bool{true, false} {
		for _, h := range []int{1, 3, 64} {
			for w := 2; w <= 64; w += 2 {
				aS, bS, dS := w+rng.Intn(3), w+rng.Intn(3), w+rng.Intn(3)
				na, nb, nd := (h-1)*aS+w, (h-1)*bS+w, (h-1)*dS+w
				if err := TryAverageBytes(u8(nd, atEnd), 0, dS, u8(na, atEnd), 0, aS, u8(nb, atEnd), 0, bS, w, h); err != nil {
					t.Fatalf("average %dx%d: %v", w, h, err)
				}
				pS := w + rng.Intn(3)
				np := (h-1)*pS + w
				log2Wd := minLog2Wd + rng.Intn(maxLog2Wd-minLog2Wd+1)
				if err := TryWeightUniBytes(u8(nd, atEnd), 0, dS, i32(np, atEnd), 0, pS, w, h, log2Wd, 3, 5); err != nil {
					t.Fatalf("weight uni %dx%d: %v", w, h, err)
				}
				if err := TryWeightBiBytes(u8(nd, atEnd), 0, dS, i32(np, atEnd), i32(np, atEnd), 0, pS, w, h, log2Wd, 3, 2, 5, 1); err != nil {
					t.Fatalf("weight bi %dx%d: %v", w, h, err)
				}
			}
			for w := 1; w <= 64; w++ {
				sS, dS := w+rng.Intn(3), w+rng.Intn(3)
				nd := (h-1)*dS + w
				ns := (h-1)*sS + w
				if err := TrySAOBandBytes(u8(nd, atEnd), 0, dS, u8(ns, atEnd), 0, sS, w, h, rng.Intn(32), &off); err != nil {
					t.Fatalf("band %dx%d: %v", w, h, err)
				}
				for class := 0; class < 4; class++ {
					rows, cols := saoMargins(class)
					sS := w + 2*cols + rng.Intn(3)
					ns := (h-1+2*rows)*sS + w + 2*cols
					if err := TrySAOEdgeBytes(u8(nd, atEnd), 0, dS, u8(ns, atEnd), rows*sS+cols, sS, w, h, class, &off); err != nil {
						t.Fatalf("edge class %d %dx%d: %v", class, w, h, err)
					}
				}
			}
		}
	}
}
