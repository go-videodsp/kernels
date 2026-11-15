// Package cpu decides which implementation of a kernel runs: NEON on arm64
// (mandatory in ARMv8-A, Android included), AVX2 on amd64 behind a feature
// probe, SVE2 on linux and android arm64 only where a benchmark shows a gain,
// and the portable Go reference everywhere else.
//
// No dispatch is implemented yet; see docs/design/design.md.
package cpu
