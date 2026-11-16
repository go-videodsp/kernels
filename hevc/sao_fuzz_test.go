package hevc

import (
	"encoding/binary"
	"errors"
	"testing"
)

// FuzzSAO drives both SAO kernels with parameters taken from the bytes, often wrong: no
// panic, and an accepted call gives the text's result with the rest of dst untouched.
//
//	go test -fuzz='^FuzzSAO$' -fuzztime=60s ./hevc     (run it on arm64 and on amd64)
func FuzzSAO(f *testing.F) {
	f.Add([]byte{0, 12, 4, 8, 20, 18, 21, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15})
	f.Add([]byte{3, 64, 64, 12, 70, 72, 200, 255, 255, 255, 255, 0, 0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 9 {
			return
		}
		mode := int(data[0]) % 5 // 0 band, 1..4 edge classes
		w, h, bd := int(data[1])%80, int(data[2])%80, int(data[3])%16
		dStride, sStride := int(data[4])%100, int(data[5])%100
		srcOff := int(binary.LittleEndian.Uint16(data[6:8])) % 8192
		body := data[8:]
		off := [4]int16{int16(int8(body[0])), int16(int8(body[len(body)/2])), int16(int8(body[len(body)-1])), int16(len(body)%7 - 3)}
		nSrc, nDst := len(body)*8, len(body)*4
		src := make([]uint16, nSrc)
		for i := range src {
			src[i] = uint16(body[i%len(body)]) << uint(bd%5) & uint16(1<<uint(bd)-1) // a sample of the bit depth
		}
		dst, want := make([]uint16, nDst), make([]uint16, nDst)
		for i := range dst {
			dst[i], want[i] = sentinel16, sentinel16
		}
		dOff, so := int(data[6])%16, srcOff%nSrc
		var err error
		if mode == 0 {
			err = TrySAOBand(dst, dOff, dStride, src, so, sStride, w, h, bd, int(data[7])%40-4, &off)
		} else {
			err = TrySAOEdge(dst, dOff, dStride, src, so, sStride, w, h, bd, mode-1, &off)
		}
		if err != nil {
			if !errors.Is(err, ErrRefused) {
				t.Fatalf("an error that is not a refusal: %v", err)
			}
			return
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				i := so + y*sStride + x
				cur := int(src[i])
				if mode == 0 {
					want[dOff+y*dStride+x] = uint16(modelBand(cur, int(data[7])%40-4, off, bd))
					continue
				}
				class := mode - 1
				a, b := saoEdgeOffsets(class, sStride)
				want[dOff+y*dStride+x] = uint16(modelEdge(cur, int(src[i+a]), int(src[i+b]), off, bd))
			}
		}
		for i := range dst {
			if dst[i] != want[i] {
				t.Fatalf("element %d is %d, the text gives %d", i, dst[i], want[i])
			}
		}
	})
}
