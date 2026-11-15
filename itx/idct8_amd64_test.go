//go:build amd64 && !purego

package itx

import (
	"bytes"
	"math/rand"
	"os"
	"testing"

	"github.com/go-videodsp/kernels/cpu"
)

// The dispatch must reach the assembly where AVX2 exists, or every test would
// pass on the portable code. On a GitHub runner (all have AVX2) a missing AVX2
// is a failure, never a skip.
func TestIDCT8DispatchIsAVX2(t *testing.T) {
	required := os.Getenv("GITHUB_ACTIONS") == "true" || os.Getenv("VIDEODSP_REQUIRE_AVX2") == "1"
	if !cpu.HasAVX2 {
		if required {
			t.Fatal("AVX2 is required here and the cpu probe says no")
		}
		t.Skip("no AVX2 on this host: the portable path is what runs")
	}
	r := rand.New(rand.NewSource(9))
	for k := blockKind(0); k < numKinds; k++ {
		coef, scale := randomBlock(r, k)
		got := randomPlane(r, 8*16)
		want := append([]byte(nil), got...)
		dequant8x8IDCTAddAVX2(got, 0, 16, &coef, &scale, 4)
		dequant8x8IDCTAddRef(want, 0, 16, &coef, &scale, 4)
		if !bytes.Equal(got, want) {
			t.Fatalf("kind %d: AVX2 differs from the reference at %d", k, firstDiff(got, want))
		}
	}
}
