package storage

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestEngineBasicOps(t *testing.T) {
	eng := NewEngine(10, "allkeys-lru")
	defer eng.Close()

	// Set & Get
	err := eng.Set("k1", []byte("v1"), 0)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val, found := eng.Get("k1")
	if !found || string(val) != "v1" {
		t.Errorf("Get k1 got %q, %v; want v1, true", val, found)
	}

	// Exists
	if eng.Exists("k1", "nonexistent") != 1 {
		t.Errorf("Exists expected 1")
	}

	// Del
	deleted := eng.Del("k1")
	if deleted != 1 {
		t.Errorf("Del expected 1 deleted key, got %d", deleted)
	}

	_, found = eng.Get("k1")
	if found {
		t.Errorf("k1 should have been deleted")
	}
}

func TestEngineTTLAndExpiration(t *testing.T) {
	eng := NewEngine(10, "allkeys-lru")
	defer eng.Close()

	// Set with 50ms TTL
	err := eng.Set("ttl_key", []byte("temp"), 50)
	if err != nil {
		t.Fatalf("Set with TTL failed: %v", err)
	}

	val, found := eng.Get("ttl_key")
	if !found || string(val) != "temp" {
		t.Errorf("Get ttl_key got %q, %v; want temp, true", val, found)
	}

	time.Sleep(70 * time.Millisecond)

	_, found = eng.Get("ttl_key")
	if found {
		t.Errorf("ttl_key should be expired")
	}
}

func TestEngineIncrBy(t *testing.T) {
	eng := NewEngine(10, "allkeys-lru")
	defer eng.Close()

	val, err := eng.IncrBy("counter", 1)
	if err != nil || val != 1 {
		t.Fatalf("IncrBy 1 got %d, %v; want 1, nil", val, err)
	}

	val, err = eng.IncrBy("counter", 5)
	if err != nil || val != 6 {
		t.Fatalf("IncrBy 5 got %d, %v; want 6, nil", val, err)
	}

	val, err = eng.IncrBy("counter", -2)
	if err != nil || val != 4 {
		t.Fatalf("IncrBy -2 got %d, %v; want 4, nil", val, err)
	}
}

func TestEngineConcurrentAccess(t *testing.T) {
	eng := NewEngine(64, "allkeys-lru")
	defer eng.Close()

	var wg sync.WaitGroup
	numGoroutines := 20
	numOps := 500

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(gID int) {
			defer wg.Done()
			for i := 0; i < numOps; i++ {
				key := fmt.Sprintf("key_%d_%d", gID, i)
				val := fmt.Sprintf("val_%d_%d", gID, i)

				eng.Set(key, []byte(val), 0)
				readVal, found := eng.Get(key)
				if !found || string(readVal) != val {
					t.Errorf("Concurrent Get failed for key %s", key)
				}
				eng.Del(key)
			}
		}(g)
	}

	wg.Wait()
}
