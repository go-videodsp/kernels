package hevc

import (
	"encoding/binary"
	"errors"
	"testing"
)

// FuzzPasses drives both passes with parameters taken from the bytes: sizes, strides,
// offsets and slice lengths that are often wrong. It holds them to two things: a
// call never panics, and a call that is accepted gives exactly the reference's
// result and writes nothing outside its block (the rest of dst keeps its sentinel).
//
//	go test -run='^$' -fuzz='^FuzzPasses$' -fuzztime=60s ./hevc
func FuzzPasses(f *testing.F) {
	f.Add([]byte{0, 8, 4, 8, 8, 12, 20, 3, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23})
	f.Add([]byte{1, 64, 64, 12, 71, 72, 200, 255, 255, 255, 255, 0, 0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8})
	f.Add([]byte{0, 3, 3, 13, 0, 0, 1, 1, 1, 1})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 9 { // the parameters, and at least one byte of samples
			return
		}
		vertical := data[0]&1 == 1
		w, h, bd := int(data[1])%80, int(data[2])%80, int(data[3])%16
		dstStride, srcStride := int(data[4])%100, int(data[5])%100
		srcOff := int(binary.LittleEndian.Uint16(data[6:8])) % 8192
		body := data[8:]
		c := lumaFilters[int(data[0]>>1)%3]
		nSrc, nDst := len(body)*8, len(body)*4 // slice lengths that follow the input
		if vertical {
			src := make([]int16, nSrc)
			for i := range src {
				src[i] = int16(binary.LittleEndian.Uint16([]byte{body[i%len(body)], body[(i+1)%len(body)]})) % 22528
			}
			dst, want := make([]int32, nDst), make([]int32, nDst)
			err := TryVPass8(dst, int(data[6])%16, dstStride, src, srcOff%nSrc, srcStride, w, h, &c)
			if err != nil {
				if !errors.Is(err, ErrRefused) {
					t.Fatalf("an error that is not a refusal: %v", err)
				}
				return
			}
			vpassRef(want, int(data[6])%16, dstStride, src, srcOff%nSrc, srcStride, w, h, &c)
			for i := range dst {
				if dst[i] != want[i] {
					t.Fatalf("element %d is %d, the reference gives %d", i, dst[i], want[i])
				}
			}
			return
		}
		src := make([]uint16, nSrc)
		for i := range src {
			src[i] = uint16(body[i%len(body)]) << uint(bd%5) & uint16(1<<12-1)
		}
		dst, want := make([]int16, nDst), make([]int16, nDst)
		for i := range dst {
			dst[i], want[i] = sentinel16, sentinel16
		}
		err := TryHPass8(dst, int(data[6])%16, dstStride, src, srcOff%nSrc, srcStride, w, h, bd, &c)
		if err != nil {
			if !errors.Is(err, ErrRefused) {
				t.Fatalf("an error that is not a refusal: %v", err)
			}
			return
		}
		hpassRef(want, int(data[6])%16, dstStride, src, srcOff%nSrc, srcStride, w, h, uint(bd-8), &c)
		for i := range dst {
			if dst[i] != want[i] {
				t.Fatalf("element %d is %d, the reference gives %d", i, dst[i], want[i])
			}
		}
	})
}
