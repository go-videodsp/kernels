package lf

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
)

// runPair filters two random planes with different indexes per plane and
// compares each with the oracle.
func runPair(t *testing.T, r *rand.Rand, vertical bool, ia, ib, ja, jb int, bS [4]int, strong bool) {
	t.Helper()
	cb := make([]byte, planeStride*planeRows)
	cr := make([]byte, planeStride*planeRows)
	fillEdges(r, cb)
	fillEdges(r, cr)
	off := edgeOffset(vertical, r.Intn(4))
	wantCb := append([]byte(nil), cb...)
	wantCr := append([]byte(nil), cr...)
	oracleChromaEdge(wantCb, off, planeStride, vertical, ia, ib, bS)
	oracleChromaEdge(wantCr, off, planeStride, vertical, ja, jb, bS)
	var u [4]uint8
	for i := range u {
		u[i] = uint8(bS[i])
	}
	switch {
	case strong && vertical:
		DeblockChromaPairIntraV(cb, cr, off, planeStride, ia, ib, ja, jb)
	case strong:
		DeblockChromaPairIntraH(cb, cr, off, planeStride, ia, ib, ja, jb)
	case vertical:
		DeblockChromaPairV(cb, cr, off, planeStride, ia, ib, ja, jb, u)
	default:
		DeblockChromaPairH(cb, cr, off, planeStride, ia, ib, ja, jb, u)
	}
	if !bytes.Equal(cb, wantCb) || !bytes.Equal(cr, wantCr) {
		t.Fatalf("vertical=%v strong=%v cb (%d,%d) cr (%d,%d) bS %v: cb differs at %d, cr at %d", vertical, strong, ia, ib, ja, jb, bS, firstDiff(cb, wantCb), firstDiff(cr, wantCr))
	}
}

// Every (indexA, indexB) for Cb against a different, random pair for Cr, and
// the reverse; plus equal indexes.
func TestChromaPairExhaustive(t *testing.T) {
	r := rand.New(rand.NewSource(17))
	patterns := [][4]int{{1, 1, 1, 1}, {2, 2, 2, 2}, {3, 3, 3, 3}, {0, 0, 0, 0}, {0, 1, 2, 3}, {3, 2, 1, 0}, {1, 0, 3, 2}, {2, 3, 0, 1}}
	for _, vertical := range []bool{true, false} {
		t.Run(fmt.Sprintf("vertical=%v", vertical), func(t *testing.T) {
			for a := 0; a < 52; a++ {
				for b := 0; b < 52; b++ {
					ja, jb := r.Intn(52), r.Intn(52)
					for _, p := range patterns {
						runPair(t, r, vertical, a, b, a, b, p, false)   // equal
						runPair(t, r, vertical, a, b, ja, jb, p, false) // Cr differs
						runPair(t, r, vertical, ja, jb, a, b, p, false) // Cb differs
					}
					for i := 0; i < 2; i++ {
						runPair(t, r, vertical, a, b, ja, jb, [4]int{r.Intn(4), r.Intn(4), r.Intn(4), r.Intn(4)}, false)
						runPair(t, r, vertical, a, b, ja, jb, [4]int{4, 4, 4, 4}, true)
						runPair(t, r, vertical, a, b, a, b, [4]int{4, 4, 4, 4}, true)
					}
				}
			}
		})
	}
	// One plane idle (alpha 0) and the other not.
	for _, vertical := range []bool{true, false} {
		runPair(t, r, vertical, 5, 40, 40, 40, [4]int{1, 2, 3, 1}, false)
		runPair(t, r, vertical, 40, 40, 5, 40, [4]int{1, 2, 3, 1}, false)
		runPair(t, r, vertical, 5, 40, 40, 40, [4]int{4, 4, 4, 4}, true)
		runPair(t, r, vertical, 40, 40, 40, 5, [4]int{4, 4, 4, 4}, true)
	}
}

func TestChromaPairRefused(t *testing.T) {
	ok := make([]byte, 64*64)
	ok2 := make([]byte, 64*64)
	short := make([]byte, 64*64-7*64-1)
	var z [4]uint8
	for name, f := range map[string]func(){
		"cb short":      func() { DeblockChromaPairV(short, ok2, 64*64-7*64-2, 64, 30, 30, 30, 30, z) },
		"cr short":      func() { DeblockChromaPairV(ok, short, 64*64-7*64-2, 64, 30, 30, 30, 30, z) },
		"H cr above":    func() { DeblockChromaPairH(ok, ok2[:64*3], 2*64, 64, 30, 30, 30, 30, z) },
		"bS 4":          func() { DeblockChromaPairV(ok, ok2, 4*64+8, 64, 30, 30, 30, 30, [4]uint8{4}) },
		"index Cb":      func() { DeblockChromaPairH(ok, ok2, 4*64+8, 64, 52, 30, 30, 30, z) },
		"index Cr":      func() { DeblockChromaPairH(ok, ok2, 4*64+8, 64, 30, 30, 30, -1, z) },
		"stride":        func() { DeblockChromaPairIntraV(ok, ok2, 4*64+8, 7, 30, 30, 30, 30) },
		"intra H short": func() { DeblockChromaPairIntraH(ok, short, 3600, 64, 30, 30, 30, 30) },
		// Sharing memory: the same slice, overlapping views, and a single
		// byte of overlap; each of the four entry points.
		"V same slice":   func() { DeblockChromaPairV(ok, ok, 4*64+8, 64, 30, 30, 30, 30, z) },
		"H same slice":   func() { DeblockChromaPairH(ok, ok, 4*64+8, 64, 30, 30, 30, 30, z) },
		"intraV same":    func() { DeblockChromaPairIntraV(ok, ok, 4*64+8, 64, 30, 30, 30, 30) },
		"intraH same":    func() { DeblockChromaPairIntraH(ok, ok, 4*64+8, 64, 30, 30, 30, 30) },
		"views overlap":  func() { DeblockChromaPairV(ok[:2048], ok[100:], 4*64+8, 64, 30, 30, 30, 30, z) },
		"one byte share": func() { DeblockChromaPairV(ok[:7*64+4], ok[7*64+3:], 2, 64, 30, 30, 30, 30, z) },
	} {
		mustPanic(t, name, f)
	}
	// Disjoint views of one array, and exactly adjacent footprints, are fine.
	DeblockChromaPairV(ok[:2048], ok[2048:], 2, 64, 30, 30, 30, 30, z)
	DeblockChromaPairV(ok[:7*64+4], ok[7*64+4:], 2, 64, 30, 30, 30, 30, z)
	DeblockChromaPairH(ok, ok2, 2*64, 64, 30, 30, 20, 40, z)
}

func FuzzChromaPair(f *testing.F) {
	r := rand.New(rand.NewSource(18))
	cb := make([]byte, planeStride*planeRows)
	cr := make([]byte, planeStride*planeRows)
	fillEdges(r, cb)
	fillEdges(r, cr)
	f.Add(cb, cr, uint16(edgeOffset(true, 0)), uint16(planeStride), uint8(30), uint8(30), uint8(25), uint8(40), uint8(0x1b), false, true)
	f.Add(cb, cr, uint16(edgeOffset(false, 0)), uint16(planeStride), uint8(51), uint8(51), uint8(10), uint8(51), uint8(0xff), true, false)
	f.Add(cb[:200], cr, uint16(100), uint16(8), uint8(10), uint8(60), uint8(30), uint8(30), uint8(0x55), false, false)

	f.Fuzz(func(t *testing.T, cbIn, crIn []byte, offIn, strideIn uint16, ia, ib, ja, jb, bsBits uint8, strong, vertical bool) {
		off, stride := int(offIn), int(strideIn)
		var bS [4]int
		var u [4]uint8
		for i := range bS {
			bS[i] = int(bsBits>>(2*i)) & 3
			u[i] = uint8(bS[i])
		}
		var first, last int
		if vertical {
			first, last = off-2, off+7*stride+1
		} else {
			first, last = off-2*stride, off+stride+7
		}
		ok := int(ia) <= 51 && int(ib) <= 51 && int(ja) <= 51 && int(jb) <= 51 && stride >= 8 && first >= 0 && last < len(cbIn) && last < len(crIn)

		gotCb, gotCr := append([]byte(nil), cbIn...), append([]byte(nil), crIn...)
		var panicked bool
		func() {
			defer func() { panicked = recover() != nil }()
			switch {
			case strong && vertical:
				DeblockChromaPairIntraV(gotCb, gotCr, off, stride, int(ia), int(ib), int(ja), int(jb))
			case strong:
				DeblockChromaPairIntraH(gotCb, gotCr, off, stride, int(ia), int(ib), int(ja), int(jb))
			case vertical:
				DeblockChromaPairV(gotCb, gotCr, off, stride, int(ia), int(ib), int(ja), int(jb), u)
			default:
				DeblockChromaPairH(gotCb, gotCr, off, stride, int(ia), int(ib), int(ja), int(jb), u)
			}
		}()
		if panicked == ok {
			t.Fatalf("ok=%v panicked=%v", ok, panicked)
		}
		if panicked {
			if !bytes.Equal(gotCb, cbIn) || !bytes.Equal(gotCr, crIn) {
				t.Fatal("a refused call wrote to a plane")
			}
			return
		}
		wantCb, wantCr := append([]byte(nil), cbIn...), append([]byte(nil), crIn...)
		if strong {
			bS = [4]int{4, 4, 4, 4}
		}
		oracleChromaEdge(wantCb, off, stride, vertical, int(ia), int(ib), bS)
		oracleChromaEdge(wantCr, off, stride, vertical, int(ja), int(jb), bS)
		if !bytes.Equal(gotCb, wantCb) || !bytes.Equal(gotCr, wantCr) {
			t.Fatalf("strong=%v vertical=%v: cb differs at %d, cr at %d", strong, vertical, firstDiff(gotCb, wantCb), firstDiff(gotCr, wantCr))
		}
	})
}

// BenchmarkChromaPair compares one two-plane call with two one-plane calls,
// on a flat plane (every line runs the whole formula). The strong horizontal
// edge has no row: its two-plane call is two one-plane calls.
func BenchmarkChromaPair(b *testing.B) {
	cb := make([]byte, planeStride*planeRows)
	cr := make([]byte, planeStride*planeRows)
	for i := range cb {
		cb[i], cr[i] = 100, 120
	}
	bS := [4]uint8{3, 2, 1, 3}
	for _, vertical := range []bool{true, false} {
		off := edgeOffset(vertical, 0)
		dir := map[bool]string{true: "V", false: "H"}[vertical]
		b.Run("pair/normal"+dir, func(b *testing.B) {
			b.SetBytes(16)
			for i := 0; i < b.N; i++ {
				if vertical {
					DeblockChromaPairV(cb, cr, off, planeStride, 40, 30, 36, 33, bS)
				} else {
					DeblockChromaPairH(cb, cr, off, planeStride, 40, 30, 36, 33, bS)
				}
			}
		})
		b.Run("twice/normal"+dir, func(b *testing.B) {
			b.SetBytes(16)
			for i := 0; i < b.N; i++ {
				if vertical {
					DeblockChromaV(cb, off, planeStride, 40, 30, bS)
					DeblockChromaV(cr, off, planeStride, 36, 33, bS)
				} else {
					DeblockChromaH(cb, off, planeStride, 40, 30, bS)
					DeblockChromaH(cr, off, planeStride, 36, 33, bS)
				}
			}
		})
		if !vertical {
			continue
		}
		b.Run("pair/strongV", func(b *testing.B) {
			b.SetBytes(16)
			for i := 0; i < b.N; i++ {
				DeblockChromaPairIntraV(cb, cr, off, planeStride, 40, 30, 36, 33)
			}
		})
		b.Run("twice/strongV", func(b *testing.B) {
			b.SetBytes(16)
			for i := 0; i < b.N; i++ {
				DeblockChromaIntraV(cb, off, planeStride, 40, 30)
				DeblockChromaIntraV(cr, off, planeStride, 36, 33)
			}
		})
	}
}
