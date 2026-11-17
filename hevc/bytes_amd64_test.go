//go:build amd64 && !purego

package hevc

import "testing"

// Without AVX2 the byte-source kernels run the portable reference: the same results, by the
// same entry points.
func TestBytesPortablePathWithoutAVX2(t *testing.T) {
	old := useAVX2
	defer func() { useAVX2 = old }()
	useAVX2 = false
	checkBytesAgainstTheText(t, 75, 600)
	src := make([]uint8, 400)
	dst := make([]int32, 64)
	if err := TryWidenBytes(dst, 0, 8, src, 0, 8, 8, 4, 2); err != nil {
		t.Fatal(err)
	}
}
