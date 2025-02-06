package pokecache

import (
	"testing"
	"time"
)

func TestCacheAddGet(t *testing.T) {
	cache := NewCache(5 * time.Second)
	key := "test"
	val := []byte("value")

	cache.Add(key, val)

	if retrieved, ok := cache.Get(key); !ok || string(retrieved) != "value" {
		t.Errorf("Cache failed to retrieve added value")
	}
}

func TestCacheReap(t *testing.T) {
	interval := 100 * time.Millisecond
	cache := NewCache(interval)
	key := "test"
	val := []byte("value")

	cache.Add(key, val)

	time.Sleep(interval + 10*time.Millisecond)

	if _, ok := cache.Get(key); ok {
		t.Errorf("Cache entry not reaped")
	}
}
