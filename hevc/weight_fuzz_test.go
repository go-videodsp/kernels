package hevc

import (
	"encoding/binary"
	"errors"
	"testing"
)

// FuzzWeight drives both weighting kernels with parameters taken from the bytes, often wrong:
// no panic, and an accepted call gives the text's result with the rest of dst untouched.
//
//	go test -fuzz='^FuzzWeight$' -fuzztime=60s ./hevc     (run it on arm64 and on amd64)
func FuzzWeight(f *testing.F) {
	f.Add([]byte{0, 8, 4, 8, 20, 18, 21, 6, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	f.Add([]byte{1, 64, 64, 12, 70, 72, 200, 13, 255, 255, 255, 255, 0, 0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 12 {
			return
		}
		bi := data[0]&1 == 1
		w, h, bd := int(data[1])%80, int(data[2])%80, int(data[3])%16
		dStride, pStride := int(data[4])%100, int(data[5])%100
		pOff := int(binary.LittleEndian.Uint16(data[6:8])) % 8192
		log2Wd := int(data[7]) % 16
		w0, w1 := int32(int8(data[8]))*2, int32(int8(data[9]))*2
		o0, o1 := int32(int8(data[10]))*40, int32(int8(data[11]))*40
		body := data[12:]
		if len(body) == 0 {
			return
		}
		np, nd := len(body)*8, len(body)*4
		p0, p1 := make([]int32, np), make([]int32, np)
		for i := range p0 {
			p0[i] = int32(binary.LittleEndian.Uint16([]byte{body[i%len(body)], body[(i+1)%len(body)]})) - 8192
			p1[i] = p0[(i+3)%np] / 2
		}
		dst, want := make([]uint16, nd), make([]uint16, nd)
		for i := range dst {
			dst[i], want[i] = sentinel16, sentinel16
		}
		dOff, po := int(data[6])%16, pOff%np
		var err error
		if bi {
			err = TryWeightBi(dst, dOff, dStride, p0, p1, po, pStride, w, h, bd, log2Wd, w0, w1, o0, o1)
		} else {
			err = TryWeightUni(dst, dOff, dStride, p0, po, pStride, w, h, bd, log2Wd, w0, o0)
		}
		if err != nil {
			if !errors.Is(err, ErrRefused) {
				t.Fatalf("an error that is not a refusal: %v", err)
			}
			return
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				i := po + y*pStride + x
				if bi {
					want[dOff+y*dStride+x] = modelBi(int64(p0[i]), int64(p1[i]), int64(w0), int64(w1), int64(o0), int64(o1), uint(log2Wd), bd)
				} else {
					want[dOff+y*dStride+x] = modelUni(int64(p0[i]), int64(w0), int64(o0), uint(log2Wd), bd)
				}
			}
		}
		for i := range dst {
			if dst[i] != want[i] {
				t.Fatalf("element %d is %d, the text gives %d", i, dst[i], want[i])
			}
		}
	})
}
