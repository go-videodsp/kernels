package hevc

import (
	"encoding/binary"
	"errors"
	"testing"
)

// FuzzBytesDst drives the byte-destination kernels (average, the two weightings, the two SAO
// filters) with parameters taken from the input, often wrong: no panic, and an accepted call
// gives the text's result with the rest of dst untouched.
//
//	go test -fuzz='^FuzzBytesDst$' -fuzztime=60s ./hevc     (run it on arm64 and on amd64)
func FuzzBytesDst(f *testing.F) {
	f.Add([]byte{0, 8, 4, 8, 20, 18, 21, 6, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20})
	f.Add([]byte{1, 10, 6, 6, 70, 72, 200, 13, 255, 255, 255, 255, 0, 0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	f.Add([]byte{2, 12, 6, 3, 70, 72, 200, 13, 255, 255, 255, 255, 0, 0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	f.Add([]byte{3, 16, 6, 5, 40, 40, 20, 13, 4, 1, 4, 1, 0, 0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	f.Add([]byte{4, 11, 5, 2, 40, 40, 20, 13, 4, 1, 4, 1, 0, 0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 24 {
			return
		}
		kind := int(data[0]) % 5 // average, weight uni, weight bi, SAO band, SAO edge
		w, h := int(data[1])%80, int(data[2])%80
		dStride, sStride := int(data[4])%100, int(data[5])%100
		sOff := int(binary.LittleEndian.Uint16(data[6:8])) % 8192
		body := data[16:]
		ns := len(body) * 16
		src, src2 := make([]uint8, ns), make([]uint8, ns)
		p0, p1 := make([]int32, ns), make([]int32, ns)
		for i := range src {
			src[i] = body[i%len(body)] + uint8(i/len(body))
			src2[i] = body[(i+1)%len(body)] ^ uint8(i)
			p0[i] = int32(int16(binary.LittleEndian.Uint16([]byte{body[i%len(body)], body[(i+3)%len(body)]})))
			p1[i] = int32(int16(binary.LittleEndian.Uint16([]byte{body[(i+5)%len(body)], body[(i+7)%len(body)]})))
		}
		dst, want := fillBytes(len(body)*8, sentinel8), fillBytes(len(body)*8, sentinel8)
		dOff, so := int(data[6])%16, sOff%ns
		off := [4]int16{int16(int8(data[8])), int16(int8(data[9])), int16(int8(data[10])), int16(int8(data[11]))}
		log2Wd, wt, o := int(data[3])%16, int32(int8(data[8])), int32(int8(data[9]))
		w1, o1 := int32(int8(data[10])), int32(int8(data[11]))
		var err error
		switch kind {
		case 0:
			err = TryAverageBytes(dst, dOff, dStride, src, so, sStride, src2, so/2, sStride, w, h)
		case 1:
			err = TryWeightUniBytes(dst, dOff, dStride, p0, so, sStride, w, h, log2Wd, wt, o)
		case 2:
			err = TryWeightBiBytes(dst, dOff, dStride, p0, p1, so, sStride, w, h, log2Wd, wt, w1, o, o1)
		case 3:
			err = TrySAOBandBytes(dst, dOff, dStride, src, so, sStride, w, h, int(data[3])%34, &off)
		default:
			err = TrySAOEdgeBytes(dst, dOff, dStride, src, so, sStride, w, h, int(data[3])%5, &off)
		}
		if err != nil {
			if !errors.Is(err, ErrRefused) {
				t.Fatalf("an error that is not a refusal: %v", err)
			}
			return
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				d, s := dOff+y*dStride+x, so+y*sStride+x
				switch kind {
				case 0:
					want[d] = uint8((int(src[s]) + int(src2[so/2+y*sStride+x]) + 1) >> 1)
				case 1:
					want[d] = modelWeightUni(p0[s], log2Wd, wt, o)
				case 2:
					want[d] = modelWeightBi(p0[s], p1[s], log2Wd, wt, w1, o, o1)
				case 3:
					want[d] = modelSAOBand(src[s], int(data[3])%34, &off)
				default:
					offA, offB := saoEdgeOffsets(int(data[3])%5, sStride)
					want[d] = modelSAOEdge(src[s], src[s+offA], src[s+offB], &off)
				}
			}
		}
		for i := range dst {
			if dst[i] != want[i] {
				t.Fatalf("kind %d: element %d is %d, the text gives %d", kind, i, dst[i], want[i])
			}
		}
	})
}
