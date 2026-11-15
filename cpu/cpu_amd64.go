//go:build amd64 && !purego

package cpu

// HasAVX2 reports that the processor implements AVX2 and the operating system
// saves the YMM registers across context switches: CPUID.1:ECX.OSXSAVE and
// .AVX, XCR0 bits 1 and 2 (SSE and AVX state), CPUID.7.0:EBX.AVX2 (Intel SDM
// vol. 1, section 14.3, "Detection of AVX instructions" and 15.2).
var HasAVX2 bool

// HasAVX512 reports AVX-512 F, BW, DQ and VL with the operating system saving
// the opmask and ZMM state (XCR0 bits 5, 6 and 7).
var HasAVX512 bool

func cpuid(eaxArg, ecxArg uint32) (eax, ebx, ecx, edx uint32)
func xgetbv0() (eax, edx uint32)

func init() {
	maxLeaf, _, _, _ := cpuid(0, 0)
	_, _, c1, _ := cpuid(1, 0)
	xcr0, _ := xgetbv0()
	const osxsave, avx = 1 << 27, 1 << 28
	osAVX := c1&osxsave != 0 && c1&avx != 0 && xcr0&0x6 == 0x6
	osAVX512 := osAVX && xcr0&0xe0 == 0xe0
	_, b7, _, _ := cpuid(7, 0)
	has7 := maxLeaf >= 7
	HasAVX2 = has7 && osAVX && b7&(1<<5) != 0
	// F is bit 16, DQ 17, BW 30, VL 31.
	const avx512 = 1<<16 | 1<<17 | 1<<30 | 1<<31
	HasAVX512 = has7 && osAVX512 && b7&avx512 == avx512
}
