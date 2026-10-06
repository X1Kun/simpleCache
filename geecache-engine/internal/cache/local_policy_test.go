package cache

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestKnownKeysBloom(t *testing.T) {
	keys := make([]string, 1000)
	for i := range keys {
		keys[i] = fmt.Sprintf("Auto-%d", i)
	}
	bf := newBloomForKeys(keys)
	for _, key := range keys {
		if !bf.Contains(key) {
			t.Fatalf("known key %q was rejected", key)
		}
	}
	calls := 0
	g := NewGroupWithOptions(t.Name(), 1024, GetterFunc(func(key string) ([]byte, error) {
		calls++
		return []byte(key), nil
	}), GroupOptions{KnownKeys: keys})
	// Choose a definite negative: Bloom false positives are allowed.
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("missing-%d", i)
		if !bf.Contains(key) {
			if _, err := g.Get(key); !errors.Is(err, ErrNotFound) || calls != 0 {
				t.Fatalf("definite negative called source or lost error: calls=%d err=%v", calls, err)
			}
			return
		}
	}
	t.Fatal("filter failed to reject any negative in the sample")
}

func TestLocalValueIsolationAndLimits(t *testing.T) {
	input := []byte("original")
	g := NewGroup(t.Name(), 2<<20, GetterFunc(func(key string) ([]byte, error) {
		switch key {
		case "max":
			return make([]byte, MaxValueBytes), nil
		case "oversized":
			return make([]byte, MaxValueBytes+1), nil
		default:
			return input, nil
		}
	}))
	view, err := g.Get("key")
	if err != nil {
		t.Fatal(err)
	}
	input[0] = 'X'
	copy := view.ByteSlice()
	copy[1] = 'X'
	if cached, err := g.Get("key"); err != nil || cached.String() != "original" {
		t.Fatal("getter input or returned slice mutated cached data")
	}
	if value, err := g.Get("max"); err != nil || value.Len() != MaxValueBytes {
		t.Fatalf("exact value boundary rejected: %v", err)
	}
	if _, err := g.Get("oversized"); !errors.Is(err, ErrValueTooLarge) {
		t.Fatalf("oversized value not rejected: %v", err)
	}
	if _, ok := g.mainCache.get("oversized"); ok {
		t.Fatal("oversized value was cached")
	}
	for _, key := range []string{"", strings.Repeat("k", 257)} {
		if _, err := g.Get(key); !errors.Is(err, ErrInvalidKey) {
			t.Fatalf("invalid key accepted: %v", err)
		}
	}
}

func TestGroupCapacityReload(t *testing.T) {
	calls := 0
	g := NewGroupWithOptions(t.Name(), 12, GetterFunc(func(key string) ([]byte, error) {
		calls++
		return []byte("12345"), nil
	}), GroupOptions{TTL: time.Hour})
	for _, key := range []string{"a", "b", "c", "a"} {
		value, err := g.Get(key)
		if err != nil || value.String() != "12345" {
			t.Fatalf("eviction/reload lost value: %v", err)
		}
	}
	if calls != 4 {
		t.Fatalf("evicted key did not reload: calls=%d", calls)
	}
	g.mainCache.mu.Lock()
	defer g.mainCache.mu.Unlock()
	if g.mainCache.lru.Len() != 2 {
		t.Fatal("cache capacity not enforced")
	}
}
