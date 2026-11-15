package cpu

import (
	"runtime"
	"testing"
)

// On a machine that runs the amd64 kernels the probe must say so: a probe that
// silently answered false would leave every dispatch on the portable path with
// all tests green. GitHub's amd64 runners all implement AVX2.
func TestProbeOnAmd64(t *testing.T) {
	t.Logf("GOARCH=%s AVX2=%v AVX512=%v", runtime.GOARCH, HasAVX2, HasAVX512)
	if HasAVX512 && !HasAVX2 {
		t.Fatal("AVX-512 without AVX2")
	}
	if runtime.GOARCH == "amd64" && probeRequired() && !HasAVX2 {
		t.Fatal("this runner is required to have AVX2 and the probe says no")
	}
}
