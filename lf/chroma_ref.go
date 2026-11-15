package lf

// Portable chroma reference: clauses 8.7.2.2 to 8.7.2.4 with
// chromaStyleFilteringFlag 1, 8-bit samples, 4:2:0.

// deblockChromaNormalRef is clause 8.7.2.3 for chroma: tC = tC0 + 1 (8-466),
// p0 and q0 only.
func deblockChromaNormalRef(pix []byte, off, stride int, vertical bool, indexA, indexB int, bS [4]uint8) {
	alpha, beta := int(alphaTable[indexA]), int(betaTable[indexB])
	for k := 0; k < 8; k++ {
		s := bS[k/2]
		base, step := lineStep(off, stride, k, vertical)
		p0, p1 := int(pix[base-step]), int(pix[base-2*step])
		q0, q1 := int(pix[base]), int(pix[base+step])
		if s == 0 || abs(p0-q0) >= alpha || abs(p1-p0) >= beta || abs(q1-q0) >= beta { // 8-460
			continue
		}
		tc := int(tc0Table[indexA][s-1]) + 1
		delta := clip3(-tc, tc, (((q0-p0)<<2)+(p1-q1)+4)>>3)
		pix[base-step] = byte(clip3(0, 255, p0+delta))
		pix[base] = byte(clip3(0, 255, q0-delta))
	}
}

// deblockChromaStrongRef is clause 8.7.2.4 for chroma (8-480, 8-487).
func deblockChromaStrongRef(pix []byte, off, stride int, vertical bool, indexA, indexB int) {
	alpha, beta := int(alphaTable[indexA]), int(betaTable[indexB])
	for k := 0; k < 8; k++ {
		base, step := lineStep(off, stride, k, vertical)
		p0, p1 := int(pix[base-step]), int(pix[base-2*step])
		q0, q1 := int(pix[base]), int(pix[base+step])
		if abs(p0-q0) >= alpha || abs(p1-p0) >= beta || abs(q1-q0) >= beta {
			continue
		}
		pix[base-step] = byte((2*p1 + p0 + q1 + 2) >> 2)
		pix[base] = byte((2*q1 + q0 + p1 + 2) >> 2)
	}
}
