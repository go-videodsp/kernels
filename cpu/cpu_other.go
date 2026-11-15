//go:build !amd64 || purego

package cpu

// HasAVX2 is false off amd64 and with the purego tag.
const HasAVX2 = false

// HasAVX512 is false off amd64 and with the purego tag.
const HasAVX512 = false
