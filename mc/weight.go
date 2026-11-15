package mc

import (
	"fmt"

	"github.com/go-videodsp/kernels/internal/bounds"
)

//go:generate go run ../internal/genweight86 ..

// WeightUni applies explicit weighted prediction to one prediction block in
// place (ITU-T H.264 (V16) (06/2026), clause 8.4.2.3.2, Equations 8-274 and
// 8-275, the single-list forms):
//
//	logWD >= 1: Clip1(((p * weight + 2^(logWD-1)) >> logWD) + offset)
//	logWD == 0: Clip1(p * weight + offset)
//
// for each of the w x h samples p at dst[off + y*stride + x]. The variables
// are the standard's logWD, w0C (or w1C) and o0C (or o1C) for 8-bit samples:
// logWD is 0..7 (luma_log2_weight_denom), weight -128..128 (a weight flag
// that is 0 infers 2^logWD, which is 128 at most) and offset -128..127.
//
// There is a single buffer, so there is no source to overlap. The supported
// sizes are PutLuma's. WeightUni panics before touching dst
// when the size, a parameter or the block's extent is not supported, or
// stride < w (rows would overlap).
func WeightUni(dst []byte, off, stride, w, h, weight, offset, logWD int) {
	if !(blockFits(len(dst), off, stride, w, h) && weightOK(logWD, weight, offset)) {
		refuseUni(len(dst), off, stride, w, h, weight, offset, logWD)
	}
	weightUniDispatch(dst, off, stride, w, h, weight, offset, logWD)
}

// WeightBi combines two predictions into the first, in place (clause
// 8.4.2.3.2, Equation 8-276, both lists in use):
//
//	Clip1(((p0 * w0 + p1 * w1 + 2^logWD) >> (logWD + 1)) + ((o0 + o1 + 1) >> 1))
//
// where p0 is the sample at dst[dstOff + y*dstStride + x] (the list 0
// prediction, replaced by the result) and p1 the one at
// src[srcOff + y*srcStride + x]. Implicit weighted prediction (8.4.3,
// Equations 8-277 to 8-283) is logWD = 5, o0 = o1 = 0 and the derived
// weights, which lie in -64..128; with w0 = w1 = 32 the formula is exactly the
// default rounding average of 8-273 and WeightBi takes that path (the same
// holds for any w0 = w1 = 2^logWD with (o0 + o1 + 1) >> 1 == 0). Ranges and
// panics are WeightUni's, for both weights and both offsets; the constraint
// of Equation 8-298 on w0 + w1 is the bitstream's and is not checked.
//
// dst and src must not share memory: WeightBi panics when the byte range of
// the destination block (first to last sample) intersects the source block's,
// which it can tell from the addresses; two slices of one array are fine
// while the ranges are disjoint. The check is on the bounding ranges, so
// blocks whose rows interleave in one plane are refused even if no sample is
// shared.
func WeightBi(dst []byte, dstOff, dstStride int, src []byte, srcOff, srcStride, w, h, w0, w1, o0, o1, logWD int) {
	if !(blockFits(len(dst), dstOff, dstStride, w, h) && blockFits(len(src), srcOff, srcStride, w, h) &&
		weightOK(logWD, w0, o0) && weightOK(logWD, w1, o1)) {
		refuseBi(len(dst), dstOff, dstStride, len(src), srcOff, srcStride, w, h, w0, w1, o0, o1, logWD)
	}
	dLast := dstOff + (h-1)*dstStride + w - 1 // the gate proved the blocks fit
	sLast := srcOff + (h-1)*srcStride + w - 1
	if overlaps(dst, dstOff, dLast, src, srcOff, sLast) {
		panic("mc: destination and source blocks share memory")
	}
	if w0 == 1<<logWD && w1 == w0 && (o0+o1+1)>>1 == 0 {
		lumaDispatch(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, 0, 0, true)
		return
	}
	weightBiDispatch(dst, dstOff, dstStride, src, srcOff, srcStride, w, h, w0, w1, o0, o1, logWD)
}

// blockFits and weightOK are the accepting conditions of checkBlock and
// checkWeights as one inlinable expression each: the call path pays two
// predictable branches instead of two calls (measured 3.7 ns for the calls on a
// 16x16 block, the larger part of the cost of WeightUni). A call they refuse
// goes to refuseUni, which runs the full checks and panics with their messages.
func blockFits(n, off, stride, w, h int) bool {
	return validSize(w, h) && stride >= w && off >= 0 && int64(off)+int64(h-1)*int64(stride)+int64(w-1) < int64(n)
}

func weightOK(logWD, weight, offset int) bool {
	return uint(logWD) <= 7 && uint(weight+128) <= 256 && uint(offset+128) <= 255
}

// refuseUni never returns: the detailed checks name what is wrong; if they
// find nothing the fast gate and they disagree, which is a defect, still a
// refusal.
//
//go:noinline
func refuseUni(n, off, stride, w, h, weight, offset, logWD int) {
	checkBlock("destination", n, off, stride, w, h)
	checkWeights(logWD, weight, offset)
	panic("mc: WeightUni gate and checks disagree")
}

// refuseBi is refuseUni for WeightBi.
//
//go:noinline
func refuseBi(dn, dstOff, dstStride, sn, srcOff, srcStride, w, h, w0, w1, o0, o1, logWD int) {
	checkBlock("destination", dn, dstOff, dstStride, w, h)
	checkBlock("source", sn, srcOff, srcStride, w, h)
	checkWeights(logWD, w0, o0)
	checkWeights(logWD, w1, o1)
	panic("mc: WeightBi gate and checks disagree")
}

func checkWeights(logWD, weight, offset int) {
	if logWD < 0 || logWD > 7 {
		panic(fmt.Sprintf("mc: logWD %d outside 0..7", logWD))
	}
	if weight < -128 || weight > 128 {
		panic(fmt.Sprintf("mc: weight %d outside -128..128", weight))
	}
	if offset < -128 || offset > 127 {
		panic(fmt.Sprintf("mc: offset %d outside -128..127", offset))
	}
}

// checkBlock proves that a w x h block at off with the stride fits in a
// slice of n bytes (internal/bounds.Window: no product can wrap on any
// target). It returns the index of the block's last sample.
func checkBlock(what string, n, off, stride, w, h int) int {
	if !validSize(w, h) {
		panic(fmt.Sprintf("mc: unsupported block size %dx%d", w, h))
	}
	if stride < w {
		panic(fmt.Sprintf("mc: %s stride %d is smaller than the block width %d", what, stride, w))
	}
	_, last, ok := bounds.Window(n, off, stride, 0, h-1, 0, w-1)
	if !ok {
		panic(fmt.Sprintf("mc: %s block does not fit: offset %d stride %d size %dx%d length %d", what, off, stride, w, h, n))
	}
	return last
}

// weightUniRef and weightBiRef are the portable forms, transcribed from the
// equations.
func weightUniRef(dst []byte, off, stride, w, h, weight, offset, logWD int) {
	rnd := 0
	if logWD >= 1 {
		rnd = 1 << (logWD - 1)
	}
	for y := 0; y < h; y++ {
		row := dst[off+y*stride : off+y*stride+w]
		for x, p := range row {
			row[x] = byte(clip1(int32(((int(p)*weight + rnd) >> logWD) + offset)))
		}
	}
}

func weightBiRef(dst []byte, dstOff, dstStride int, src []byte, srcOff, srcStride, w, h, w0, w1, o0, o1, logWD int) {
	o := (o0 + o1 + 1) >> 1
	for y := 0; y < h; y++ {
		d := dst[dstOff+y*dstStride : dstOff+y*dstStride+w]
		s := src[srcOff+y*srcStride : srcOff+y*srcStride+w]
		for x := range d {
			d[x] = byte(clip1(int32(((int(d[x])*w0 + int(s[x])*w1 + 1<<logWD) >> (logWD + 1)) + o)))
		}
	}
}
