package storage

const (
	offset64 = 14695981039346656037
	prime64  = 1099511628211
)

// Hash64 calculates a fast 64-bit FNV-1a hash of a byte slice.
func Hash64(data []byte) uint64 {
	var hash uint64 = offset64
	for _, b := range data {
		hash ^= uint64(b)
		hash *= prime64
	}
	return hash
}

// HashString64 calculates a fast 64-bit FNV-1a hash of a string.
func HashString64(s string) uint64 {
	var hash uint64 = offset64
	for i := 0; i < len(s); i++ {
		hash ^= uint64(s[i])
		hash *= prime64
	}
	return hash
}
