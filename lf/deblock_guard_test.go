//go:build unix

package lf

import (
	"bytes"
	"fmt"
	"math/rand"
	"runtime/debug"
	"syscall"
	"testing"
	"unsafe"
)

// guarded returns a slice of n bytes that ends at an inaccessible page
// (atEnd) or starts right after one, so that a byte read or written outside
// the footprint faults.
func guarded(t *testing.T, n int, atEnd bool) []byte {
	t.Helper()
	pg := syscall.Getpagesize()
	body := (n + pg - 1) / pg * pg
	m, err := syscall.Mmap(-1, 0, body+2*pg, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_ANON|syscall.MAP_PRIVATE)
	if err != nil {
		t.Fatalf("mmap: %v", err)
	}
	t.Cleanup(func() { _ = syscall.Munmap(m) })
	for _, g := range [][]byte{m[:pg], m[pg+body:]} {
		if err := syscall.Mprotect(g, syscall.PROT_NONE); err != nil {
			t.Fatalf("mprotect: %v", err)
		}
	}
	if atEnd {
		return m[pg+body-n : pg+body : pg+body]
	}
	return m[pg : pg+n : pg+n]
}

var guardSink byte

func TestGuardFaults(t *testing.T) {
	old := debug.SetPanicOnFault(true)
	defer debug.SetPanicOnFault(old)
	for _, atEnd := range []bool{true, false} {
		b := guarded(t, 100, atEnd)
		var faulted bool
		func() {
			defer func() { faulted = recover() != nil }()
			off := len(b)
			if !atEnd {
				off = -1
			}
			guardSink = *(*byte)(unsafe.Add(unsafe.Pointer(&b[0]), off))
		}()
		if !faulted {
			t.Errorf("atEnd=%v: no fault past the slice", atEnd)
		}
	}
}

// Every kernel on a plane that is exactly its footprint, end-aligned and
// start-aligned against an inaccessible page: any access outside rows -4..3
// (or columns -4..3) of the 16 lines faults.
func TestGuardPages(t *testing.T) {
	old := debug.SetPanicOnFault(true)
	defer debug.SetPanicOnFault(old)
	r := rand.New(rand.NewSource(5))
	for _, atEnd := range []bool{true, false} {
		for _, vertical := range []bool{true, false} {
			for _, strong := range []bool{false, true} {
				for _, stride := range []int{16, 23, 64} {
					name := fmt.Sprintf("end=%v/vertical=%v/strong=%v/stride=%d", atEnd, vertical, strong, stride)
					t.Run(name, func(t *testing.T) {
						var off, n int
						if vertical {
							off, n = 4, 15*stride+8
						} else {
							off, n = 4*stride, 7*stride+16
						}
						for round := 0; round < 40; round++ {
							init := make([]byte, n)
							// Samples close together so that the filters fire.
							base := r.Intn(256)
							for i := range init {
								init[i] = byte(clip3(0, 255, base+r.Intn(9)-4+(i%stride)/8*r.Intn(3)))
							}
							pl := guarded(t, n, atEnd)
							copy(pl, init)
							a, b := 20+r.Intn(32), 14+r.Intn(38)
							bS := [4]uint8{uint8(r.Intn(4)), uint8(r.Intn(4)), uint8(r.Intn(4)), uint8(r.Intn(4))}
							want := append([]byte(nil), init...)
							bsOracle := [4]int{int(bS[0]), int(bS[1]), int(bS[2]), int(bS[3])}
							if strong {
								bsOracle = [4]int{4, 4, 4, 4}
							}
							oracleEdge(want, off, stride, vertical, a, b, bsOracle)
							switch {
							case strong && vertical:
								DeblockLumaIntraV(pl, off, stride, a, b)
							case strong:
								DeblockLumaIntraH(pl, off, stride, a, b)
							case vertical:
								DeblockLumaV(pl, off, stride, a, b, bS)
							default:
								DeblockLumaH(pl, off, stride, a, b, bS)
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
