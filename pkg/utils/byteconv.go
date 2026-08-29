package utils

import (
	"unsafe"
)

// BytesToString converts a byte slice to a string without memory allocation.
// The caller must ensure that the byte slice is not modified while the string is in use.
func BytesToString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// StringToBytes converts a string to a byte slice without memory allocation.
// The returned byte slice must be treated as read-only.
func StringToBytes(s string) []byte {
	if len(s) == 0 {
		return nil
	}
	return unsafe.Slice(unsafe.StringData(s), len(s))
}
