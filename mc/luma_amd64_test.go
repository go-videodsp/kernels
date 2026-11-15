//go:build amd64 && !purego

package mc

import (
	"os"
	"testing"

	"github.com/go-videodsp/kernels/cpu"
)

// The dispatch must reach the assembly where AVX2 exists: otherwise every
// test of this package would pass on the portable code. On a GitHub runner
// (all have AVX2) a missing AVX2 is a failure, never a skip.
func TestDispatchIsAVX2(t *testing.T) {
	required := os.Getenv("GITHUB_ACTIONS") == "true" || os.Getenv("VIDEODSP_REQUIRE_AVX2") == "1"
	if !cpu.HasAVX2 {
		if required {
			t.Fatal("AVX2 is required here and the cpu probe says no")
		}
		t.Skip("no AVX2 on this host: the portable path is what runs")
	}
	src := make([]byte, 40*40)
	for i := range src {
		src[i] = byte(i * 7)
	}
	got := make([]byte, 16*16)
	want := append([]byte(nil), got...)
	lumaAVX2(got, 0, 16, src, 3*40+3, 40, 16, 16, 2, 2, false)
	lumaRef(want, 0, 16, src, 3*40+3, 40, 16, 16, 2, 2, false)
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("lumaAVX2 differs from the reference at %d", i)
		}
	}
}
