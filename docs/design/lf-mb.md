# Whole-macroblock deblocking: design note

Status: proposal, before code. Scope: package `lf`, H.264 first, HEVC later.

## Why

Measured on a Ryzen 9 7950X (fractal, one core, perf cycles:u, 1080p decodes
of two clips), after the 8x8 and 4x4 transform kernels, the loop filter of
go-avkit/h264 is the largest non-CABAC cluster: about 11 % of all cycles for
the code that decides boundary strengths and prepares each edge, plus about 5 %
for the edge kernels and their argument checks. The kernels themselves are fast;
what is left is per-edge overhead: for every macroblock the decoder makes up to
eight luma and four chroma-pair calls, each with its own bounds proof
(`internal/bounds.Window`), its own classification (all bS 4, none, or mixed)
and a Go-to-assembly transition. The earlier h264 experiments on the decision
side (a per-macroblock non-zero mask, a shortcut for quadrants of uniform
motion, thresholds computed once per macroblock) together gained 1 to 1.5 %:
the decision code is already cheap per edge, so the gain has to come from doing
fewer, larger things.

## What the standard fixes (H.264, clause 8.7)

For one macroblock the filter runs on the luma plane vertical edges left to
right (0 to 3, edge 0 being the macroblock's left edge), then horizontal edges
top to bottom, and the chroma planes the same way with their own thresholds;
luma and chroma do not read each other's samples. A 4:2:0 chroma plane has
edges 0 and 2 only (chroma edge 1 is luma edge 2). Per edge there are four
boundary strengths, one per four lines of luma (two lines of chroma); bS 4 only
on the macroblock edge. The thresholds (`indexA`, `indexB`) are those of the
average QP of the two macroblocks: on the left and top edges the neighbour's, on
inner edges the current macroblock's own. Chroma has one pair per plane.
Neighbouring macroblocks are filtered in raster order, so the samples a
macroblock's left and top edges read were already modified by the neighbours;
a macroblock call may therefore not be reordered against its neighbours, only
fused inside itself.

## Layout: what the decoder passes

One descriptor and the three planes' windows; no pointer to the neighbours.

```go
// MBEdges describes the filtering of one macroblock (H.264 frame
// macroblocks, 4:2:0, 8-bit). BS[d][e][g] is the boundary strength of edge e
// (0..3) of direction d (0 vertical, 1 horizontal), line group g (0..3); the
// decoder zeroes the edges it does not filter (transform 8x8 skips edges 1 and
// 3, a missing neighbour or disable_deblocking_filter_idc skips edge 0).
// IndexA and IndexB are per direction and per kind of edge (0 macroblock edge,
// 1 inner edges); Chroma holds the same per plane.
type MBEdges struct {
    BS     [2][4][4]uint8
    Luma   [2][2]Thresholds    // [direction][macroblock edge, inner]
    Chroma [2][2][2]Thresholds // [plane][direction][macroblock edge, inner]
}
type Thresholds struct{ IndexA, IndexB uint8 }

func DeblockMB(y []byte, yOff, yStride int, cb, cr []byte, cOff, cStride int, e *MBEdges)
```

The decoder keeps filling `BS` and the thresholds as it does today (that is the
decision code, which depends on its own motion and coefficient structures and
stays in the codec); the kernel takes over everything after the decision.
Mixed strengths on one edge (bS 4 beside lower values) occur only in field
and MBAFF macroblocks; for frame macroblocks an edge is all 4 or all below 4,
and `DeblockMB` panics on a mix instead of falling back, so the decoder keeps
its scalar path for the pictures that need it.

## Implementation, in two steps

1. A Go loop over the existing edge kernels. One bounds proof for the whole
   macroblock window (the filter reads 3 samples before the left and top
   edges and writes 3, so the window is 16+3 by 16+3 of luma and 8+2 of
   chroma), then direct calls of the assembly edge routines with no per-edge
   check, in the standard's order. No new assembly, so exactness is inherited
   edge by edge, and the test is the macroblock result against the composition
   of the existing public edge calls on random content and every bS pattern.
   This removes the per-edge argument checks and classification, the Go wrapper
   layers and the pair of chroma wrappers; its measured gain decides step 2.
2. Only if step 1 leaves the call overhead visible in a profile: one assembly
   routine that loops over the edges itself and keeps the constants (the
   thresholds tables, the saturating-add constants) in registers between them.
   The edge bodies are the existing ones, so again no new arithmetic.

Both steps keep the contract of the package: portable reference, guard-page and
bounds tests, fuzz against the composition of per-edge calls, mutation checks,
100 % coverage, AVX2 and NEON.

## Reuse by HEVC (clause 8.7.2 of H.265)

HEVC deblocks an 8x8 grid in the picture, vertical edges of the whole coding
tree block before the horizontal ones, with bS 0 to 2, per four-line segments
(the same granularity as H.264's line groups), thresholds beta and tc from the
QP and slice offsets, and chroma only for bS 2 on a 16-luma-sample grid. The
descriptor generalises without change of shape: a list of edges, each with its
position, direction, length, per-segment strength and thresholds. The H.264
macroblock is the special case of 4 + 4 edges of 16 lines; an HEVC call would
take the edge list of one coding tree block (or one row of them). That is why
the first version is written as a loop over edges taking
`(position, direction, bS [4]uint8, thresholds)`, with `DeblockMB` a fixed
instance of it, and why `Thresholds` carries indexes into tables rather than
alpha and beta values: HEVC would add its own tables beside H.264's, not
another call shape. HEVC's strong and normal luma filters and its chroma
filter are different arithmetic and stay new kernels; what is shared is the
call structure, the bounds proof and the order of edges.

## Open questions

- Whether the transform-8x8 case (edges 1 and 3 zero) is worth a flag so the
  loop skips them without reading the zeros: decided by the step 1 profile.
- 4:2:2 and 4:4:4 chroma have other edge sets and are out of scope here.
- 16-bit samples: out of scope, as everywhere in the package for now.
