package cabac

// fetchRef ORs the next byte (zero past the end) into the value at shift.
func (st *State) fetchRef(shift uint) {
	if st.Ptr < len(st.Data) {
		st.Value |= uint32(st.Data[st.Ptr]) << shift
	}
	st.Ptr++
}

// DecideRef is DecodeDecision of 9.3.3.2.1 and its RenormD for one context.
func (st *State) DecideRef(ctx *Contexts, idx int) uint32 {
	s := ctx[idx]
	lps := uint32(tabs.lps[s][st.Rng>>6&3])
	st.Rng -= lps
	scaled := st.Rng << 7
	if st.Value < scaled {
		ctx[idx] = tabs.nextMPS[s]
		// Rng is below 512: it fell below 256 exactly when bit 8 is clear, and
		// then one doubling restores it.
		sh := uint(^st.Rng>>8) & 1
		st.Rng <<= sh
		st.Value <<= sh
		st.BitsNeeded += int(sh)
		if st.BitsNeeded >= 0 {
			st.fetchRef(0)
			st.BitsNeeded = -8
		}
		return uint32(s & 1)
	}
	n := uint(tabs.shift[lps>>3])
	st.Value = (st.Value - scaled) << n
	st.Rng = lps << n
	ctx[idx] = tabs.nextLPS[s]
	st.BitsNeeded += int(n)
	if st.BitsNeeded >= 0 {
		st.fetchRef(uint(st.BitsNeeded))
		st.BitsNeeded -= 8
	}
	return uint32(s&1) ^ 1
}

// BypassRef is DecodeBypass of 9.3.3.2.3.
func (st *State) BypassRef() uint32 {
	st.Value <<= 1
	st.BitsNeeded++
	if st.BitsNeeded >= 0 {
		st.fetchRef(0)
		st.BitsNeeded = -8
	}
	scaled := st.Rng << 7
	if st.Value >= scaled {
		st.Value -= scaled
		return 1
	}
	return 0
}

// absLevelMinus1Ref is coeff_abs_level_minus1 (9.3.2.3 binarisation, UEG0 with
// a prefix of 14 and the contexts of 9.3.3.1.3).
func (st *State) absLevelMinus1Ref(ctx *Contexts, base, limit, numGt1, numEq1 int) int {
	prefix := 0
	for prefix < 14 {
		var inc int
		switch {
		case prefix > 0:
			inc = 5 + min(numGt1, limit)
		case numGt1 != 0:
			inc = 0
		default:
			inc = min(1+numEq1, 4)
		}
		if st.DecideRef(ctx, base+inc) == 0 {
			break
		}
		prefix++
	}
	if prefix < 14 {
		return prefix
	}
	value := 14
	k := 0
	for st.BypassRef() == 1 {
		value += 1 << uint(k)
		k++
		if k > 24 {
			break
		}
	}
	for k--; k >= 0; k-- {
		value += int(st.BypassRef()) << uint(k)
	}
	return value
}

// decodeRef is the portable Decode and the specification of the assembly.
func decodeRef(st *State, ctx *Contexts, s *Scan, coeffs []int32) int {
	// the significant positions, in order, as the map is decoded
	var pos [64]uint8
	n := 0
	last := false
	for i := 0; i < s.nsig; i++ {
		if st.DecideRef(ctx, int(s.sig[i])) == 1 {
			pos[n] = uint8(i)
			n++
			if st.DecideRef(ctx, int(s.last[i])) == 1 {
				last = true
				break
			}
		}
	}
	if !last { // the last coefficient is significant without being coded
		pos[n] = uint8(s.nsig)
		n++
	}
	numGt1, numEq1 := 0, 0
	for k := n - 1; k >= 0; k-- {
		level := st.absLevelMinus1Ref(ctx, s.levelBase, s.levelLimit, numGt1, numEq1) + 1
		if level > MaxLevel {
			st.Overflow = 1
			level = MaxLevel
		}
		if level > 1 {
			numGt1++
		} else {
			numEq1++
		}
		if st.BypassRef() == 1 {
			level = -level
		}
		coeffs[pos[k]] = int32(level)
	}
	return n
}
