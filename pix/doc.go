// Package pix holds the pixel kernels every video decoder shares: block copy,
// rounding average, residual add with clipping to the bit depth, and edge
// emulation for a motion vector whose footprint leaves the picture.
//
// Planes are []byte for 8-bit samples and []uint16 for 9 to 12 bits, each
// with its own stride in samples. The footprint of a call is checked in Go
// before any assembly runs. No kernel is implemented yet; see
// docs/design/design.md.
package pix
