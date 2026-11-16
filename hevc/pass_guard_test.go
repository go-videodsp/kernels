//go:build unix

package hevc

import (
	"math/rand"
	"runtime/debug"
	"syscall"
	"testing"
	"unsafe"
)

// guarded returns the memory for n elements of size bytes each, ending at an
// inaccessible page (atEnd) or starting right after one, so that one element read or
// written past the checked footprint faults instead of touching a neighbour.
func guarded(t *testing.T, n, size int, atEnd bool) unsafe.Pointer {
	t.Helper()
	pg := syscall.Getpagesize()
	nb := n * size
	body := (nb + pg - 1) / pg * pg
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
		return unsafe.Pointer(&m[pg+body-nb])
	}
	return unsafe.Pointer(&m[pg])
}

var guardSink byte

// The guard works: one byte past the memory faults. Without this the tests below
// could pass on a broken setup.
func TestGuardFaults(t *testing.T) {
	old := debug.SetPanicOnFault(true)
	defer debug.SetPanicOnFault(old)
	for _, atEnd := range []bool{true, false} {
		p := guarded(t, 100, 2, atEnd)
		off := uintptr(200) // one past the end
		if !atEnd {
			off = ^uintptr(0) // one before the start
		}
		var faulted bool
		func() {
			defer func() { faulted = recover() != nil }()
			guardSink = *(*byte)(unsafe.Add(p, int(off)))
		}()
		if !faulted {
			t.Fatalf("atEnd=%v: an access beyond the memory did not fault", atEnd)
		}
	}
}

// The passes with exactly their footprint in memory that ends (or starts) at a
// PROT_NONE page: a read or write outside the footprint or the block faults.
func TestPassesGuardPages(t *testing.T) {
	old := debug.SetPanicOnFault(true)
	defer debug.SetPanicOnFault(old)
	rng := rand.New(rand.NewSource(3))
	c := lumaFilters[1]
	for _, atEnd := range []bool{true, false} {
		for w := 4; w <= 64; w += 4 {
			for _, h := range []int{1, 2, 5, 8, 64} {
				dStride, sStride := w+rng.Intn(3), w+7+rng.Intn(3)
				// horizontal: footprint rows 0..h-1, columns -3 .. w+3
				ns, nd := (h-1)*sStride+w+7, (h-1)*dStride+w
				src := unsafe.Slice((*uint16)(guarded(t, ns, 2, atEnd)), ns)
				dst := unsafe.Slice((*int16)(guarded(t, nd, 2, atEnd)), nd)
				for i := range src {
					src[i] = uint16(rng.Intn(4096))
				}
				if err := TryHPass8(dst, 0, dStride, src, 3, sStride, w, h, 12, &c); err != nil {
					t.Fatalf("horizontal %dx%d: %v", w, h, err)
				}
				// vertical: footprint rows -3 .. h+3, columns 0..w-1
				sStride = w + rng.Intn(3)
				ns, nd = (h+7-1)*sStride+w, (h-1)*dStride+w
				mid := unsafe.Slice((*int16)(guarded(t, ns, 2, atEnd)), ns)
				out := unsafe.Slice((*int32)(guarded(t, nd, 4, atEnd)), nd)
				for i := range mid {
					mid[i] = int16(rng.Intn(20000))
				}
				if err := TryVPass8(out, 0, dStride, mid, 3*sStride, sStride, w, h, &c); err != nil {
					t.Fatalf("vertical %dx%d: %v", w, h, err)
				}
			}
		}
	}
}
