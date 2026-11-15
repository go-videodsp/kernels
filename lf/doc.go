// Package lf holds in-loop filters: the H.264 luma and chroma deblocking
// filters (boundary strength below 4 and equal to 4, vertical and horizontal
// edges) and a boundary-strength helper; later HEVC deblocking and SAO, and
// AV1 deblocking, CDEF and loop restoration.
//
// No kernel is implemented yet; see docs/design/design.md.
package lf
