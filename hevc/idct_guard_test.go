//go:build unix

package hevc

import (
	"math/rand"
	"runtime/debug"
	"testing"
	"unsafe"
)

// The transform kernels with the coefficients and the residual samples exactly their size in
// memory that ends (or starts) at a PROT_NONE page: a read or write outside the block faults.
// Every size, the DCT and the DST, blocks with one coefficient at every extent.
func TestIDCTGuardPages(t *testing.T) {
	old := debug.SetPanicOnFault(true)
	defer debug.SetPanicOnFault(old)
	rng := rand.New(rand.NewSource(54))
	for _, atEnd := range []bool{true, false} {
		for log2 := 2; log2 <= 5; log2++ {
			n := 1 << uint(log2)
			for iter := 0; iter < 64; iter++ {
				d := unsafe.Slice((*int32)(guarded(t, n*n, 4, atEnd)), n*n)
				dst := unsafe.Slice((*int32)(guarded(t, n*n, 4, atEnd)), n*n)
				copy(d, randomCoefficients(rng, n, iter%5))
				if err := TryIDCT(dst, d, log2, 8+rng.Intn(5)); err != nil {
					t.Fatalf("%dx%d: %v", n, n, err)
				}
				if n == 4 {
					if err := TryIDST4(dst, d, 8+rng.Intn(5)); err != nil {
						t.Fatalf("dst: %v", err)
					}
				}
			}
		}
	}
}
