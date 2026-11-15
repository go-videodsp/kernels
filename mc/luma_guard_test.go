//go:build unix

package mc

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
// (atEnd) or starts right after one, so that one byte read or written past
// the checked footprint faults instead of reading a neighbour.
func guarded(t *testing.T, n int, atEnd bool) []byte {
	t.Helper()
	pg := syscall.Getpagesize()
	body := (n + pg - 1) / pg * pg
	m, err := syscall.Mmap(-1, 0, body+2*pg, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_ANON|syscall.MAP_PRIVATE)
	if err != nil {
		t.Skipf("mmap: %v", err)
	}
	t.Cleanup(func() { _ = syscall.Munmap(m) })
	for _, g := range [][]byte{m[:pg], m[pg+body:]} {
		if err := syscall.Mprotect(g, syscall.PROT_NONE); err != nil {
			t.Skipf("mprotect: %v", err)
		}
	}
	if atEnd {
		return m[pg+body-n : pg+body : pg+body]
	}
	return m[pg : pg+n : pg+n]
}

var guardSink byte // keeps the probing read from being optimised away

// The guard works: touching one byte past the slice faults. Without this the
// tests below could pass on a broken setup.
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
				off = -1 // one before the start
			}
			guardSink = *(*byte)(unsafe.Add(unsafe.Pointer(&b[0]), off))
		}()
		if !faulted {
			t.Errorf("atEnd=%v: no fault past the slice", atEnd)
		}
	}
}

// Every position and size, with the source and the destination placed so
// that they end (or start) exactly at the guard page: any access outside the
// footprint the Go bounds proof computed faults.
func TestGuardPages(t *testing.T) {
	old := debug.SetPanicOnFault(true)
	defer debug.SetPanicOnFault(old)
	r := rand.New(rand.NewSource(7))
	for _, atEnd := range []bool{true, false} {
		for _, sz := range sizes {
			w, h := sz[0], sz[1]
			for fy := 0; fy < 4; fy++ {
				for fx := 0; fx < 4; fx++ {
					t.Run(fmt.Sprintf("end=%v/%dx%d/%c", atEnd, w, h, table812[fx][fy]), func(t *testing.T) {
						left, right, up, down := margins(fx, fy)
						for _, extra := range []int{0, 5} { // tight stride, and a wider one
							stride := w + left + right + extra
							srcOff := up*stride + left
							srcLen := srcOff + (h-1+down)*stride + w + right
							dstStride := w + extra
							dstLen := (h-1)*dstStride + w

							srcPlane := make([]byte, srcLen)
							fill(r, srcPlane, fillRandom, stride)
							dstInit := make([]byte, dstLen)
							fill(r, dstInit, fillRandom, 1)

							for _, avg := range []bool{false, true} {
								src := guarded(t, srcLen, atEnd)
								copy(src, srcPlane)
								dst := guarded(t, dstLen, atEnd)
								copy(dst, dstInit)
								want := append([]byte(nil), dstInit...)
								oracle(want, 0, dstStride, srcPlane, srcOff, stride, w, h, fx, fy, avg)
								if avg {
									AvgLuma(dst, 0, dstStride, src, srcOff, stride, w, h, fx, fy)
								} else {
									PutLuma(dst, 0, dstStride, src, srcOff, stride, w, h, fx, fy)
								}
								if !bytes.Equal(dst, want) {
									t.Fatalf("avg=%v extra=%d: first difference at %d", avg, extra, firstDiff(dst, want))
								}
							}
						}
					})
				}
			}
		}
	}
}
