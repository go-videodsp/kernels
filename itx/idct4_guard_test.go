//go:build unix

package itx

import (
	"bytes"
	"fmt"
	"math/rand"
	"runtime/debug"
	"testing"
)

// The region with its footprint placed so that it ends (or starts) exactly at
// a PROT_NONE page: a read or write outside the selected blocks' region
// faults. The mask has holes, so the blocks not selected are shown not to be
// touched in a way that matters (their bytes stay as they were).
func TestBlocksGuardPages(t *testing.T) {
	old := debug.SetPanicOnFault(true)
	defer debug.SetPanicOnFault(old)
	r := rand.New(rand.NewSource(31))
	for _, atEnd := range []bool{true, false} {
		for _, n := range []int{4, 16} {
			size := 16
			if n == 4 {
				size = 8
			}
			for _, stride := range []int{size, size + 3, 64} {
				t.Run(fmt.Sprintf("end=%v/n=%d/stride=%d", atEnd, n, stride), func(t *testing.T) {
					plane := (size-1)*stride + size
					for k := blockKind(0); k < numKinds; k++ {
						for _, qpDiv := range []int{0, 3, 4, 8, 31} {
							for _, mask := range []uint16{0xffff, 0x8001, 0x5555, 0x0008} {
								coef, scale := randomBlocks(r, k)
								init := randomPlane(r, plane)
								d := guarded(t, plane, atEnd)
								copy(d, init)
								Dequant4x4IDCTAddBlocks(d, 0, stride, n, &coef, mask, &scale, qpDiv, mask&1 != 0)
								want := append([]byte(nil), init...)
								dequant4x4IDCTAddBlocksRef(want, 0, stride, &coef, mask&(1<<uint(n)-1), &scale, qpDiv, mask&1 != 0)
								if !bytes.Equal(d, want) {
									t.Fatalf("kind %d qpDiv %d mask %#x: first difference at %d", k, qpDiv, mask, firstDiff(d, want))
								}
							}
						}
					}
				})
			}
		}
	}
}
