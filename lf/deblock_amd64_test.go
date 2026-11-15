//go:build amd64 && !purego

package lf

import (
	"os"
	"testing"

	"github.com/go-videodsp/kernels/cpu"
)

// The dispatch must reach the assembly where AVX2 exists, or every deblocking
// test would pass on the portable code. On a GitHub runner (all have AVX2) a
// missing AVX2 is a failure, never a skip.
func TestDispatchIsAVX2(t *testing.T) {
	required := os.Getenv("GITHUB_ACTIONS") == "true" || os.Getenv("VIDEODSP_REQUIRE_AVX2") == "1"
	if !cpu.HasAVX2 {
		if required {
			t.Fatal("AVX2 is required here and the cpu probe says no")
		}
		t.Skip("no AVX2 on this host: the portable path is what runs")
	}
	for _, vertical := range []bool{false, true} {
		for _, strong := range []bool{false, true} {
			got := make([]byte, 40*40)
			for i := range got {
				got[i] = byte(100 + (i*3)%7)
			}
			want := append([]byte(nil), got...)
			off := 8*40 + 8
			if strong {
				deblockStrongAVX2(got, off, 40, vertical, 40, 30)
				deblockStrongRef(want, off, 40, vertical, 40, 30)
			} else {
				deblockNormalAVX2(got, off, 40, vertical, 40, 30, [4]uint8{1, 2, 3, 0})
				deblockNormalRef(want, off, 40, vertical, 40, 30, [4]uint8{1, 2, 3, 0})
			}
			changed := false
			for i := range got {
				if got[i] != want[i] {
					t.Fatalf("vertical=%v strong=%v: AVX2 differs from the reference at %d", vertical, strong, i)
				}
				changed = changed || got[i] != byte(100+(i*3)%7)
			}
			if !changed {
				t.Fatalf("vertical=%v strong=%v: the case filters nothing, it proves nothing", vertical, strong)
			}
		}
	}
}
