package storage

import (
	"sync/atomic"
	"time"
)

const (
	// ItemStructSize accounts for item struct overhead in Go runtime (~48-64 bytes) + map entry overhead (~32 bytes)
	ItemStructSize = 96
)

// Item represents a single key-value entry stored in memory.
type Item struct {
	Value       []byte
	ExpiresAt   int64  // Unix millisecond timestamp (0 = no expiration)
	LastAccess  int64  // Unix millisecond timestamp for LRU
	AccessCount uint32 // Frequency counter for LFU
}

// NewItem creates a new item instance with initial access metrics.
func NewItem(value []byte, ttlMs int64) *Item {
	now := time.Now().UnixMilli()
	var expiresAt int64 = 0
	if ttlMs > 0 {
		expiresAt = now + ttlMs
	}

	valCopy := make([]byte, len(value))
	copy(valCopy, value)

	return &Item{
		Value:       valCopy,
		ExpiresAt:   expiresAt,
		LastAccess:  now,
		AccessCount: 1,
	}
}

// IsExpired returns true if the item has passed its expiration time.
func (item *Item) IsExpired() bool {
	if item.ExpiresAt <= 0 {
		return false
	}
	return time.Now().UnixMilli() >= item.ExpiresAt
}

// UpdateAccess updates LRU access time and increments LFU frequency counter.
func (item *Item) UpdateAccess() {
	item.LastAccess = time.Now().UnixMilli()
	atomic.AddUint32(&item.AccessCount, 1)
}

// MemorySize calculates the total heap footprint of key and item payload.
func (item *Item) MemorySize(key string) int64 {
	return int64(len(key) + len(item.Value) + ItemStructSize)
}
