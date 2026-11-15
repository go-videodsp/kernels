// Package mc holds motion-compensation kernels: H.264 quarter-sample luma
// interpolation (six-tap, put and average forms for every fractional
// position) and eighth-sample chroma interpolation, the weighted and default
// combination of two predictions, and later the HEVC and AV1 separable
// filters producing 16-bit intermediates.
//
// No kernel is implemented yet; see docs/design/design.md.
package mc
