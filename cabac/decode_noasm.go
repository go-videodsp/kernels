//go:build (!amd64 && !arm64) || purego

package cabac

func decode(st *State, ctx *Contexts, s *Scan, coeffs []int32) int {
	return decodeRef(st, ctx, s, coeffs)
}
