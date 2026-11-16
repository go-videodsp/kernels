# HEVC luma interpolation passes

`hevc.HPass8` and `hevc.VPass8` are the two stages of the separable eight-tap filter of
ITU-T H.265 (V11) (01/2026), clause 8.5.3.3.3.2 (Equations 8-224 to 8-238).

## Types and ranges

Without them the kernels would not be exact, so they are the contract.

* Samples are `uint16` below `1 << bitDepth`, bit depth 8 to 12. The kernels widen them
  as signed 16-bit numbers, so none may exceed 32767.
* The first stage shifts by `shift1 = bitDepth - 8` and keeps `int16`. The sum of the
  magnitudes of the negative taps of the half-sample filter is 24 and of the positive
  ones 88, so for bit depth 12 or less the outputs lie in -6144 to 22528 and fit.
* The second stage takes those `int16` values, shifts by 6 and keeps `int32`: its outputs
  can reach about 33 150 for 8-bit video, which does not fit `int16`.
* Above bit depth 12 the first stage no longer fits `int16`: `TryHPass8` refuses, and the
  caller uses its own path.

## Implementations

* Portable (`pass_ref.go`): the equations, one sample at a time. The oracle, and the path
  of every target without a kernel and of builds with `purego`.
* arm64 (`pass_arm64.s`): per group of eight columns, eight `VLD1` at the eight tap
  offsets, `VSMLAL` and `VSMLAL2` with the coefficients broadcast (`VLD1R`), `VSSHR`
  by the immediate shift (one entry point per shift 0 to 4), `VXTN`, `VST1`; a group of
  four for the width's remainder. No sample past the footprint is read.
* amd64 (`pass_amd64.s`): the same with `VPMOVZXWD`/`VPMOVSXWD` to eight 32-bit lanes,
  `VPMULLD` by the broadcast coefficient, `VPSRAD` by a count register, then a byte
  shuffle that keeps the low 16 bits of each lane (`VPSHUFB`, `VPERMQ`). Not
  `VPACKSSDW`: it saturates, and a sum beyond the bound then differs from the
  reference and from NEON, whose `XTN` truncates (the fuzz corpus seed that caught
  it ran green on arm64 and red on amd64).
* Both are written by `internal/genhevc`, not by hand; `go generate ./hevc` rewrites them
  and a test fails if the checked-in files differ.

## The four-tap chroma passes

`HPass4` and `VPass4` are the same two stages for the chroma filter (clause 8.5.3.3.3.3,
Equations 8-241 to 8-261): taps at offsets -1 to 2 (footprint 1 before and 2 after), the same
types and shifts, and any even block width from 2 to 64 (the chroma widths of 4:2:0 and
4:2:2 are half the luma ones, so not multiples of 4). Each kernel runs groups of eight, then
four, then two columns; the group of two loads and stores exactly 4 bytes (`FMOVS` on NEON,
`VMOVD` on AVX2), so no sample past the footprint is touched. They are written by the same
generator (`internal/genhevc/chroma.go`).

## Checks

Each entry has a `Try` form that proves the footprint with `internal/bounds.Window` (no
wrapping arithmetic), the block size (width a multiple of 4 up to 64, height 1 to 64), the
strides and the memory of the two sides (they must not overlap), and refuses with an error
wrapping `ErrRefused`, nothing written. The panicking forms call it.
