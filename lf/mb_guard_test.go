//go:build unix

package lf

import (
	"bytes"
	"math/rand"
	"runtime/debug"
	"testing"
)

// DeblockMB with the three planes cut exactly to the macroblock windows and
// placed against a PROT_NONE page at either end: an access outside the window
// (the 16x16 luma block and 8x8 chroma blocks plus the borders of the filtered
// outer edges) faults.
func TestDeblockMBGuardPages(t *testing.T) {
	old := debug.SetPanicOnFault(true)
	defer debug.SetPanicOnFault(old)
	r := rand.New(rand.NewSource(55))
	for _, atEnd := range []bool{true, false} {
		for _, strideExtra := range []int{0, 3, 40} {
			yStride, cStride := 16+strideExtra, 8+strideExtra
			// all outer edges filtered: the borders are included
			yN, cN := 19*yStride+16+4, 9*cStride+8+2
			yOff, cOff := 4*yStride+4, 2*cStride+2
			for n := 0; n < 60; n++ {
				e := randomEdges(r)
				e.BS[0][0] = [4]uint8{1, 2, 0, 3}
				e.BS[1][0] = [4]uint8{3, 3, 1, 1}
				yInit, cbInit, crInit := smoothPlane(r, yN), smoothPlane(r, cN), smoothPlane(r, cN)
				y, cb, cr := guarded(t, yN, atEnd), guarded(t, cN, atEnd), guarded(t, cN, atEnd)
				copy(y, yInit)
				copy(cb, cbInit)
				copy(cr, crInit)
				DeblockMB(y, yOff, yStride, cb, cr, cOff, cStride, e)
				y2, cb2, cr2 := append([]byte(nil), yInit...), append([]byte(nil), cbInit...), append([]byte(nil), crInit...)
				composeMB(y2, yOff, yStride, cb2, cr2, cOff, cStride, e)
				if !bytes.Equal(y, y2) || !bytes.Equal(cb, cb2) || !bytes.Equal(cr, cr2) {
					t.Fatalf("atEnd=%v stride extra %d: result differs from the per-edge calls", atEnd, strideExtra)
				}
			}
		}
	}
}
