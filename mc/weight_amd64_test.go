//go:build amd64 && !purego

package mc

import (
	"os"
	"testing"

	"github.com/go-videodsp/kernels/cpu"
)

// The dispatch must reach the assembly where AVX2 exists, or every weighted
// prediction test would pass on the portable code. On a GitHub runner (all
// have AVX2) a missing AVX2 is a failure, never a skip.
func TestWeightDispatchIsAVX2(t *testing.T) {
	required := os.Getenv("GITHUB_ACTIONS") == "true" || os.Getenv("VIDEODSP_REQUIRE_AVX2") == "1"
	if !cpu.HasAVX2 {
		if required {
			t.Fatal("AVX2 is required here and the cpu probe says no")
		}
		t.Skip("no AVX2 on this host: the portable path is what runs")
	}
	// Worst-case magnitudes: the largest products and the offset limits.
	for _, c := range []struct{ w0, w1, o0, o1, logWD int }{
		{128, 128, 127, 127, 0}, {-128, -128, -128, -128, 7}, {128, -128, 127, -128, 3}, {-64, 128, 5, -6, 5},
	} {
		for _, sz := range sizes {
			w, h := sz[0], sz[1]
			dst := make([]byte, w*h)
			src := make([]byte, w*h)
			for i := range dst {
				dst[i] = byte(i * 37)
				src[i] = byte(255 - i*11)
			}
			want := append([]byte(nil), dst...)
			weightBiAVX2(dst, 0, w, src, 0, w, w, h, c.w0, c.w1, c.o0, c.o1, c.logWD)
			weightBiRef(want, 0, w, src, 0, w, w, h, c.w0, c.w1, c.o0, c.o1, c.logWD)
			for i := range dst {
				if dst[i] != want[i] {
					t.Fatalf("bi %dx%d %+v: AVX2 differs from the reference at %d", w, h, c, i)
				}
			}
			weightUniAVX2(dst, 0, w, w, h, c.w0, c.o0, c.logWD)
			weightUniRef(want, 0, w, w, h, c.w0, c.o0, c.logWD)
			for i := range dst {
				if dst[i] != want[i] {
					t.Fatalf("uni %dx%d %+v: AVX2 differs from the reference at %d", w, h, c, i)
				}
			}
		}
	}
}
