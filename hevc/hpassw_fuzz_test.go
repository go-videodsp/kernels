package hevc

import (
	"encoding/binary"
	"errors"
	"testing"
)

// FuzzHPassW drives the horizontal filters into int32 and the widening copy with parameters
// taken from the bytes, often wrong: no panic, and an accepted call gives the text's result with
// the rest of dst untouched.
//
//	go test -fuzz='^FuzzHPassW$' -fuzztime=60s ./hevc     (run it on arm64 and on amd64)
func FuzzHPassW(f *testing.F) {
	f.Add([]byte{0, 8, 4, 8, 20, 18, 21, 6, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20})
	f.Add([]byte{1, 10, 64, 12, 70, 72, 200, 13, 255, 255, 255, 255, 0, 0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	f.Add([]byte{2, 12, 6, 3, 70, 72, 200, 13, 255, 255, 255, 255, 0, 0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 24 {
			return
		}
		kind := int(data[0]) % 3 // luma, chroma, widen
		w, h, bd := int(data[1])%80, int(data[2])%80, int(data[3])%16
		dStride, sStride := int(data[4])%100, int(data[5])%100
		sOff := int(binary.LittleEndian.Uint16(data[6:8])) % 8192
		var c8 [8]int16
		for i := range c8 {
			c8[i] = int16(int8(data[8+i]))
		}
		c4 := [4]int16{c8[0], c8[1], c8[2], c8[3]}
		body := data[16:]
		ns := len(body) * 16
		src := make([]uint16, ns)
		for i := range src {
			src[i] = binary.LittleEndian.Uint16([]byte{body[i%len(body)], body[(i+1)%len(body)]})
			if kind != 2 {
				src[i] &= 0x7fff
			}
		}
		dst, want := make([]int32, len(body)*8), make([]int32, len(body)*8)
		for i := range dst {
			dst[i], want[i] = int32(sentinel16), int32(sentinel16)
		}
		dOff, so := int(data[6])%16, sOff%ns
		shift := int(data[3]) % 18
		var err error
		switch kind {
		case 0:
			err = TryHPassW8(dst, dOff, dStride, src, so, sStride, w, h, bd, &c8)
		case 1:
			err = TryHPassW4(dst, dOff, dStride, src, so, sStride, w, h, bd, &c4)
		default:
			err = TryWiden(dst, dOff, dStride, src, so, sStride, w, h, shift)
		}
		if err != nil {
			if !errors.Is(err, ErrRefused) {
				t.Fatalf("an error that is not a refusal: %v", err)
			}
			return
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				switch kind {
				case 0:
					want[dOff+y*dStride+x] = modelHW(src, so+y*sStride+x, c8[:], before, bd)
				case 1:
					want[dOff+y*dStride+x] = modelHW(src, so+y*sStride+x, c4[:], chromaBefore, bd)
				default:
					want[dOff+y*dStride+x] = int32(src[so+y*sStride+x]) << uint(shift)
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
