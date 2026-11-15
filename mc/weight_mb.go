package mc

import (
	"fmt"

	"github.com/go-videodsp/kernels/internal/bounds"
)

// UniPart is one partition of a macroblock for WeightUniMB: its position (X, Y)
// inside the 16x16 macroblock, its size (one of the seven the standard uses:
// 16x16, 16x8, 8x16, 8x8, 8x4, 4x8, 4x4) and its explicit weight and offset
// (ranges of WeightUni).
type UniPart struct {
	X, Y, W, H     int
	Weight, Offset int
}

// BiPart is one partition of a macroblock for WeightBiMB, with the two weights
// and the two offsets of Equation 8-276.
type BiPart struct {
	X, Y, W, H int
	W0, W1     int
	O0, O1     int
}

// WeightUniMB is WeightUni for every partition of one macroblock in a single
// call (clause 8.4.2.3.2, Equations 8-274 and 8-275): the 16x16 macroblock whose
// top-left sample is dst[off], rows stride apart, is proved to lie inside dst
// once (internal/bounds.MacroBlock), each partition is proved to lie inside the
// macroblock, and the kernels then run on the partitions with no further check.
// Every partition uses the same logWD (luma_log2_weight_denom) with its own
// weight and offset.
//
// Partitions must not overlap (a sample weighted twice is not a prediction) and
// there are at most 16. WeightUniMB panics, before touching dst, when the
// macroblock does not fit, stride < 16, logWD is outside 0..7, or any partition
// has an unsupported size, lies outside the macroblock, overlaps another, or has
// a weight or offset outside WeightUni's ranges.
func WeightUniMB(dst []byte, off, stride int, parts []UniPart, logWD int) {
	if _, ok := bounds.MacroBlock(len(dst), off, stride); !ok || uint(logWD) > 7 || len(parts) > 16 {
		refuseMB("destination", len(dst), off, stride, logWD, len(parts))
	}
	// One snapshot of the partitions serves the check and the kernels: a caller
	// (another goroutine) that changes parts after the check cannot make the
	// kernels see a partition that was not proved.
	var local [16]UniPart
	n := copy(local[:], parts)
	if i := uniPartsBad(local[:n], logWD); i >= 0 {
		refusePart(i, local[i].X, local[i].Y, local[i].W, local[i].H)
	}
	weightUniParts(dst, off, stride, local[:n], logWD)
}

// uniPartsBad returns the index of the first partition that is unsupported,
// outside the macroblock, overlaps an earlier one or has weights out of range, or
// -1. A separate function keeps the entry's own frame small (the checks and the
// panics, inlined into it, cost about 25 cycles of spills on a 16x16 macroblock).
//
//go:noinline
func uniPartsBad(parts []UniPart, logWD int) int {
	for i := range parts {
		p := &parts[i]
		if !partOK(p.X, p.Y, p.W, p.H) || !weightOK(logWD, p.Weight, p.Offset) {
			return i
		}
		for j := 0; j < i; j++ {
			if q := &parts[j]; !disjoint(p.X, p.Y, p.W, p.H, q.X, q.Y, q.W, q.H) {
				return i
			}
		}
	}
	return -1
}

//go:noinline
func biPartsBad(parts []BiPart, logWD int) int {
	for i := range parts {
		p := &parts[i]
		if !partOK(p.X, p.Y, p.W, p.H) || !weightOK(logWD, p.W0, p.O0) || !weightOK(logWD, p.W1, p.O1) {
			return i
		}
		for j := 0; j < i; j++ {
			if q := &parts[j]; !disjoint(p.X, p.Y, p.W, p.H, q.X, q.Y, q.W, q.H) {
				return i
			}
		}
	}
	return -1
}

// weightUniPartsDispatch applies the weights to the partitions of a proved
// macroblock through the per-block dispatch (NEON on arm64, AVX2 or the
// portable code on amd64). off + Y*stride + X is inside the proved macroblock:
// Y <= 15 and stride <= (len-off-16)/15 were established, X <= 15.
func weightUniPartsDispatch(dst []byte, off, stride int, parts []UniPart, logWD int) {
	for i := range parts {
		p := &parts[i]
		weightUniDispatch(dst, off+p.Y*stride+p.X, stride, p.W, p.H, p.Weight, p.Offset, logWD)
	}
}

func weightBiPartsDispatch(dst []byte, dstOff, dstStride int, src []byte, srcOff, srcStride int, parts []BiPart, logWD int) {
	for i := range parts {
		p := &parts[i]
		weightBiApply(dst, dstOff+p.Y*dstStride+p.X, dstStride, src, srcOff+p.Y*srcStride+p.X, srcStride, p.W, p.H, p.W0, p.W1, p.O0, p.O1, logWD)
	}
}

// WeightBiMB is WeightBi for every partition of one macroblock in a single
// call: the destination and the source macroblocks are proved once, the
// partitions once against the macroblock, and the kernels run on them with no
// further check. dst and src must not share memory (the bounding ranges of the
// two macroblocks must be disjoint). Everything else is WeightUniMB's.
func WeightBiMB(dst []byte, dstOff, dstStride int, src []byte, srcOff, srcStride int, parts []BiPart, logWD int) {
	dLast, dok := bounds.MacroBlock(len(dst), dstOff, dstStride)
	sLast, sok := bounds.MacroBlock(len(src), srcOff, srcStride)
	if !dok {
		refuseMB("destination", len(dst), dstOff, dstStride, logWD, len(parts))
	}
	if !sok {
		refuseMB("source", len(src), srcOff, srcStride, logWD, len(parts))
	}
	if uint(logWD) > 7 || len(parts) > 16 {
		refuseMB("destination", len(dst), dstOff, dstStride, logWD, len(parts))
	}
	if overlaps(dst, dstOff, dLast, src, srcOff, sLast) {
		panic("mc: destination and source macroblocks share memory")
	}
	var local [16]BiPart // one snapshot for the check and the kernels, as in WeightUniMB
	n := copy(local[:], parts)
	if i := biPartsBad(local[:n], logWD); i >= 0 {
		refusePart(i, local[i].X, local[i].Y, local[i].W, local[i].H)
	}
	weightBiParts(dst, dstOff, dstStride, src, srcOff, srcStride, local[:n], logWD)
}

// weightOK is the accepting condition of checkWeights as one inlinable
// expression (uint conversions turn both bounds of each range into one compare).
func weightOK(logWD, weight, offset int) bool {
	return uint(logWD) <= 7 && uint(weight+128) <= 256 && uint(offset+128) <= 255
}

// partOK reports whether the w x h partition at (x, y) is a supported size and
// lies inside the 16x16 macroblock. A supported size has 16-w and 16-h in 0..12,
// so the unsigned compares also refuse a negative or huge x or y.
func partOK(x, y, w, h int) bool {
	return validSize(w, h) && uint(x) <= uint(16-w) && uint(y) <= uint(16-h)
}

// disjoint reports whether two partitions that are inside the macroblock (so
// their sums are small) share no sample.
func disjoint(ax, ay, aw, ah, bx, by, bw, bh int) bool {
	return ax+aw <= bx || bx+bw <= ax || ay+ah <= by || by+bh <= ay
}

// refuseMB and refusePart never return.
//
//go:noinline
func refuseMB(what string, n, off, stride, logWD, parts int) {
	if _, ok := bounds.MacroBlock(n, off, stride); !ok {
		panic(fmt.Sprintf("mc: %s macroblock does not fit: offset %d stride %d length %d", what, off, stride, n))
	}
	if uint(logWD) > 7 {
		panic(fmt.Sprintf("mc: logWD %d outside 0..7", logWD))
	}
	panic(fmt.Sprintf("mc: %d partitions, at most 16", parts))
}

//go:noinline
func refusePart(i, x, y, w, h int) {
	panic(fmt.Sprintf("mc: partition %d (%d,%d %dx%d) is unsupported, outside the macroblock, overlaps another, or has weights out of range", i, x, y, w, h))
}
