//go:build amd64 && !purego

package mc

import (
	"bytes"
	"math/rand"
	"os"
	"testing"

	"github.com/go-videodsp/kernels/cpu"
)

// The dispatch must reach the assembly where AVX2 exists, or every chroma test
// would pass on the portable code. On a GitHub runner a missing AVX2 is a
// failure, never a skip.
func TestChromaDispatchIsAVX2(t *testing.T) {
	required := os.Getenv("GITHUB_ACTIONS") == "true" || os.Getenv("VIDEODSP_REQUIRE_AVX2") == "1"
	if !cpu.HasAVX2 {
		if required {
			t.Fatal("AVX2 is required here and the cpu probe says no")
		}
		t.Skip("no AVX2 on this host: the portable path is what runs")
	}
	r := rand.New(rand.NewSource(5))
	for _, sz := range chromaSizes {
		w, h := sz[0], sz[1]
		src, dst := chromaPlane(r, 16*16), chromaPlane(r, 16*16)
		want := append([]byte(nil), dst...)
		putChromaAVX2(dst, dst, 0, 16, src, src, 0, 16, w, h, 5, 3, 1, false)
		putChromaRef(want, 0, 16, src, 0, 16, w, h, 5, 3)
		if !bytes.Equal(dst, want) {
			t.Fatalf("%dx%d: AVX2 differs from the reference at %d", w, h, chromaDiff(dst, want))
		}
	}
}
