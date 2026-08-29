package storage

import (
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"go_redis/internal/storage/eviction"
	"go_redis/pkg/utils"
)

const NumShards = 64

// Engine is the top-level sharded in-memory key-value database engine.
type Engine struct {
	shards         [NumShards]*Shard
	usedMemory     int64
	maxMemory      int64
	evictionPolicy string
	hits           int64
	misses         int64
	stopCh         chan struct{}
	wg             sync.WaitGroup
}

// NewEngine creates and initializes a new sharded database engine.
func NewEngine(maxMemoryMB int64, evictionPolicy string) *Engine {
	maxMemBytes := maxMemoryMB * 1024 * 1024
	if maxMemBytes <= 0 {
		maxMemBytes = 512 * 1024 * 1024 // 512MB default
	}

	e := &Engine{
		maxMemory:      maxMemBytes,
		evictionPolicy: evictionPolicy,
		stopCh:         make(chan struct{}),
	}

	for i := 0; i < NumShards; i++ {
		e.shards[i] = NewShard()
	}

	// Start background active TTL expiration sampler
	e.wg.Add(1)
	go e.activeExpirationLoop()

	return e
}

// Close gracefully stops background tasks.
func (e *Engine) Close() {
	close(e.stopCh)
	e.wg.Wait()
}

// getShard returns the shard for a given key using 64-bit fast hash.
func (e *Engine) getShard(key string) *Shard {
	idx := HashString64(key) & (NumShards - 1)
	return e.shards[idx]
}

// Get fetches a key's value. Returns nil, false if not found or expired.
func (e *Engine) Get(key string) ([]byte, bool) {
	shard := e.getShard(key)
	item, found := shard.Get(key)
	if !found {
		atomic.AddInt64(&e.misses, 1)
		return nil, false
	}

	atomic.AddInt64(&e.hits, 1)
	return item.Value, true
}

// Set stores a key-value pair with an optional TTL (in milliseconds).
func (e *Engine) Set(key string, val []byte, ttlMs int64) error {
	item := NewItem(val, ttlMs)
	newItemSize := item.MemorySize(key)

	// Check eviction requirement
	if e.maxMemory > 0 && atomic.LoadInt64(&e.usedMemory)+newItemSize > e.maxMemory {
		if err := e.evictMemory(newItemSize); err != nil {
			return err
		}
	}

	shard := e.getShard(key)
	freedBytes, addedBytes := shard.Put(key, item)

	netMemoryDelta := addedBytes - freedBytes
	atomic.AddInt64(&e.usedMemory, netMemoryDelta)

	return nil
}

// Del deletes one or more keys from the cache. Returns count of deleted keys.
func (e *Engine) Del(keys ...string) int {
	count := 0
	for _, key := range keys {
		shard := e.getShard(key)
		if freed, ok := shard.Delete(key); ok {
			atomic.AddInt64(&e.usedMemory, -freed)
			count++
		}
	}
	return count
}

// Exists checks how many of the specified keys exist.
func (e *Engine) Exists(keys ...string) int {
	count := 0
	for _, key := range keys {
		shard := e.getShard(key)
		if _, found := shard.Get(key); found {
			count++
		}
	}
	return count
}

// Expire sets an expiration TTL (in milliseconds) on a key.
func (e *Engine) Expire(key string, ttlMs int64) bool {
	shard := e.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	item, ok := shard.items[key]
	if !ok || item.IsExpired() {
		return false
	}

	if ttlMs > 0 {
		item.ExpiresAt = time.Now().UnixMilli() + ttlMs
	} else {
		item.ExpiresAt = 0
	}
	return true
}

// TTL returns remaining TTL in milliseconds for a key.
// Returns -2 if key does not exist, -1 if key has no associated expire.
func (e *Engine) TTL(key string) (int64, bool) {
	shard := e.getShard(key)
	shard.mu.RLock()
	item, ok := shard.items[key]
	shard.mu.RUnlock()

	if !ok || item.IsExpired() {
		return -2, false
	}

	if item.ExpiresAt <= 0 {
		return -1, true
	}

	remaining := item.ExpiresAt - time.Now().UnixMilli()
	if remaining < 0 {
		return -2, false
	}

	return remaining, true
}

// Persist removes the expiration from a key.
func (e *Engine) Persist(key string) bool {
	shard := e.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	item, ok := shard.items[key]
	if !ok || item.IsExpired() {
		return false
	}

	if item.ExpiresAt == 0 {
		return false
	}

	item.ExpiresAt = 0
	return true
}

// IncrBy increments the integer value of a key by delta.
func (e *Engine) IncrBy(key string, delta int64) (int64, error) {
	shard := e.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	var valInt int64 = 0
	item, ok := shard.items[key]

	if ok && !item.IsExpired() {
		parsed, err := strconv.ParseInt(utils.BytesToString(item.Value), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("ERR value is not an integer or out of range")
		}
		valInt = parsed
	}

	valInt += delta
	newValStr := strconv.FormatInt(valInt, 10)
	newItem := NewItem(utils.StringToBytes(newValStr), 0)

	if ok {
		if item.ExpiresAt > 0 && !item.IsExpired() {
			newItem.ExpiresAt = item.ExpiresAt
		}
		freed := item.MemorySize(key)
		shard.items[key] = newItem
		added := newItem.MemorySize(key)
		atomic.AddInt64(&e.usedMemory, added-freed)
	} else {
		shard.items[key] = newItem
		added := newItem.MemorySize(key)
		atomic.AddInt64(&e.usedMemory, added)
	}

	return valInt, nil
}

// FlushDB clears all keys across all shards.
func (e *Engine) FlushDB() {
	for i := 0; i < NumShards; i++ {
		e.shards[i].mu.Lock()
		e.shards[i].items = make(map[string]*Item, 1024)
		e.shards[i].mu.Unlock()
	}
	atomic.StoreInt64(&e.usedMemory, 0)
}

// DBSize returns total number of unexpired keys across all shards.
func (e *Engine) DBSize() int64 {
	var total int64
	for i := 0; i < NumShards; i++ {
		total += int64(e.shards[i].Count())
	}
	return total
}

// Keys returns all keys matching pattern across all shards.
func (e *Engine) Keys(pattern string) []string {
	var allKeys []string
	for i := 0; i < NumShards; i++ {
		keys := e.shards[i].Keys(pattern)
		allKeys = append(allKeys, keys...)
	}
	return allKeys
}

// Stats returns hit/miss/memory operational stats.
func (e *Engine) Stats() (hits int64, misses int64, usedMem int64, keysCount int64) {
	return atomic.LoadInt64(&e.hits), atomic.LoadInt64(&e.misses), atomic.LoadInt64(&e.usedMemory), e.DBSize()
}

// Iterate calls fn for all unexpired key-value entries across all shards.
func (e *Engine) Iterate(fn func(key string, val []byte, expiresAt int64) bool) {
	for i := 0; i < NumShards; i++ {
		shard := e.shards[i]
		shard.mu.RLock()
		for k, item := range shard.items {
			if !item.IsExpired() {
				if !fn(k, item.Value, item.ExpiresAt) {
					shard.mu.RUnlock()
					return
				}
			}
		}
		shard.mu.RUnlock()
	}
}

// evictMemory runs eviction logic when used memory exceeds limits.
func (e *Engine) evictMemory(neededBytes int64) error {
	if eviction.IsNoEviction(e.evictionPolicy) {
		return eviction.ErrOOM
	}

	accessors := make([]*eviction.ShardAccessor, NumShards)
	for i := 0; i < NumShards; i++ {
		idx := i
		accessors[i] = &eviction.ShardAccessor{
			GetRandomItem: func(r *rand.Rand, volatileOnly bool) *eviction.EvictableItem {
				shard := e.shards[idx]
				shard.mu.RLock()
				defer shard.mu.RUnlock()

				if len(shard.items) == 0 {
					return nil
				}

				// Pick random entry
				targetIdx := r.Intn(len(shard.items))
				curr := 0
				for k, item := range shard.items {
					if curr == targetIdx {
						if volatileOnly && item.ExpiresAt <= 0 {
							return nil
						}
						return &eviction.EvictableItem{
							Key:        k,
							LastAccess: item.LastAccess,
							AccessCnt:  item.AccessCount,
							ExpiresAt:  item.ExpiresAt,
							Size:       item.MemorySize(k),
							ShardIdx:   idx,
						}
					}
					curr++
				}
				return nil
			},
		}
	}

	attempts := 0
	volatileOnly := e.evictionPolicy == "volatile-lru" || e.evictionPolicy == "volatile-lfu"
	isLFU := e.evictionPolicy == "allkeys-lfu" || e.evictionPolicy == "volatile-lfu"

	for atomic.LoadInt64(&e.usedMemory)+neededBytes > e.maxMemory && attempts < 100 {
		attempts++
		var key string
		var shardIdx int
		var found bool

		if isLFU {
			key, shardIdx, _, found = eviction.SampleLFU(accessors, 5, volatileOnly)
		} else {
			key, shardIdx, _, found = eviction.SampleLRU(accessors, 5, volatileOnly)
		}

		if !found || key == "" {
			break
		}

		shard := e.shards[shardIdx]
		if freed, deleted := shard.Delete(key); deleted {
			atomic.AddInt64(&e.usedMemory, -freed)
		}
	}

	if atomic.LoadInt64(&e.usedMemory)+neededBytes > e.maxMemory {
		return eviction.ErrOOM
	}

	return nil
}

// activeExpirationLoop periodically tests random keys for TTL expiration.
func (e *Engine) activeExpirationLoop() {
	defer e.wg.Done()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.sampleAndExpireKeys(r)
		}
	}
}

func (e *Engine) sampleAndExpireKeys(r *rand.Rand) {
	for i := 0; i < NumShards; i++ {
		shard := e.shards[i]
		shard.mu.Lock()

		if len(shard.items) == 0 {
			shard.mu.Unlock()
			continue
		}

		sampleCount := 20
		if len(shard.items) < sampleCount {
			sampleCount = len(shard.items)
		}

		expiredCount := 0
		now := time.Now().UnixMilli()

		// Random key sampling
		for k, item := range shard.items {
			if sampleCount <= 0 {
				break
			}
			sampleCount--

			if item.ExpiresAt > 0 && now >= item.ExpiresAt {
				freed := item.MemorySize(k)
				delete(shard.items, k)
				atomic.AddInt64(&e.usedMemory, -freed)
				expiredCount++
			}
		}

		shard.mu.Unlock()
	}
}
