//go:build unix

package lf

import (
	"bytes"
	"fmt"
	"math/rand"
	"runtime/debug"
	"testing"
)

// Every chroma kernel on a plane that is exactly its footprint, end-aligned
// and start-aligned against an inaccessible page: any access outside columns
// -2..1 (or rows -2..1) of the 8 lines faults.
func TestChromaGuardPages(t *testing.T) {
	old := debug.SetPanicOnFault(true)
	defer debug.SetPanicOnFault(old)
	r := rand.New(rand.NewSource(9))
	for _, atEnd := range []bool{true, false} {
		for _, vertical := range []bool{true, false} {
			for _, strong := range []bool{false, true} {
				for _, stride := range []int{8, 13, 64} {
					name := fmt.Sprintf("end=%v/vertical=%v/strong=%v/stride=%d", atEnd, vertical, strong, stride)
					t.Run(name, func(t *testing.T) {
						var off, n int
						if vertical {
							off, n = 2, 7*stride+4
						} else {
							off, n = 2*stride, 3*stride+8
						}
						for round := 0; round < 40; round++ {
							init := make([]byte, n)
							base := r.Intn(256)
							for i := range init {
								init[i] = byte(clip3(0, 255, base+r.Intn(9)-4+(i%stride)/4*r.Intn(3)))
							}
							pl := guarded(t, n, atEnd)
							copy(pl, init)
							a, b := 20+r.Intn(32), 14+r.Intn(38)
							bS := [4]uint8{uint8(r.Intn(4)), uint8(r.Intn(4)), uint8(r.Intn(4)), uint8(r.Intn(4))}
							bsOracle := [4]int{int(bS[0]), int(bS[1]), int(bS[2]), int(bS[3])}
							if strong {
								bsOracle = [4]int{4, 4, 4, 4}
							}
							want := append([]byte(nil), init...)
							oracleChromaEdge(want, off, stride, vertical, a, b, bsOracle)
							switch {
							case strong && vertical:
								DeblockChromaIntraV(pl, off, stride, a, b)
							case strong:
								DeblockChromaIntraH(pl, off, stride, a, b)
							case vertical:
								DeblockChromaV(pl, off, stride, a, b, bS)
							default:
								DeblockChromaH(pl, off, stride, a, b, bS)
							}
							if !bytes.Equal(pl, want) {
								t.Fatalf("round %d indexA %d indexB %d bS %v: first difference at %d", round, a, b, bS, firstDiff(pl, want))
							}
						}
					})
				}
			}
		}
	}
}
