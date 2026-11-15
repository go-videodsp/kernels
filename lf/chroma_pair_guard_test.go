//go:build unix

package lf

import (
	"bytes"
	"fmt"
	"math/rand"
	"runtime/debug"
	"testing"
)

// Both planes exactly their footprint, each against an inaccessible page.
func TestChromaPairGuardPages(t *testing.T) {
	old := debug.SetPanicOnFault(true)
	defer debug.SetPanicOnFault(old)
	r := rand.New(rand.NewSource(19))
	for _, atEnd := range []bool{true, false} {
		for _, vertical := range []bool{true, false} {
			for _, strong := range []bool{false, true} {
				for _, stride := range []int{8, 13, 64} {
					t.Run(fmt.Sprintf("end=%v/vertical=%v/strong=%v/stride=%d", atEnd, vertical, strong, stride), func(t *testing.T) {
						var off, n int
						if vertical {
							off, n = 2, 7*stride+4
						} else {
							off, n = 2*stride, 3*stride+8
						}
						for round := 0; round < 30; round++ {
							var init [2][]byte
							var pl [2][]byte
							for p := range pl {
								init[p] = make([]byte, n)
								base := r.Intn(256)
								for i := range init[p] {
									init[p][i] = byte(clip3(0, 255, base+r.Intn(9)-4+(i%stride)/4*r.Intn(3)))
								}
								pl[p] = guarded(t, n, atEnd)
								copy(pl[p], init[p])
							}
							a, b := 20+r.Intn(32), 14+r.Intn(38)
							bS := [4]uint8{uint8(r.Intn(4)), uint8(r.Intn(4)), uint8(r.Intn(4)), uint8(r.Intn(4))}
							bsOracle := [4]int{int(bS[0]), int(bS[1]), int(bS[2]), int(bS[3])}
							if strong {
								bsOracle = [4]int{4, 4, 4, 4}
							}
							for p := range init {
								oracleChromaEdge(init[p], off, stride, vertical, a, b, bsOracle)
							}
							switch {
							case strong && vertical:
								DeblockChromaPairIntraV(pl[0], pl[1], off, stride, a, b)
							case strong:
								DeblockChromaPairIntraH(pl[0], pl[1], off, stride, a, b)
							case vertical:
								DeblockChromaPairV(pl[0], pl[1], off, stride, a, b, bS)
							default:
								DeblockChromaPairH(pl[0], pl[1], off, stride, a, b, bS)
							}
							for p := range pl {
								if !bytes.Equal(pl[p], init[p]) {
									t.Fatalf("round %d plane %d: first difference at %d", round, p, firstDiff(pl[p], init[p]))
								}
							}
						}
					})
				}
			}
		}
	}
}
