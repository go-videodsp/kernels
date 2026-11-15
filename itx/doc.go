// Package itx holds inverse transforms and dequantisation: the H.264 4x4 and
// 8x8 integer transforms, their DC-only forms and the luma and chroma DC
// transforms, fused with the residual add; later the HEVC DCT 4 to 32 and
// DST 4, and the AV1 transform set.
//
// Implemented: Dequant8x8IDCTAdd (AVX2, NEON, portable reference). The rest
// is listed in docs/design/design.md.
package itx
