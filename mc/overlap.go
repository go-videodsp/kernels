package mc

import "unsafe"

// overlaps reports whether the bytes a[aLo..aHi] and b[bLo..bHi] (inclusive
// indices, already proved to be inside their slices) intersect in memory.
func overlaps(a []byte, aLo, aHi int, b []byte, bLo, bHi int) bool {
	pa := uintptr(unsafe.Pointer(unsafe.SliceData(a)))
	pb := uintptr(unsafe.Pointer(unsafe.SliceData(b)))
	return pa+uintptr(aLo) <= pb+uintptr(bHi) && pb+uintptr(bLo) <= pa+uintptr(aHi)
}
