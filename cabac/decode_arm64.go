//go:build arm64 && !purego

package cabac

//go:noescape
func decodeAsm(st *State, ctx *byte, sig, last *uint16, nsig, levelBase, levelLimit int, coeffs *int32) int

func decode(st *State, ctx *Contexts, s *Scan, coeffs []int32) int {
	return decodeAsm(st, &ctx[0], &s.sig[0], &s.last[0], s.nsig, s.levelBase, s.levelLimit, &coeffs[0])
}
