// Package cpu decides which implementation of a kernel runs: NEON on arm64
// (mandatory in ARMv8-A, Android included, so no probe), AVX2 on amd64 behind
// the probe of this package, SVE2 on linux and android arm64 only where a
// benchmark shows a gain, and the portable Go reference everywhere else and
// with the purego tag.
//
// The amd64 probe reads CPUID and XGETBV directly: it needs no other module.
package cpu
