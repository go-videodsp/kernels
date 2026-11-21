package cabac

import (
	"math/rand"
	"testing"
)

// FuzzDecode holds the assembly to the reference, bin for bin, on any stream
// (including one shorter than the block needs), any contexts and any scan. The
// bytes pick the scan shape, the context seed and the stream.
func FuzzDecode(f *testing.F) {
	f.Add([]byte{0, 1, 0, 0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0})
	f.Add([]byte{2, 7, 1})
	f.Add([]byte{4, 3, 1, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
	f.Add([]byte{3, 0, 0})
	scans := []*Scan{
		mustScan(f, 15, 0, 247, 4),
		mustScan(f, 14, 0, 237, 4),
		mustScan(f, 3, 1, 257, 3),
		mustScan(f, 7, 1, 257, 3),
		mustScan(f, 63, 2, 426, 4),
	}
	f.Fuzz(func(t *testing.T, in []byte) {
		if len(in) < 3 {
			return
		}
		s := scans[int(in[0])%len(scans)]
		rng := rand.New(rand.NewSource(int64(in[1])))
		ctx := randomContexts(rng, in[2]&1 == 1)
		compare(t, in[3:], ctx, 0, s)
	})
}
