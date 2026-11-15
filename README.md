# kernels

[![ci](https://github.com/go-videodsp/kernels/actions/workflows/ci.yml/badge.svg)](https://github.com/go-videodsp/kernels/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/go-videodsp/kernels.svg)](https://pkg.go.dev/github.com/go-videodsp/kernels)
[![License: BSD-3-Clause](https://img.shields.io/badge/License-BSD--3--Clause-blue.svg)](LICENSE)

Video codec kernels shared by the go-avkit decoders: motion compensation,
inverse transforms, in-loop filters and intra prediction. Each kernel has a
portable Go reference written from the standard's equations, and assembly for
arm64 NEON first and amd64 AVX2 next, bit-exact against that reference.
H.264 is the first consumer (go-avkit/h264); HEVC and AV1 reuse the same
packages.

**Status: first kernel.** `mc.PutLuma` and `mc.AvgLuma` (H.264 luma
quarter-sample interpolation, all 16 positions, 16x16 down to 4x4) have a
portable reference and an arm64 NEON implementation; the other packages are
still empty. The design, with the profile that sets the priorities, is in
[docs/design/design.md](docs/design/design.md).

```go
// Predict a 16x16 block at quarter position (3, 1) into dst, then average a
// second prediction into the same block (bi-prediction, clause 8.4.2.3.1).
mc.PutLuma(dst, dstOff, dstStride, ref0, off0, refStride, 16, 16, 3, 1)
mc.AvgLuma(dst, dstOff, dstStride, ref1, off1, refStride, 16, 16, 1, 2)
```

The footprint (the block plus 2 samples before and 3 after in each
interpolated direction) must lie inside the reference slice; the call panics
before touching memory otherwise. Edge emulation is the caller's, until `pix`
provides it.

**mc.PutLuma, Apple M4 Max, one core, ns per block (NEON / portable Go).**
Indicative: the machine's 1-min load was 66 (the rule is 8), so read the
ratios as an order of magnitude and re-measure on a quiet machine. Raw output
in [docs/bench](docs/bench).

| Block | G (copy) | b (half, H) | j (centre) | e (diagonal) |
|---|---|---|---|---|
| 16x16 | 15 / 643 | 26 / 788 | 76 / 1317 | 52 / 1719 |
| 8x8 | 8.6 / 177 | 12 / 213 | 21 / 356 | 17 / 342 |
| 4x4 | 7.1 / 57 | 9.4 / 66 | 18 / 134 | 12 / 141 |

**Weighted prediction (clause 8.4.2.3.2).** `mc.WeightUni` applies the
single-list form (Equations 8-274, 8-275) in place, `mc.WeightBi` the two-list
form (8-276) into the first prediction. With `w0 = w1 = 2^logWD` and a zero
rounded offset (the implicit (32, 32) case among them) `WeightBi` runs the
fused averaging kernel of `AvgLuma`, as the design asks; that is
bit-identical to 8-273 by the equation. NEON, one core, ns per block (NEON /
portable Go), load 26, indicative:

| Block | uni | bi |
|---|---|---|
| 16x16 | 24 / 158 | 32 / 258 |
| 8x8 | 11.6 / 48 | 14.6 / 56 |
| 4x4 | 10.4 / 12.5 | 11.1 / 15.2 |

The 4x4 gain is small: the call (bounds proof plus dispatch) costs about as
much as the work. Callers batch by macroblock, or the next step takes the
whole partition. Raw output in [docs/bench](docs/bench).

No FFmpeg `checkasm h264qpel` column: no checkasm binary is installed and the
local FFmpeg 9.0.2 clone is a partial tree with no build system.

The NEON source is generated: `go generate ./mc` runs `internal/genmc`, and a
test fails when the checked-in `.s` differs from its output. The one
instruction `cmd/asm` lacks that the kernels need, `SQRSHRUN`, is emitted as a
`WORD`.

| Package | Holds |
|---|---|
| [`pix`](pix) | copy, rounding average, residual add, edge emulation |
| [`mc`](mc) | luma and chroma interpolation, weighted prediction |
| [`itx`](itx) | inverse transforms and dequantisation |
| [`lf`](lf) | deblocking and the other in-loop filters |
| [`ipred`](ipred) | intra prediction |
| [`cpu`](cpu) | which implementation runs on this processor |

Pure Go plus Go assembly, `CGO_ENABLED=0`. The assembly is generated with
[go-asmgen](https://github.com/go-asmgen/asmgen).

## Licence

BSD-3-Clause, see [LICENSE](LICENSE).
