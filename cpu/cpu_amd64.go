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

const (
	osxsaveBit = 1 << 27 // CPUID.1:ECX
	avxBit     = 1 << 28 // CPUID.1:ECX
	avx512Bits = 1<<16 | 1<<17 | 1<<30 | 1<<31
)

// decide turns the CPUID and XCR0 words into the two answers. xcr0 is only
// meaningful when OSXSAVE is set; decide ignores it otherwise.
func decide(maxLeaf, c1, b7, xcr0 uint32) (avx2, avx512 bool) {
	osAVX := c1&osxsaveBit != 0 && c1&avxBit != 0 && xcr0&0x6 == 0x6
	osAVX512 := osAVX && xcr0&0xe0 == 0xe0
	has7 := maxLeaf >= 7
	return has7 && osAVX && b7&(1<<5) != 0, has7 && osAVX512 && b7&avx512Bits == avx512Bits
}

func init() {
	maxLeaf, _, _, _ := cpuid(0, 0)
	_, _, c1, _ := cpuid(1, 0)
	var xcr0 uint32
	// XGETBV raises #UD when OSXSAVE is clear: read XCR0 only after checking it.
	if c1&osxsaveBit != 0 {
		xcr0, _ = xgetbv0()
	}
	_, b7, _, _ := cpuid(7, 0)
	HasAVX2, HasAVX512 = decide(maxLeaf, c1, b7, xcr0)
}
