package itx

import "unsafe"

func addOffset(p *byte, off int) unsafe.Pointer { return unsafe.Add(unsafe.Pointer(p), off) }
