// Package itx holds inverse transforms and dequantisation: the H.264 4x4 and
// 8x8 integer transforms, their DC-only forms and the luma and chroma DC
// transforms, fused with the residual add; later the HEVC DCT 4 to 32 and
// DST 4, and the AV1 transform set.
//
// No kernel is implemented yet; see docs/design/design.md.
package itx
