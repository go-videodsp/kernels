package lf

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
)

func runPair(t *testing.T, r *rand.Rand, vertical bool, indexA, indexB int, bS [4]int, strong bool) {
	t.Helper()
	cb := make([]byte, planeStride*planeRows)
	cr := make([]byte, planeStride*planeRows)
	fillEdges(r, cb)
	fillEdges(r, cr)
	off := edgeOffset(vertical, r.Intn(4))
	wantCb := append([]byte(nil), cb...)
	wantCr := append([]byte(nil), cr...)
	oracleChromaEdge(wantCb, off, planeStride, vertical, indexA, indexB, bS)
	oracleChromaEdge(wantCr, off, planeStride, vertical, indexA, indexB, bS)
	var u [4]uint8
	for i := range u {
		u[i] = uint8(bS[i])
	}
	switch {
	case strong && vertical:
		DeblockChromaPairIntraV(cb, cr, off, planeStride, indexA, indexB)
	case strong:
		DeblockChromaPairIntraH(cb, cr, off, planeStride, indexA, indexB)
	case vertical:
		DeblockChromaPairV(cb, cr, off, planeStride, indexA, indexB, u)
	default:
		DeblockChromaPairH(cb, cr, off, planeStride, indexA, indexB, u)
	}
	if !bytes.Equal(cb, wantCb) || !bytes.Equal(cr, wantCr) {
		t.Fatalf("vertical=%v strong=%v indexA %d indexB %d bS %v: cb differs at %d, cr at %d", vertical, strong, indexA, indexB, bS, firstDiff(cb, wantCb), firstDiff(cr, wantCr))
	}
}

func TestChromaPairExhaustive(t *testing.T) {
	r := rand.New(rand.NewSource(17))
	patterns := [][4]int{{1, 1, 1, 1}, {2, 2, 2, 2}, {3, 3, 3, 3}, {0, 0, 0, 0}, {0, 1, 2, 3}, {3, 2, 1, 0}, {1, 0, 3, 2}, {2, 3, 0, 1}}
	for _, vertical := range []bool{true, false} {
		t.Run(fmt.Sprintf("vertical=%v", vertical), func(t *testing.T) {
			for a := 0; a < 52; a++ {
				for b := 0; b < 52; b++ {
					for _, p := range patterns {
						runPair(t, r, vertical, a, b, p, false)
					}
					for i := 0; i < 3; i++ {
						runPair(t, r, vertical, a, b, [4]int{r.Intn(4), r.Intn(4), r.Intn(4), r.Intn(4)}, false)
						runPair(t, r, vertical, a, b, [4]int{4, 4, 4, 4}, true)
					}
				}
			}
		})
	}
}

func TestChromaPairRefused(t *testing.T) {
	ok := make([]byte, 64*64)
	short := make([]byte, 64*64-7*64-1)
	var z [4]uint8
	for name, f := range map[string]func(){
		"cb short":      func() { DeblockChromaPairV(short, ok, 64*64-7*64-2, 64, 30, 30, z) },
		"cr short":      func() { DeblockChromaPairV(ok, short, 64*64-7*64-2, 64, 30, 30, z) },
		"H cr above":    func() { DeblockChromaPairH(ok, ok[:64*3], 2*64, 64, 30, 30, z) },
		"bS 4":          func() { DeblockChromaPairV(ok, ok, 4*64+8, 64, 30, 30, [4]uint8{4}) },
		"index":         func() { DeblockChromaPairH(ok, ok, 4*64+8, 64, 52, 30, z) },
		"stride":        func() { DeblockChromaPairIntraV(ok, ok, 4*64+8, 7, 30, 30) },
		"intra H short": func() { DeblockChromaPairIntraH(ok, short, 3600, 64, 30, 30) },
	} {
		mustPanic(t, name, f)
	}
	DeblockChromaPairV(ok, ok[:64*64-1], 2, 64, 30, 30, z)
	DeblockChromaPairH(ok, ok, 2*64, 64, 30, 30, z)
}

func FuzzChromaPair(f *testing.F) {
	r := rand.New(rand.NewSource(18))
	cb := make([]byte, planeStride*planeRows)
	cr := make([]byte, planeStride*planeRows)
	fillEdges(r, cb)
	fillEdges(r, cr)
	f.Add(cb, cr, uint16(edgeOffset(true, 0)), uint16(planeStride), uint8(30), uint8(30), uint8(0x1b), false, true)
	f.Add(cb, cr, uint16(edgeOffset(false, 0)), uint16(planeStride), uint8(51), uint8(51), uint8(0xff), true, false)
	f.Add(cb[:200], cr, uint16(100), uint16(8), uint8(10), uint8(60), uint8(0x55), false, false)

	f.Fuzz(func(t *testing.T, cbIn, crIn []byte, offIn, strideIn uint16, ia, ib, bsBits uint8, strong, vertical bool) {
		off, stride := int(offIn), int(strideIn)
		indexA, indexB := int(ia), int(ib)
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
		ok := indexA <= 51 && indexB <= 51 && stride >= 8 && first >= 0 && last < len(cbIn) && last < len(crIn)

		gotCb, gotCr := append([]byte(nil), cbIn...), append([]byte(nil), crIn...)
		var panicked bool
		func() {
			defer func() { panicked = recover() != nil }()
			switch {
			case strong && vertical:
				DeblockChromaPairIntraV(gotCb, gotCr, off, stride, indexA, indexB)
			case strong:
				DeblockChromaPairIntraH(gotCb, gotCr, off, stride, indexA, indexB)
			case vertical:
				DeblockChromaPairV(gotCb, gotCr, off, stride, indexA, indexB, u)
			default:
				DeblockChromaPairH(gotCb, gotCr, off, stride, indexA, indexB, u)
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
		oracleChromaEdge(wantCb, off, stride, vertical, indexA, indexB, bS)
		oracleChromaEdge(wantCr, off, stride, vertical, indexA, indexB, bS)
		if !bytes.Equal(gotCb, wantCb) || !bytes.Equal(gotCr, wantCr) {
			t.Fatalf("strong=%v vertical=%v: cb differs at %d, cr at %d", strong, vertical, firstDiff(gotCb, wantCb), firstDiff(gotCr, wantCr))
		}
	})
}

// BenchmarkChromaPair compares one two-plane call with two one-plane calls,
// on a flat plane (every line runs the whole formula).
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
					DeblockChromaPairV(cb, cr, off, planeStride, 40, 30, bS)
				} else {
					DeblockChromaPairH(cb, cr, off, planeStride, 40, 30, bS)
				}
			}
		})
		b.Run("twice/normal"+dir, func(b *testing.B) {
			b.SetBytes(16)
			for i := 0; i < b.N; i++ {
				if vertical {
					DeblockChromaV(cb, off, planeStride, 40, 30, bS)
					DeblockChromaV(cr, off, planeStride, 40, 30, bS)
				} else {
					DeblockChromaH(cb, off, planeStride, 40, 30, bS)
					DeblockChromaH(cr, off, planeStride, 40, 30, bS)
				}
			}
		})
		b.Run("pair/strong"+dir, func(b *testing.B) {
			b.SetBytes(16)
			for i := 0; i < b.N; i++ {
				if vertical {
					DeblockChromaPairIntraV(cb, cr, off, planeStride, 40, 30)
				} else {
					DeblockChromaPairIntraH(cb, cr, off, planeStride, 40, 30)
				}
			}
		})
		b.Run("twice/strong"+dir, func(b *testing.B) {
			b.SetBytes(16)
			for i := 0; i < b.N; i++ {
				if vertical {
					DeblockChromaIntraV(cb, off, planeStride, 40, 30)
					DeblockChromaIntraV(cr, off, planeStride, 40, 30)
				} else {
					DeblockChromaIntraH(cb, off, planeStride, 40, 30)
					DeblockChromaIntraH(cr, off, planeStride, 40, 30)
				}
			}
		})
	}
}
