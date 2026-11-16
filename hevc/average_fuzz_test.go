package hevc

import (
	"encoding/binary"
	"errors"
	"testing"
)

// FuzzAverage drives the averaging kernel with parameters taken from the bytes, often
// wrong: no panic, and an accepted call gives the text's result with the rest of dst
// untouched.
//
//	go test -fuzz='^FuzzAverage$' -fuzztime=60s ./hevc     (run it on arm64 and on amd64)
func FuzzAverage(f *testing.F) {
	f.Add([]byte{8, 4, 8, 9, 10, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12})
	f.Add([]byte{64, 64, 64, 66, 70, 255, 255, 255, 255, 0, 0, 0, 0, 1, 2, 3, 4, 5, 6})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 8 {
			return
		}
		w, h := int(data[0])%80, int(data[1])%80
		dStride, aStride, bStride := int(data[2])%100, int(data[3])%100, int(data[4])%100
		aOff := int(binary.LittleEndian.Uint16(data[5:7])) % 8192
		body := data[7:]
		na := len(body) * 8
		a, b := make([]uint16, na), make([]uint16, na)
		for i := range a {
			a[i] = binary.LittleEndian.Uint16([]byte{body[i%len(body)], body[(i+1)%len(body)]})
			b[i] = a[(i*7+3)%na] ^ 0x5a5a
		}
		dst, want := make([]uint16, len(body)*4), make([]uint16, len(body)*4)
		for i := range dst {
			dst[i], want[i] = sentinel16, sentinel16
		}
		dOff, ao, bo := int(data[5])%16, aOff%na, int(data[6])%na
		if err := TryAverage(dst, dOff, dStride, a, ao, aStride, b, bo, bStride, w, h); err != nil {
			if !errors.Is(err, ErrRefused) {
				t.Fatalf("an error that is not a refusal: %v", err)
			}
			return
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				want[dOff+y*dStride+x] = modelAverage(a[ao+y*aStride+x], b[bo+y*bStride+x])
			}
		}
		for i := range dst {
			if dst[i] != want[i] {
				t.Fatalf("element %d is %d, the text gives %d", i, dst[i], want[i])
			}
		}
	})
}
