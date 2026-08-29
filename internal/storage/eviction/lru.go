package eviction

import (
	"errors"
	"math/rand"
	"time"
)

var ErrOOM = errors.New("OOM command not allowed when used memory > 'maxmemory'")

// ItemAccess is used for sampled evaluation in LRU/LFU eviction.
type EvictableItem struct {
	Key        string
	LastAccess int64
	AccessCnt  uint32
	ExpiresAt  int64
	Size       int64
	ShardIdx   int
}

// SampleLRU samples random keys from shards and returns the best candidate for eviction.
// volatileOnly = true means only consider keys with TTL (ExpiresAt > 0).
func SampleLRU(shards []*ShardAccessor, sampleSize int, volatileOnly bool) (string, int, int64, bool) {
	var oldestItem EvictableItem
	found := false

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < sampleSize; i++ {
		shardIdx := r.Intn(len(shards))
		item := shards[shardIdx].GetRandomItem(r, volatileOnly)
		if item == nil {
			continue
		}

		if !found || item.LastAccess < oldestItem.LastAccess {
			oldestItem = *item
			found = true
		}
	}

	if !found {
		return "", -1, 0, false
	}

	return oldestItem.Key, oldestItem.ShardIdx, oldestItem.Size, true
}

// ShardAccessor provides a thread-safe sampling interface for shards.
type ShardAccessor struct {
	GetRandomItem func(r *rand.Rand, volatileOnly bool) *EvictableItem
}
