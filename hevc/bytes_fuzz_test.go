package hevc

import (
	"encoding/binary"
	"errors"
	"testing"
)

// FuzzBytes drives the six filters and the widening copy over the bytes of an 8-bit picture
// with parameters taken from the input, often wrong: no panic, and an accepted call gives the
// text's result with the rest of dst untouched.
//
//	go test -fuzz='^FuzzBytes$' -fuzztime=60s ./hevc     (run it on arm64 and on amd64)
func FuzzBytes(f *testing.F) {
	f.Add([]byte{0, 8, 4, 8, 20, 18, 21, 6, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20})
	f.Add([]byte{1, 10, 64, 12, 70, 72, 200, 13, 255, 255, 255, 255, 0, 0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	f.Add([]byte{2, 12, 6, 3, 70, 72, 200, 13, 255, 255, 255, 255, 0, 0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	f.Add([]byte{3, 6, 6, 3, 30, 30, 20, 13, 255, 1, 255, 1, 0, 0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	f.Add([]byte{4, 12, 3, 3, 30, 32, 4, 13, 90, 166, 90, 166, 0, 0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	f.Add([]byte{5, 6, 6, 3, 30, 30, 20, 13, 255, 1, 255, 1, 0, 0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	f.Add([]byte{6, 6, 6, 5, 30, 30, 20, 13, 255, 1, 255, 1, 0, 0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 24 {
			return
		}
		kind := int(data[0]) % 7 // the six filters, then the widening
		w, h := int(data[1])%80, int(data[2])%80
		dStride, sStride := int(data[4])%100, int(data[5])%100
		sOff := int(binary.LittleEndian.Uint16(data[6:8])) % 8192
		var c8 [8]int16
		for i := range c8 {
			c8[i] = int16(int8(data[8+i]))
		}
		body := data[16:]
		ns := len(body) * 16
		src := make([]uint8, ns)
		for i := range src {
			src[i] = body[i%len(body)] + uint8(i/len(body))
		}
		dOff, so := int(data[6])%16, sOff%ns
		if kind == 6 {
			shift := int(data[3]) % 18
			dst, want := make([]int32, len(body)*8), make([]int32, len(body)*8)
			for i := range dst {
				dst[i], want[i] = sentinel16, sentinel16
			}
			if err := TryWidenBytes(dst, dOff, dStride, src, so, sStride, w, h, shift); err != nil {
				if !errors.Is(err, ErrRefused) {
					t.Fatalf("an error that is not a refusal: %v", err)
				}
				return
			}
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					want[dOff+y*dStride+x] = int32(src[so+y*sStride+x]) << uint(shift)
				}
			}
			for i := range dst {
				if dst[i] != want[i] {
					t.Fatalf("element %d is %d, the text gives %d", i, dst[i], want[i])
				}
			}
			return
		}
		b := byteKernels[kind]
		n, _, _, _ := b.geometry()
		c := c8[:n]
		d, wd := b.newDst(len(body)*8), b.newDst(len(body)*8)
		if err := b.run(d, dOff, dStride, src, so, sStride, w, h, c); err != nil {
			if !errors.Is(err, ErrRefused) {
				t.Fatalf("an error that is not a refusal: %v", err)
			}
			return
		}
		set := func(i int, v int32) {
			if wd.d16 != nil {
				wd.d16[i] = int16(v)
			} else {
				wd.d32[i] = v
			}
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				set(dOff+y*dStride+x, b.model(src, so+y*sStride+x, sStride, c))
			}
		}
		for i := 0; i < d.len(); i++ {
			if d.at(i) != wd.at(i) {
				t.Fatalf("%s: element %d is %d, the text gives %d", b.name, i, d.at(i), wd.at(i))
			}
		}
	})
}
