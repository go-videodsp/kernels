package main

import (
	"bytes"
	"fmt"
	"testing"
)

func bitString(w *bitWriter) string {
	var s string
	for i, b := range w.buf {
		n := 8
		if i == len(w.buf)-1 && w.nbit != 0 {
			n = w.nbit
		}
		s += fmt.Sprintf("%08b", b)[:n]
	}
	return s
}

// Tables 9-2 and 9-3: the first codeNums of ue(v) and the signed mapping of
// se(v).
func TestExpGolomb(t *testing.T) {
	for _, c := range []struct {
		ue   uint32
		want string
	}{{0, "1"}, {1, "010"}, {2, "011"}, {3, "00100"}, {4, "00101"}, {7, "0001000"}} {
		var w bitWriter
		w.ue(c.ue)
		if got := bitString(&w); got != c.want {
			t.Errorf("ue(%d) = %s, want %s", c.ue, got, c.want)
		}
	}
	for _, c := range []struct {
		se   int
		want string
	}{{0, "1"}, {1, "010"}, {-1, "011"}, {2, "00100"}, {-2, "00101"}} {
		var w bitWriter
		w.se(c.se)
		if got := bitString(&w); got != c.want {
			t.Errorf("se(%d) = %s, want %s", c.se, got, c.want)
		}
	}
}

func TestTrailingAndAlign(t *testing.T) {
	var w bitWriter
	w.u(3, 0b101)
	w.trailing()
	if got := bitString(&w); got != "10110000" {
		t.Errorf("trailing bits: %s", got)
	}
	var a bitWriter
	a.bit(1)
	a.align()
	a.align() // already aligned
	if got := bitString(&a); got != "10000000" {
		t.Errorf("align: %s", got)
	}
}

// 7.4.1: 0x03 is inserted after two zero bytes when the next byte is 0..3.
func TestEmulationPrevention(t *testing.T) {
	got := nal(2, 1, []byte{0, 0, 1, 0, 0, 0, 0, 4, 0, 0, 5})
	want := []byte{0, 0, 0, 1, 2<<5 | 1, 0, 0, 3, 1, 0, 0, 3, 0, 0, 4, 0, 0, 5}
	if !bytes.Equal(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

// nalTypes splits an Annex B stream at its 4-byte start codes.
func nalTypes(s []byte) []int {
	var types []int
	for i := 0; i+4 < len(s); i++ {
		if s[i] == 0 && s[i+1] == 0 && s[i+2] == 0 && s[i+3] == 1 {
			types = append(types, int(s[i+4]&0x1f))
		}
	}
	return types
}

func TestStreamStructure(t *testing.T) {
	for _, w := range []*weights{nil, {5, 40, 3}} {
		p := params{wMB: 3, hMB: 2, frames: 4, mvx: 2, mvy: 3, weight: w}
		got := nalTypes(stream(p))
		want := []int{7, 8, 5, 1, 1, 1, 1}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("weights %v: NAL types %v, want %v", w, got, want)
		}
	}
	// The weighted PPS differs from the unweighted one in weighted_pred_flag.
	a := pps(params{wMB: 1, hMB: 1})
	b := pps(params{wMB: 1, hMB: 1, weight: &weights{}})
	if bytes.Equal(a, b) {
		t.Error("weighted_pred_flag is not written")
	}
	// A P picture with more frames than 16 wraps frame_num.
	if len(pic(params{wMB: 1, hMB: 1}, 17)) == 0 {
		t.Error("empty picture")
	}
}
