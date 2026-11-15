//go:build amd64 && !purego

package mc

import (
	"unsafe"

	"github.com/go-videodsp/kernels/cpu"
)

// The whole-macroblock entries run the kernels directly: the macroblock and the
// partitions were proved by WeightUniMB / WeightBiMB, so a partition's pointer is
// the macroblock's plus Y*stride + X (Y <= 15 and stride <= (len-off-16)/15 were
// established, X <= 15: the sum stays inside the proved window), with no slice
// indexing, no re-dispatch and no re-check per partition.

var (
	weightUniPartsImpl = weightUniPartsDispatch
	weightBiPartsImpl  = weightBiPartsDispatch
)

func init() {
	if cpu.HasAVX2 {
		weightUniPartsImpl = weightUniPartsAVX2
		weightBiPartsImpl = weightBiPartsAVX2
	}
}

func weightUniParts(dst []byte, off, stride int, parts []UniPart, logWD int) {
	weightUniPartsImpl(dst, off, stride, parts, logWD)
}

func weightBiParts(dst []byte, dstOff, dstStride int, src []byte, srcOff, srcStride int, parts []BiPart, logWD int) {
	weightBiPartsImpl(dst, dstOff, dstStride, src, srcOff, srcStride, parts, logWD)
}

func weightUniPartsAVX2(dst []byte, off, stride int, parts []UniPart, logWD int) {
	if len(parts) == 0 {
		return
	}
	rnd := 0
	if logWD >= 1 {
		rnd = 1 << (logWD - 1)
	}
	base := unsafe.Pointer(&dst[off])
	for i := range parts {
		p := &parts[i]
		q := (*byte)(unsafe.Add(base, p.Y*stride+p.X))
		switch p.W {
		case 4:
			weightUni4(q, stride, p.H, p.Weight, rnd, logWD, p.Offset)
		case 8:
			weightUni8(q, stride, p.H, p.Weight, rnd, logWD, p.Offset)
		default:
			weightUni16(q, stride, p.H, p.Weight, rnd, logWD, p.Offset)
		}
	}
}

func weightBiPartsAVX2(dst []byte, dstOff, dstStride int, src []byte, srcOff, srcStride int, parts []BiPart, logWD int) {
	if len(parts) == 0 {
		return
	}
	dbase := unsafe.Pointer(&dst[dstOff])
	sbase := unsafe.Pointer(&src[srcOff])
	for i := range parts {
		p := &parts[i]
		if p.W0 == 1<<logWD && p.W1 == p.W0 && (p.O0+p.O1+1)>>1 == 0 {
			// The default average of 8-273: the fused luma kernel.
			lumaDispatch(dst, dstOff+p.Y*dstStride+p.X, dstStride, src, srcOff+p.Y*srcStride+p.X, srcStride, p.W, p.H, 0, 0, true)
			continue
		}
		d := (*byte)(unsafe.Add(dbase, p.Y*dstStride+p.X))
		s := (*byte)(unsafe.Add(sbase, p.Y*srcStride+p.X))
		wpair := int(uint16(p.W0)) | p.W1<<16
		k := 1<<logWD + ((p.O0+p.O1+1)>>1)<<(logWD+1)
		switch p.W {
		case 4:
			weightBi4(d, dstStride, s, srcStride, p.H, wpair, k, logWD+1)
		case 8:
			weightBi8(d, dstStride, s, srcStride, p.H, wpair, k, logWD+1)
		default:
			weightBi16(d, dstStride, s, srcStride, p.H, wpair, k, logWD+1)
		}
	}
}
