package hevc

import (
	"encoding/binary"
	"errors"
	"testing"
)

// FuzzIDCT drives the transform kernels with sizes, shifts and coefficients taken from the
// bytes, often wrong: no panic, and an accepted call gives the text's result with the rest of
// dst untouched.
//
//	go test -fuzz='^FuzzIDCT$' -fuzztime=60s ./hevc     (run it on arm64 and on amd64)
func FuzzIDCT(f *testing.F) {
	f.Add([]byte{0, 3, 12, 1, 0, 100, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	f.Add([]byte{1, 2, 12, 255, 255, 255, 127, 0, 128, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 6 {
			return
		}
		dst4 := data[0]&1 == 1
		log2, bdShift := int(data[1])%8, int(data[2])%28
		body := data[3:]
		n := 1 << uint(log2)
		if dst4 {
			n = 4
		}
		coeffs := make([]int32, n*n+int(data[3])%4)
		for i := range coeffs {
			v := int32(int16(binary.LittleEndian.Uint16([]byte{body[i%len(body)], body[(i*5+1)%len(body)]})))
			if i%3 != 0 && len(body) > 8 && body[i%len(body)]&7 != 0 {
				v = 0 // mostly sparse, as the blocks are
			}
			coeffs[i] = v
		}
		if len(body) > 4 && body[4] == 255 { // sometimes a coefficient beyond 16 bits
			coeffs[int(body[3])%len(coeffs)] = 1 << 20
		}
		dst, want := make([]int32, n*n+int(data[4])%4), make([]int32, n*n+int(data[4])%4)
		for i := range dst {
			dst[i], want[i] = int32(sentinel16), int32(sentinel16)
		}
		var err error
		if dst4 {
			err = TryIDST4(dst, coeffs, bdShift)
		} else {
			err = TryIDCT(dst, coeffs, log2, bdShift)
		}
		if err != nil {
			if !errors.Is(err, ErrRefused) {
				t.Fatalf("an error that is not a refusal: %v", err)
			}
			return
		}
		m := dctEntry(n)
		if dst4 {
			m = dstEntry
		}
		copy(want, modelTransform(coeffs[:n*n], n, bdShift, m))
		for i := range dst {
			if dst[i] != want[i] {
				t.Fatalf("element %d is %d, the text gives %d", i, dst[i], want[i])
			}
		}
	})
}
