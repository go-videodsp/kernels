package cabac

import (
	"errors"
	"math/rand"
	"reflect"
	"testing"
)

// scanKinds are the shapes the H.264 categories use: 4x4 blocks (15 coded
// positions), AC blocks (14), chroma DC of 4:2:0 and 4:2:2 (3 and 7, with the
// context increment clipped to 2) and the 8x8 block (63, with the two tables of
// Table 9-43).
func mustScan(t testing.TB, nsig int, kind int, levelBase, limit int) *Scan {
	t.Helper()
	sig := make([]uint16, nsig)
	last := make([]uint16, nsig)
	for i := range sig {
		switch kind {
		case 0: // one context per position
			sig[i], last[i] = uint16(105+i), uint16(166+i)
		case 1: // chroma DC
			sig[i], last[i] = uint16(105+44+min(i/2, 2)), uint16(166+44+min(i/2, 2))
		default: // clustered, as the 8x8 tables are
			sig[i], last[i] = uint16(402+i%15), uint16(417+i%9)
		}
	}
	s, err := NewScan(sig, last, levelBase, limit)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func newState(data []byte, value uint32) State {
	st := State{Data: data, Rng: 510, BitsNeeded: -8}
	if len(data) >= 2 {
		st.Value = uint32(data[0])<<8 | uint32(data[1])
		st.Ptr = 2
	} else {
		st.Ptr = len(data)
	}
	if value != 0 {
		st.Value = value
	}
	return st
}

// compare runs the assembly (Decode) and the reference on copies of the same
// input and fails on the first difference in coefficients, count, state or
// contexts.
func compare(t testing.TB, data []byte, ctx0 *Contexts, value uint32, s *Scan) (overflow bool) {
	t.Helper()
	sa, sr := newState(data, value), newState(data, value)
	ca, cr := *ctx0, *ctx0
	coa := make([]int32, s.Coefficients())
	cor := make([]int32, s.Coefficients())
	na := Decode(&sa, &ca, s, coa)
	nr := decodeRef(&sr, &cr, s, cor)
	switch {
	case na != nr:
		t.Fatalf("count %d, reference %d (data %x, scan %d)", na, nr, data, s.nsig)
	case !reflect.DeepEqual(coa, cor):
		t.Fatalf("coefficients differ\n asm %v\n ref %v\n data %x", coa, cor, data)
	case sa.Ptr != sr.Ptr || sa.Value != sr.Value || sa.Rng != sr.Rng || sa.BitsNeeded != sr.BitsNeeded || sa.Overflow != sr.Overflow:
		t.Fatalf("state %+v, reference %+v (data %x)", sa, sr, data)
	case ca != cr:
		t.Fatalf("contexts differ (data %x)", data)
	}
	return sr.Overflow != 0
}

func randomContexts(rng *rand.Rand, wide bool) *Contexts {
	var c Contexts
	for i := range c {
		if wide {
			c[i] = uint8(rng.Intn(256))
		} else {
			c[i] = uint8(rng.Intn(126)) // pStateIdx 0..62
		}
	}
	return &c
}

func TestDecodeMatchesReference(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	scans := []*Scan{
		mustScan(t, 15, 0, 227+20, 4),
		mustScan(t, 14, 0, 227+10, 4),
		mustScan(t, 3, 1, 227+30, 3),
		mustScan(t, 7, 1, 227+30, 3),
		mustScan(t, 63, 2, 426, 4),
	}
	for round := 0; round < 20000; round++ {
		n := rng.Intn(40) // short streams run off the end: zeros are read there
		data := make([]byte, n)
		rng.Read(data)
		compare(t, data, randomContexts(rng, round%7 == 0), 0, scans[round%len(scans)])
	}
}

// TestLevelOverflowIsClampedAndFlagged finds, among states with a large offset
// (every bypass bin of an Exp-Golomb suffix reads 1), blocks whose level
// exceeds MaxLevel, and holds the assembly to the reference on them.
func TestLevelOverflowIsClampedAndFlagged(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	s := mustScan(t, 15, 0, 247, 4)
	// A valid offset is below Rng<<7 = 65280. Just under it, every bypass bin
	// reads 1 (15 of them), and with valMPS 0 every context bin is a least
	// probable symbol, which reads 1.
	mk := func() (*Contexts, uint32) {
		ctx := randomContexts(rng, false)
		for i := range ctx {
			// high states take many least probable symbols in a row
			// before the state reaches 0 and valMPS flips
			ctx[i] = uint8(50+rng.Intn(13)) << 1
		}
		return ctx, 65279 - uint32(rng.Intn(8))
	}
	seen := 0
	for round := 0; round < 400000 && seen < 5; round++ {
		data := make([]byte, 24)
		for i := range data {
			data[i] = 0xff
		}
		ctx, v := mk()
		if compare(t, data, ctx, v, s) {
			seen++
		}
	}
	if seen == 0 {
		t.Fatal("no block exceeded MaxLevel: the clamp is not exercised")
	}
	// the clamp itself
	var found bool
	for round := 0; round < 400000 && !found; round++ {
		data := make([]byte, 24)
		for i := range data {
			data[i] = 0xff
		}
		ctx, v := mk()
		st := newState(data, v)
		co := make([]int32, s.Coefficients())
		c2 := *ctx
		n := decodeRef(&st, &c2, s, co)
		if st.Overflow != 0 {
			found = true
			var hit bool
			for _, v := range co {
				if v == MaxLevel || v == -MaxLevel {
					hit = true
				}
				if v > MaxLevel || v < -MaxLevel {
					t.Fatalf("level %d beyond the clamp (n=%d)", v, n)
				}
			}
			if !hit {
				t.Fatal("overflow flagged but no coefficient was clamped")
			}
		}
	}
	if !found {
		t.Fatal("overflow never flagged")
	}
}

func TestDecodeNeverReadsPastTheEnd(t *testing.T) {
	// A stream shorter than the block needs decodes zeros past its end; the
	// bytes after the slice's end are never read. Put a poisoned region after
	// the slice (a sub-slice of a larger array) and require the result not to
	// depend on it.
	rng := rand.New(rand.NewSource(3))
	s := mustScan(t, 15, 0, 247, 4)
	for round := 0; round < 5000; round++ {
		n := rng.Intn(12)
		buf := make([]byte, n+64)
		rng.Read(buf)
		ctx := randomContexts(rng, false)
		run := func(fill byte) (State, []int32) {
			b := append([]byte(nil), buf...)
			for i := n; i < len(b); i++ {
				b[i] = fill
			}
			st := newState(b[:n:n], 0)
			co := make([]int32, s.Coefficients())
			c := *ctx
			Decode(&st, &c, s, co)
			st.Data = nil
			return st, co
		}
		s0, c0 := run(0x00)
		s1, c1 := run(0xff)
		if !reflect.DeepEqual(s0, s1) || !reflect.DeepEqual(c0, c1) {
			t.Fatalf("the result depends on bytes past the end of a %d-byte stream", n)
		}
	}
}

func TestNewScanRefusesWhatWouldLeaveTheContextArray(t *testing.T) {
	ok := []uint16{1, 2, 3}
	cases := []struct {
		name       string
		sig, last  []uint16
		base, lim  int
		wantRefuse bool
	}{
		{"valid", ok, ok, 227, 4, false},
		{"empty", nil, nil, 227, 4, true},
		{"length mismatch", ok, ok[:2], 227, 4, true},
		{"sixty-four positions", make([]uint16, 64), make([]uint16, 64), 227, 4, true},
		{"sixty-three positions", make([]uint16, 63), make([]uint16, 63), 227, 4, false},
		{"sig index 1024", []uint16{1024}, []uint16{0}, 227, 4, true},
		{"last index 1024", []uint16{0}, []uint16{1024}, 227, 4, true},
		{"sig index 1023", []uint16{1023}, []uint16{1023}, 227, 4, false},
		{"base zero", ok, ok, 0, 4, false},
		{"limit zero", ok, ok, 227, 0, false},
		{"negative base", ok, ok, -1, 4, true},
		{"negative limit", ok, ok, 227, -1, true},
		{"limit above four", ok, ok, 227, 5, true},
		{"top level context at 1023", ok, ok, 1023 - 9, 4, false},
		{"top level context at 1024", ok, ok, 1024 - 9, 4, true},
	}
	for _, c := range cases {
		_, err := NewScan(c.sig, c.last, c.base, c.lim)
		if (err != nil) != c.wantRefuse {
			t.Errorf("%s: err = %v, refuse = %v", c.name, err, c.wantRefuse)
		}
		if err != nil && !errors.Is(err, errScan) {
			t.Errorf("%s: %v does not wrap errScan", c.name, err)
		}
	}
}

func TestDecodeRefusesAShortCoefficientBuffer(t *testing.T) {
	s := mustScan(t, 15, 0, 247, 4)
	st := newState(make([]byte, 8), 0)
	var c Contexts
	for _, n := range []int{0, 15} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("a buffer of %d for 16 coefficients was accepted", n)
				}
			}()
			Decode(&st, &c, s, make([]int32, n))
		}()
	}
	if got := Decode(&st, &c, s, make([]int32, 16)); got < 1 {
		t.Fatalf("a block decodes at least one coefficient, got %d", got)
	}
}

func TestEngineTablesAreTheSpecsAtEveryContextByte(t *testing.T) {
	for s := 0; s < 256; s++ {
		state, mps := (s>>1)&63, uint8(s&1)
		for q := 0; q < 4; q++ {
			if tabs.lps[s][q] != rangeTabLPS[state][q] {
				t.Fatalf("lps[%d][%d]", s, q)
			}
		}
		if tabs.nextMPS[s] != transIdxMPS[state]<<1|mps {
			t.Fatalf("nextMPS[%d]", s)
		}
		wantMPS := mps
		if state == 0 {
			wantMPS = 1 - mps
		}
		if tabs.nextLPS[s] != transIdxLPS[state]<<1|wantMPS {
			t.Fatalf("nextLPS[%d]", s)
		}
	}
	// the shift brings every lps the decoder can meet to at least 256 in the
	// fewest doublings
	for state := 0; state < 63; state++ {
		for q := 0; q < 4; q++ {
			lps := uint32(rangeTabLPS[state][q])
			n := uint(tabs.shift[lps>>3])
			if lps<<n < 256 || lps<<(n-1) >= 256 {
				t.Fatalf("shift %d for lps %d", n, lps)
			}
		}
	}
}
