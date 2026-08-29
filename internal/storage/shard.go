package storage

import (
	"sync"
)

// Shard represents an independent, thread-safe memory partition.
type Shard struct {
	mu    sync.RWMutex
	items map[string]*Item
	_pad  [56]byte // Cache-line alignment padding to prevent false sharing
}

// NewShard initializes a new shard partition.
func NewShard() *Shard {
	return &Shard{
		items: make(map[string]*Item, 1024),
	}
}

// Get fetches an item from the shard. Performs lazy expiration check.
func (s *Shard) Get(key string) (*Item, bool) {
	s.mu.RLock()
	item, ok := s.items[key]
	s.mu.RUnlock()

	if !ok {
		return nil, false
	}

	if item.IsExpired() {
		// Lazy deletion
		s.mu.Lock()
		// Double check after acquiring write lock
		if item, ok = s.items[key]; ok && item.IsExpired() {
			delete(s.items, key)
			s.mu.Unlock()
			return nil, false
		}
		s.mu.Unlock()
	}

	item.UpdateAccess()
	return item, true
}

// Put inserts or updates an item in the shard. Returns freed memory (if replaced) and new memory size.
func (s *Shard) Put(key string, item *Item) (freedBytes int64, addedBytes int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	newSize := item.MemorySize(key)

	if oldItem, exists := s.items[key]; exists {
		freedBytes = oldItem.MemorySize(key)
	}

	s.items[key] = item
	return freedBytes, newSize
}

// Delete removes a key from the shard. Returns bytes freed and whether key was found.
func (s *Shard) Delete(key string) (int64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if item, exists := s.items[key]; exists {
		freed := item.MemorySize(key)
		delete(s.items, key)
		return freed, true
	}
	return 0, false
}

// Keys returns all unexpired keys matching a pattern (or all if pattern is "*").
func (s *Shard) Keys(pattern string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]string, 0, len(s.items))
	for k, item := range s.items {
		if !item.IsExpired() {
			if pattern == "*" || pattern == "" || k == pattern {
				res = append(res, k)
			}
		}
	}
	return res
}

// Count returns current key count in shard.
func (s *Shard) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}
