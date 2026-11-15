package mc

// Portable reference for H.264 luma sample interpolation, written from
// ITU-T H.264 (V16) (06/2026), clause 8.4.2.2.1 (Figure 8-4, Table 8-11,
// Equations 8-241 to 8-261, Table 8-12) and clause 8.4.2.3.1 (Equation 8-273,
// the rounding average of two predictions).
//
// The reference reads the plane without clamping: clause 8.4.2.2.1 clamps the
// reference coordinates (Equations 8-239, 8-240), which the caller reproduces
// by emulating the edge when the footprint leaves the picture.

// termKind names one of the samples of Figure 8-4 that a position is built
// from.
type termKind uint8

const (
	termG termKind = iota // a full sample, displaced by (dx, dy)
	termB                 // half sample b (dy = 0) or s (dy = 1): horizontal six-tap, 8-243, 8-248
	termH                 // half sample h (dx = 0) or m (dx = 1): vertical six-tap, 8-244, 8-249
	termJ                 // centre half sample j, 8-246, 8-247
)

type term struct {
	kind   termKind
	dx, dy int
}

// lumaTerms is Table 8-12 with Equations 8-250 to 8-261 applied: index is
// xFrac + 4*yFrac; a position is one term, or the rounding average of two
// (n == 2).
var lumaTerms [16]struct {
	n int
	t [2]term
}

func init() {
	// Index = xFrac + 4*yFrac. Letters are Table 8-12.
	set := func(xf, yf int, a term, b ...term) {
		e := &lumaTerms[xf+4*yf]
		e.n, e.t[0] = 1, a
		if len(b) == 1 {
			e.n, e.t[1] = 2, b[0]
		}
	}
	bT, sT := term{termB, 0, 0}, term{termB, 0, 1} // b, s
	hT, mT := term{termH, 0, 0}, term{termH, 1, 0} // h, m
	jT := term{termJ, 0, 0}
	set(0, 0, term{termG, 0, 0})     // G
	set(0, 1, term{termG, 0, 0}, hT) // d = (G + h + 1) >> 1       8-252
	set(0, 2, hT)                    // h
	set(0, 3, term{termG, 0, 1}, hT) // n = (M + h + 1) >> 1       8-253
	set(1, 0, term{termG, 0, 0}, bT) // a = (G + b + 1) >> 1       8-250
	set(1, 1, bT, hT)                // e = (b + h + 1) >> 1       8-258
	set(1, 2, hT, jT)                // i = (h + j + 1) >> 1       8-255
	set(1, 3, hT, sT)                // p = (h + s + 1) >> 1       8-260
	set(2, 0, bT)                    // b
	set(2, 1, bT, jT)                // f = (b + j + 1) >> 1       8-254
	set(2, 2, jT)                    // j
	set(2, 3, jT, sT)                // q = (j + s + 1) >> 1       8-257
	set(3, 0, term{termG, 1, 0}, bT) // c = (H + b + 1) >> 1       8-251
	set(3, 1, bT, mT)                // g = (b + m + 1) >> 1       8-259
	set(3, 2, jT, mT)                // k = (j + m + 1) >> 1       8-256
	set(3, 3, mT, sT)                // r = (m + s + 1) >> 1       8-261
}

func clip1(v int32) int32 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

// tap6 is the six-tap filter (1, -5, 20, 20, -5, 1) of 8.4.2.2.1 on six
// samples, the intermediate of Equations 8-241 and 8-242.
func tap6(a, b, c, d, e, f int32) int32 {
	return a - 5*b + 20*c + 20*d - 5*e + f
}

// lumaRef writes (avg false) or averages into (avg true, Equation 8-273) the
// w x h block of predicted luma samples at fractional position (fx, fy).
// Inputs are already validated by the caller.
func lumaRef(dst []byte, dOff, dStride int, src []byte, sOff, sStride, w, h, fx, fy int, avg bool) {
	e := &lumaTerms[fx+4*fy]

	// j needs the horizontal intermediates b1 of rows -2..h+2 (Equation
	// 8-246): compute each once.
	var b1 [21 * 16]int32
	needJ := e.t[0].kind == termJ || (e.n == 2 && e.t[1].kind == termJ)
	if needJ {
		for r := 0; r < h+5; r++ {
			p := sOff + (r-2)*sStride
			for x := 0; x < w; x++ {
				q := p + x
				b1[r*w+x] = tap6(int32(src[q-2]), int32(src[q-1]), int32(src[q]), int32(src[q+1]), int32(src[q+2]), int32(src[q+3]))
			}
		}
	}

	eval := func(t term, x, y, p int) int32 {
		switch t.kind {
		case termG:
			return int32(src[p+t.dx+t.dy*sStride])
		case termB: // b (8-243), s (8-248); dy selects the row
			q := p + t.dy*sStride
			return clip1((tap6(int32(src[q-2]), int32(src[q-1]), int32(src[q]), int32(src[q+1]), int32(src[q+2]), int32(src[q+3])) + 16) >> 5)
		case termH: // h (8-244), m (8-249); dx selects the column
			q := p + t.dx
			s := sStride
			return clip1((tap6(int32(src[q-2*s]), int32(src[q-s]), int32(src[q]), int32(src[q+s]), int32(src[q+2*s]), int32(src[q+3*s])) + 16) >> 5)
		default: // j, 8-246 and 8-247
			c := y*w + x
			return clip1((tap6(b1[c], b1[c+w], b1[c+2*w], b1[c+3*w], b1[c+4*w], b1[c+5*w]) + 512) >> 10)
		}
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p := sOff + y*sStride + x
			v := eval(e.t[0], x, y, p)
			if e.n == 2 {
				v = (v + eval(e.t[1], x, y, p) + 1) >> 1
			}
			d := dOff + y*dStride + x
			if avg {
				v = (int32(dst[d]) + v + 1) >> 1 // 8-273
			}
			dst[d] = byte(v)
		}
	}
}
