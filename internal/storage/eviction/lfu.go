package eviction

import (
	"math/rand"
	"time"
)

// SampleLFU samples random keys from shards and returns the least frequently used key.
func SampleLFU(shards []*ShardAccessor, sampleSize int, volatileOnly bool) (string, int, int64, bool) {
	var lfuItem EvictableItem
	found := false

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < sampleSize; i++ {
		shardIdx := r.Intn(len(shards))
		item := shards[shardIdx].GetRandomItem(r, volatileOnly)
		if item == nil {
			continue
		}

		if !found || item.AccessCnt < lfuItem.AccessCnt {
			lfuItem = *item
			found = true
		}
	}

	if !found {
		return "", -1, 0, false
	}

	return lfuItem.Key, lfuItem.ShardIdx, lfuItem.Size, true
}
