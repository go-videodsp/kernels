package lf

import "fmt"

//go:generate go run ../internal/genmc ..

// Alpha is the threshold α of clause 8.7.2.2 for 8-bit samples: α′ of
// Table 8-16 at indexA (Equation 8-456 with BitDepth 8). indexA is
// Clip3(0, 51, qPav + filterOffsetA) (Equation 8-454).
func Alpha(indexA int) int {
	checkIndex("indexA", indexA)
	return int(alphaTable[indexA])
}

// Beta is the threshold β of clause 8.7.2.2 for 8-bit samples: β′ of
// Table 8-16 at indexB (Equation 8-457).
func Beta(indexB int) int {
	checkIndex("indexB", indexB)
	return int(betaTable[indexB])
}

// TC0 is t′C0 of Table 8-17 at indexA for boundary strength bS 1..3 (the
// value of Equation 8-461 for 8-bit samples).
func TC0(bS, indexA int) int {
	checkIndex("indexA", indexA)
	if bS < 1 || bS > 3 {
		panic(fmt.Sprintf("lf: tC0 is defined for bS 1..3, got %d", bS))
	}
	return int(tc0Table[indexA][bS-1])
}

func checkIndex(name string, v int) {
	if v < 0 || v > 51 {
		panic(fmt.Sprintf("lf: %s %d outside 0..51", name, v))
	}
}

// DeblockLumaV filters one vertical luma edge of 16 lines with boundary
// strength below 4 (ITU-T H.264 (V16) (06/2026), clause 8.7.2.3, Equations
// 8-463 to 8-475, after the thresholds of 8.7.2.2): pix[off] is q0 of the
// first line, the sample just right of the edge, and line k is row k, so
// p_i = pix[off + k*stride - i - 1] and q_i = pix[off + k*stride + i]
// (Equations 8-442, 8-443). bS holds the strength of each group of four
// lines, 0 to 3 (0 leaves the group untouched); indexA and indexB are those
// of Equations 8-454 and 8-455.
//
// The footprint is columns -4 .. +3 around the edge on all 16 lines, whatever
// the strengths: DeblockLumaV panics, before touching pix, when it does not
// fit, when stride < 16, when an index is outside 0..51 or a strength
// outside 0..3.
func DeblockLumaV(pix []byte, off, stride, indexA, indexB int, bS [4]uint8) {
	checkEdge(len(pix), off, stride, true, indexA, indexB)
	for _, s := range bS {
		if s > 3 {
			panic(fmt.Sprintf("lf: bS %d is not below 4; use DeblockLumaIntraV for bS 4", s))
		}
	}
	deblockNormal(pix, off, stride, true, indexA, indexB, bS)
}

// DeblockLumaH is DeblockLumaV for a horizontal edge: pix[off] is q0 of the
// first line, the sample just below the edge, and line k is column k, so
// p_i = pix[off - (i+1)*stride + k] and q_i = pix[off + i*stride + k]
// (Equations 8-444, 8-445). The footprint is rows -4 .. +3 on 16 columns.
func DeblockLumaH(pix []byte, off, stride, indexA, indexB int, bS [4]uint8) {
	checkEdge(len(pix), off, stride, false, indexA, indexB)
	for _, s := range bS {
		if s > 3 {
			panic(fmt.Sprintf("lf: bS %d is not below 4; use DeblockLumaIntraH for bS 4", s))
		}
	}
	deblockNormal(pix, off, stride, false, indexA, indexB, bS)
}

// DeblockLumaIntraV filters one vertical luma edge of 16 lines with boundary
// strength 4 on every line (clause 8.7.2.4, Equations 8-476 to 8-489).
// Geometry, footprint and panics are DeblockLumaV's.
func DeblockLumaIntraV(pix []byte, off, stride, indexA, indexB int) {
	checkEdge(len(pix), off, stride, true, indexA, indexB)
	deblockStrong(pix, off, stride, true, indexA, indexB)
}

// DeblockLumaIntraH is DeblockLumaIntraV for a horizontal edge.
func DeblockLumaIntraH(pix []byte, off, stride, indexA, indexB int) {
	checkEdge(len(pix), off, stride, false, indexA, indexB)
	deblockStrong(pix, off, stride, false, indexA, indexB)
}

// checkEdge is the bounds proof, in int64 so that no 32-bit target can
// overflow. Lines of a horizontal edge are 16 adjacent columns and rows of
// the footprint must not overlap, hence stride >= 16 for both directions.
func checkEdge(n, off, stride int, vertical bool, indexA, indexB int) {
	checkIndex("indexA", indexA)
	checkIndex("indexB", indexB)
	if stride < 16 {
		panic(fmt.Sprintf("lf: stride %d is smaller than the 16 lines of an edge", stride))
	}
	var first, last int64
	if vertical {
		first = int64(off) - 4
		last = int64(off) + 15*int64(stride) + 3
	} else {
		first = int64(off) - 4*int64(stride)
		last = int64(off) + 3*int64(stride) + 15
	}
	if first < 0 || last >= int64(n) {
		panic(fmt.Sprintf("lf: edge footprint does not fit: offset %d stride %d vertical %v length %d", off, stride, vertical, n))
	}
}

// Table 8-16: α′ and β′ by indexA / indexB.
var alphaTable = [52]uint8{
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	4, 4, 5, 6, 7, 8, 9, 10, 12, 13,
	15, 17, 20, 22, 25, 28, 32, 36, 40, 45, 50, 56, 63, 71, 80, 90, 101, 113, 127, 144, 162, 182, 203, 226, 255, 255,
}

var betaTable = [52]uint8{
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	2, 2, 2, 3, 3, 3, 3, 4, 4, 4,
	6, 6, 7, 7, 8, 8, 9, 9, 10, 10, 11, 11, 12, 12, 13, 13, 14, 14, 15, 15, 16, 16, 17, 17, 18, 18,
}

// Table 8-17: t′C0 by indexA, columns bS = 1, 2, 3.
var tc0Table = [52][3]uint8{
	0: {0, 0, 0}, 1: {0, 0, 0}, 2: {0, 0, 0}, 3: {0, 0, 0}, 4: {0, 0, 0}, 5: {0, 0, 0}, 6: {0, 0, 0}, 7: {0, 0, 0},
	8: {0, 0, 0}, 9: {0, 0, 0}, 10: {0, 0, 0}, 11: {0, 0, 0}, 12: {0, 0, 0}, 13: {0, 0, 0}, 14: {0, 0, 0}, 15: {0, 0, 0},
	16: {0, 0, 0}, 17: {0, 0, 1}, 18: {0, 0, 1}, 19: {0, 0, 1}, 20: {0, 0, 1}, 21: {0, 1, 1}, 22: {0, 1, 1}, 23: {1, 1, 1},
	24: {1, 1, 1}, 25: {1, 1, 1}, 26: {1, 1, 1}, 27: {1, 1, 2}, 28: {1, 1, 2}, 29: {1, 1, 2}, 30: {1, 1, 2}, 31: {1, 2, 3},
	32: {1, 2, 3}, 33: {2, 2, 3}, 34: {2, 2, 4}, 35: {2, 3, 4}, 36: {2, 3, 4}, 37: {3, 3, 5}, 38: {3, 4, 6}, 39: {3, 4, 6},
	40: {4, 5, 7}, 41: {4, 5, 8}, 42: {4, 6, 9}, 43: {5, 7, 10}, 44: {6, 8, 11}, 45: {6, 8, 13}, 46: {7, 10, 14}, 47: {8, 11, 16},
	48: {9, 12, 18}, 49: {10, 13, 20}, 50: {11, 15, 23}, 51: {13, 17, 25},
}
