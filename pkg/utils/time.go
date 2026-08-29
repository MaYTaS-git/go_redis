package utils

import (
	"time"
)

// FastNowUnixMilli returns the current unix timestamp in milliseconds.
func FastNowUnixMilli() int64 {
	return time.Now().UnixMilli()
}

// FastNowUnixNano returns the current unix timestamp in nanoseconds.
func FastNowUnixNano() int64 {
	return time.Now().UnixNano()
}

// IsExpired checks if a given expiration timestamp (in unix milliseconds) has passed.
func IsExpired(expiresAt int64) bool {
	if expiresAt <= 0 {
		return false
	}
	return time.Now().UnixMilli() >= expiresAt
}
