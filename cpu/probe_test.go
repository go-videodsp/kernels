package cpu

import "os"

// probeRequired is true where the host is known to implement AVX2: GitHub
// Actions runners (every amd64 hosted runner does) and any host that sets
// VIDEODSP_REQUIRE_AVX2=1.
func probeRequired() bool {
	return os.Getenv("GITHUB_ACTIONS") == "true" || os.Getenv("VIDEODSP_REQUIRE_AVX2") == "1"
}
