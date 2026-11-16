package hevc

import "unsafe"

// unsafeInt16 views the memory of a uint16 slice as int16, to build two slices that share it.
func unsafeInt16(s []uint16) []int16 {
	return unsafe.Slice((*int16)(unsafe.Pointer(unsafe.SliceData(s))), len(s))
}

// unsafeInt32 views the memory of an int16 slice as int32 (half as many elements).
func unsafeInt32(s []int16) []int32 {
	return unsafe.Slice((*int32)(unsafe.Pointer(unsafe.SliceData(s))), len(s)/2)
}
