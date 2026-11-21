// Package cabac decodes the residual blocks of H.264 CABAC (ITU-T H.264 (V16)
// clause 9.3): the significance map, the last flags, the coefficient levels and
// their signs of one transform block, in one call that keeps the arithmetic
// decoder's registers (range, offset, stream position) in machine registers from
// the first bin to the last.
//
// The decoder state is a State, which a caller shares with its own single-bin
// decoding (DecideRef and BypassRef are the reference bins). The kernel is
// written for amd64 and arm64 and has a portable reference, Decode with the
// purego tag or on another architecture, which is also the specification of
// what the assembly must compute: the same coefficients, the same state, bin
// for bin.
//
// Input safety. The stream is untrusted. A byte is read only while Ptr is below
// len(Data); the decoder reads zeros past the end and counts them in Ptr, as
// the specification's reader does, so a truncated stream decodes (to garbage)
// and the caller learns it from Ptr. The scan tables are validated once by
// NewScan, so no context index leaves the context array; a context byte takes
// all 256 values safely (see tables.go).
package cabac
