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

**Status: scaffold.** The packages exist and are empty. The design, with the
profile that sets the priorities, is in
[docs/design/design.md](docs/design/design.md).

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
