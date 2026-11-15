//go:build amd64 && !purego

package itx

import (
	"bytes"
	"math/rand"
	"os"
	"testing"

	"github.com/go-videodsp/kernels/cpu"
)

// The dispatch must reach the assembly where AVX2 exists. On a GitHub runner
// a missing AVX2 is a failure, never a skip.
func TestIDCT4DispatchIsAVX2(t *testing.T) {
	required := os.Getenv("GITHUB_ACTIONS") == "true" || os.Getenv("VIDEODSP_REQUIRE_AVX2") == "1"
	if !cpu.HasAVX2 {
		if required {
			t.Fatal("AVX2 is required here and the cpu probe says no")
		}
		t.Skip("no AVX2 on this host: the portable path is what runs")
	}
	r := rand.New(rand.NewSource(8))
	coef, scale := randomBlocks(r, kindDense)
	got := randomPlane(r, 16*32)
	want := append([]byte(nil), got...)
	dequant4x4IDCTAddBlocksAVX2(got, 0, 32, &coef, 0xffff, &scale, 4, true)
	dequant4x4IDCTAddBlocksRef(want, 0, 32, &coef, 0xffff, &scale, 4, true)
	if !bytes.Equal(got, want) {
		t.Fatalf("AVX2 differs from the reference at %d", firstDiff(got, want))
	}
	dc, wantDC := [16]int32{1, -2, 3, 4, 5, 6, -7, 8, 9, 10, 11, 12, 13, 14, 15, -16}, [16]int32{}
	wantDC = dc
	lumaDCDequantAVX2(&dc, 77, 3)
	lumaDCDequantRef(&wantDC, 77, 3)
	if dc != wantDC {
		t.Fatalf("luma DC: AVX2 %v, reference %v", dc, wantDC)
	}
}
