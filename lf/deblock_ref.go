package lf

// Portable reference, written from ITU-T H.264 (V16) (06/2026) clauses 8.7.2
// to 8.7.2.4 (Equations 8-451 to 8-489), for 8-bit luma samples
// (chromaStyleFilteringFlag 0).

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func clip3(lo, hi, v int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// lineStep returns the index of q0 of line k and the distance between
// successive samples across the edge.
func lineStep(off, stride, k int, vertical bool) (base, step int) {
	if vertical {
		return off + k*stride, 1
	}
	return off + k, stride
}

// deblockNormalRef is clause 8.7.2.3, for bS below 4.
func deblockNormalRef(pix []byte, off, stride int, vertical bool, indexA, indexB int, bS [4]uint8) {
	alpha, beta := int(alphaTable[indexA]), int(betaTable[indexB])
	for k := 0; k < 16; k++ {
		s := bS[k/4]
		base, step := lineStep(off, stride, k, vertical)
		p0, p1, p2 := int(pix[base-step]), int(pix[base-2*step]), int(pix[base-3*step])
		q0, q1, q2 := int(pix[base]), int(pix[base+step]), int(pix[base+2*step])

		// 8-460
		if s == 0 || abs(p0-q0) >= alpha || abs(p1-p0) >= beta || abs(q1-q0) >= beta {
			continue
		}
		tc0 := int(tc0Table[indexA][s-1])
		ap, aq := abs(p2-p0), abs(q2-q0) // 8-463, 8-464
		tc := tc0
		if ap < beta {
			tc++
		}
		if aq < beta {
			tc++
		} // 8-465
		delta := clip3(-tc, tc, (((q0-p0)<<2)+(p1-q1)+4)>>3) // 8-467
		pix[base-step] = byte(clip3(0, 255, p0+delta))       // 8-468
		pix[base] = byte(clip3(0, 255, q0-delta))            // 8-469
		if ap < beta {                                       // 8-470
			pix[base-2*step] = byte(p1 + clip3(-tc0, tc0, (p2+((p0+q0+1)>>1)-(p1<<1))>>1))
		}
		if aq < beta { // 8-472
			pix[base+step] = byte(q1 + clip3(-tc0, tc0, (q2+((p0+q0+1)>>1)-(q1<<1))>>1))
		}
	}
}

// deblockStrongRef is clause 8.7.2.4, for bS equal to 4.
func deblockStrongRef(pix []byte, off, stride int, vertical bool, indexA, indexB int) {
	alpha, beta := int(alphaTable[indexA]), int(betaTable[indexB])
	for k := 0; k < 16; k++ {
		base, step := lineStep(off, stride, k, vertical)
		p0, p1, p2, p3 := int(pix[base-step]), int(pix[base-2*step]), int(pix[base-3*step]), int(pix[base-4*step])
		q0, q1, q2, q3 := int(pix[base]), int(pix[base+step]), int(pix[base+2*step]), int(pix[base+3*step])

		if abs(p0-q0) >= alpha || abs(p1-p0) >= beta || abs(q1-q0) >= beta { // 8-460
			continue
		}
		ap, aq := abs(p2-p0), abs(q2-q0)
		near := abs(p0-q0) < (alpha>>2)+2
		if ap < beta && near { // 8-476
			pix[base-step] = byte((p2 + 2*p1 + 2*p0 + 2*q0 + q1 + 4) >> 3) // 8-477
			pix[base-2*step] = byte((p2 + p1 + p0 + q0 + 2) >> 2)          // 8-478
			pix[base-3*step] = byte((2*p3 + 3*p2 + p1 + p0 + q0 + 4) >> 3) // 8-479
		} else {
			pix[base-step] = byte((2*p1 + p0 + q1 + 2) >> 2) // 8-480
		}
		if aq < beta && near { // 8-483
			pix[base] = byte((p1 + 2*p0 + 2*q0 + 2*q1 + q2 + 4) >> 3)      // 8-484
			pix[base+step] = byte((p0 + q0 + q1 + q2 + 2) >> 2)            // 8-485
			pix[base+2*step] = byte((2*q3 + 3*q2 + q1 + q0 + p0 + 4) >> 3) // 8-486
		} else {
			pix[base] = byte((2*q1 + q0 + p1 + 2) >> 2) // 8-487
		}
	}
}
