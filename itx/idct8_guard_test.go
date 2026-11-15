//go:build unix

package itx

import (
	"bytes"
	"fmt"
	"math/rand"
	"runtime/debug"
	"syscall"
	"testing"
)

// guarded returns a slice of n bytes that ends at an inaccessible page
// (atEnd) or starts right after one, so that one byte read or written past
// the checked footprint faults instead of reading a neighbour.
func guarded(t *testing.T, n int, atEnd bool) []byte {
	t.Helper()
	pg := syscall.Getpagesize()
	body := (n + pg - 1) / pg * pg
	m, err := syscall.Mmap(-1, 0, body+2*pg, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_ANON|syscall.MAP_PRIVATE)
	if err != nil {
		t.Fatalf("mmap: %v", err) // a gate that cannot be set up must not pass
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

var guardSink byte // keeps the probing read from being optimised away

// The guard works: touching one byte past the slice faults. Without this the
// test below could pass on a broken setup.
func TestGuardFaults(t *testing.T) {
	old := debug.SetPanicOnFault(true)
	defer debug.SetPanicOnFault(old)
	for _, atEnd := range []bool{true, false} {
		b := guarded(t, 100, atEnd)
		var faulted bool
		func() {
			defer func() { faulted = recover() != nil }()
			off := len(b) // one past the end
			if !atEnd {
				off = -1
			}
			p := &b[0]
			guardSink = *(*byte)(addOffset(p, off))
		}()
		if !faulted {
			t.Fatalf("atEnd=%v: access beyond the slice did not fault", atEnd)
		}
	}
}

// The block with its footprint placed so that it ends (or starts) exactly at
// a PROT_NONE page: a read or write outside the eight rows of eight bytes
// faults.
func TestGuardPages(t *testing.T) {
	old := debug.SetPanicOnFault(true)
	defer debug.SetPanicOnFault(old)
	r := rand.New(rand.NewSource(21))
	for _, atEnd := range []bool{true, false} {
		for _, stride := range []int{8, 11, 64} {
			t.Run(fmt.Sprintf("end=%v/stride=%d", atEnd, stride), func(t *testing.T) {
				n := 7*stride + 8
				for k := blockKind(0); k < numKinds; k++ {
					for _, qpDiv := range []int{0, 3, 5, 6, 8, 31} {
						coef, scale := randomBlock(r, k)
						init := randomPlane(r, n)
						d := guarded(t, n, atEnd)
						copy(d, init)
						Dequant8x8IDCTAdd(d, 0, stride, &coef, &scale, qpDiv)
						want := append([]byte(nil), init...)
						dequant8x8IDCTAddRef(want, 0, stride, &coef, &scale, qpDiv)
						if !bytes.Equal(d, want) {
							t.Fatalf("kind %d qpDiv %d: first difference at %d", k, qpDiv, firstDiff(d, want))
						}
					}
				}
			})
		}
	}
}
