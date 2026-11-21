package cabac

import (
	"errors"
	"fmt"
)

// NumContexts is the number of context variables of H.264 CABAC.
const NumContexts = 1024

// Contexts holds pStateIdx<<1 | valMPS of every context variable.
type Contexts = [NumContexts]uint8

// State is the arithmetic decoding engine of 9.3.1.2 and 9.3.3.2 with the
// offset scaled by 2^7 and seven bits of lookahead below it, refilled a byte
// at a time: Value is codIOffset<<7 | lookahead, a decision compares it with
// Rng<<7, and a renormalisation is a shift. The assembly addresses the fields
// by offset: Data at 0, Ptr 24, Value 32, Rng 36, BitsNeeded 40, Overflow 48.
type State struct {
	Data       []byte // the stream; read only below len(Data)
	Ptr        int    // the next byte to fetch, counting those past the end
	Value      uint32 // codIOffset<<7 | lookahead
	Rng        uint32 // codIRange
	BitsNeeded int    // -8..-1: shifts left before the next byte is due, minus eight
	Overflow   uint32 // set to 1 when a level exceeded MaxLevel (and was clamped)
}

// MaxLevel is the largest coefficient magnitude Decode returns; a larger one
// is clamped and sets State.Overflow.
const MaxLevel = 32767

// Scan is what a residual block category decodes with: the context index of
// the significance and of the last flag at every position of the map, and the
// base of the coefficient-level contexts. It is immutable and shared.
type Scan struct {
	sig, last  [63]uint16
	nsig       int // positions coded in the map: the block has nsig+1 coefficients
	levelBase  int
	levelLimit int
}

var errScan = errors.New("cabac: invalid scan")

// NewScan checks and copies a category's tables. sig and last give the context
// index of the significant_coeff_flag and the last_significant_coeff_flag of
// each of the first len(sig) positions (the last position of the block is not
// coded: it is significant when no last flag was set before it, so a block has
// len(sig)+1 coefficients, at most 64). levelBase is the first context of
// coeff_abs_level_minus1 for the category and levelLimit the largest increment
// of its later bins (9.3.3.1.3: 4, or 3 for chroma DC).
func NewScan(sig, last []uint16, levelBase, levelLimit int) (*Scan, error) {
	if len(sig) < 1 || len(sig) > 63 || len(last) != len(sig) {
		return nil, fmt.Errorf("%w: %d significance and %d last contexts", errScan, len(sig), len(last))
	}
	if levelBase < 0 || levelLimit < 0 || levelLimit > 4 || levelBase+5+levelLimit >= NumContexts {
		return nil, fmt.Errorf("%w: level contexts %d..%d outside 0..%d", errScan, levelBase, levelBase+5+levelLimit, NumContexts-1)
	}
	s := &Scan{nsig: len(sig), levelBase: levelBase, levelLimit: levelLimit}
	for i := range sig {
		if sig[i] >= NumContexts || last[i] >= NumContexts {
			return nil, fmt.Errorf("%w: context index at position %d outside 0..%d", errScan, i, NumContexts-1)
		}
		s.sig[i], s.last[i] = sig[i], last[i]
	}
	return s, nil
}

// Coefficients is the number of coefficients of a block of this scan.
func (s *Scan) Coefficients() int { return s.nsig + 1 }

// Decode decodes one residual block whose coded_block_flag was 1: it writes
// the signed levels of the significant coefficients into coeffs, which the
// caller has zeroed and which holds at least Coefficients() entries, and returns
// how many there are. It never reads Data at or above len(Data).
func Decode(st *State, ctx *Contexts, s *Scan, coeffs []int32) int {
	if len(coeffs) < s.nsig+1 {
		panic("cabac: coefficient buffer shorter than the block")
	}
	return decode(st, ctx, s, coeffs)
}
