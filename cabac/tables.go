package cabac

// A context is one byte, pStateIdx<<1 | valMPS. The engine reads three tables
// by that byte and one by the width of the least probable subrange. The tables
// are laid out in one block, in this order, which the assembly addresses by
// offset from the base of tabs (see internal/gencabac):
//
//	lps      0     256 contexts x 4 qCodIRangeIdx
//	nextMPS  1024  the context byte after a most probable symbol
//	nextLPS  1280  the context byte after a least probable one
//	shift    1536  the renormalisation shift of a least probable symbol, by lps>>3
//
// All 256 values of a context byte index them, so a context array holding a
// byte above 127 (which the decoder never writes) reads a defined entry, the
// one of its low seven bits, and never reads outside the table.
type engineTables struct {
	lps     [256][4]uint8
	nextMPS [256]uint8
	nextLPS [256]uint8
	shift   [32]uint8
}

var tabs = engineTables{
	// the number of doublings that bring the new range to at least 256, by
	// rangeTabLPS >> 3 (6 for 6 and 7, 5 for 8..15, ... 1 for 128..255)
	shift: [32]uint8{
		6, 5, 4, 4, 3, 3, 3, 3,
		2, 2, 2, 2, 2, 2, 2, 2,
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	},
}

func init() {
	for s := 0; s < 256; s++ {
		state, mps := (s>>1)&63, s&1
		tabs.lps[s] = rangeTabLPS[state]
		tabs.nextMPS[s] = transIdxMPS[state]<<1 | uint8(mps)
		flipped := mps
		if state == 0 {
			flipped = 1 - mps
		}
		tabs.nextLPS[s] = transIdxLPS[state]<<1 | uint8(flipped)
	}
}
