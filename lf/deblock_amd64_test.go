//go:build amd64 && !purego

package lf

import (
	"os"
	"testing"

	"github.com/go-videodsp/kernels/cpu"
)

// The dispatch must reach the assembly where AVX2 exists, or every deblocking
// test would pass on the portable code. On a GitHub runner (all have AVX2) a
// missing AVX2 is a failure, never a skip. Each of the eight kernels (luma and
// chroma, normal and strong, vertical and horizontal) is run on a plane the
// filter changes, against its reference.
func TestDispatchIsAVX2(t *testing.T) {
	required := os.Getenv("GITHUB_ACTIONS") == "true" || os.Getenv("VIDEODSP_REQUIRE_AVX2") == "1"
	if !cpu.HasAVX2 {
		if required {
			t.Fatal("AVX2 is required here and the cpu probe says no")
		}
		t.Skip("no AVX2 on this host: the portable path is what runs")
	}
	type edge func(pix []byte, vertical bool)
	bS := [4]uint8{1, 2, 3, 0}
	kernels := map[string][2]edge{
		"luma normal": {
			func(p []byte, v bool) { deblockNormalAVX2(p, 8*40+8, 40, v, 40, 30, bS) },
			func(p []byte, v bool) { deblockNormalRef(p, 8*40+8, 40, v, 40, 30, bS) },
		},
		"luma strong": {
			func(p []byte, v bool) { deblockStrongAVX2(p, 8*40+8, 40, v, 40, 30) },
			func(p []byte, v bool) { deblockStrongRef(p, 8*40+8, 40, v, 40, 30) },
		},
		"chroma normal": {
			func(p []byte, v bool) { deblockChromaNormalAVX2(p, 8*40+8, 40, v, 40, 30, bS) },
			func(p []byte, v bool) { deblockChromaNormalRef(p, 8*40+8, 40, v, 40, 30, bS) },
		},
		"chroma strong": {
			func(p []byte, v bool) { deblockChromaStrongAVX2(p, 8*40+8, 40, v, 40, 30) },
			func(p []byte, v bool) { deblockChromaStrongRef(p, 8*40+8, 40, v, 40, 30) },
		},
	}
	for name, k := range kernels {
		for _, vertical := range []bool{false, true} {
			orig := make([]byte, 40*40)
			for i := range orig {
				orig[i] = byte(100 + (i*3)%7)
			}
			got := append([]byte(nil), orig...)
			want := append([]byte(nil), orig...)
			k[0](got, vertical)
			k[1](want, vertical)
			changed := false
			for i := range got {
				if got[i] != want[i] {
					t.Fatalf("%s vertical=%v: AVX2 differs from the reference at %d", name, vertical, i)
				}
				changed = changed || got[i] != orig[i]
			}
			if !changed {
				t.Fatalf("%s vertical=%v: the case filters nothing, it proves nothing", name, vertical)
			}
		}
	}
}
