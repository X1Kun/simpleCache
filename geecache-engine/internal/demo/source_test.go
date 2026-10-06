package demo

import (
	"errors"
	geecache "github.com/X1Kun/simpleCache/geecache-engine/internal/cache"
	"testing"
)

func TestFiniteSourceAndBloomAgree(t *testing.T) {
	source := New(0)
	keys := source.Keys()
	if len(keys) != AutoKeys+3 {
		t.Fatalf("unexpected dataset size: %d", len(keys))
	}
	// Walk the finite source directly, then sample the actual cache path.
	for _, key := range keys {
		if _, err := source.Get(key); err != nil {
			t.Fatalf("enumerated key %q is missing: %v", key, err)
		}
	}
	group := geecache.NewGroupWithOptions(t.Name(), 1<<20, source, geecache.GroupOptions{KnownKeys: keys})
	for key, want := range map[string]string{"Tom": "630", "Auto-0": "Value-for-Auto-0", "Auto-9999": "Value-for-Auto-9999"} {
		for i := 0; i < 2; i++ {
			value, err := group.Get(key)
			if err != nil || value.String() != want {
				t.Fatalf("%s: value=%q error=%v", key, value.String(), err)
			}
		}
	}
	for _, key := range []string{"Auto-10000", "missing", "Auto--1"} {
		if _, err := group.Get(key); !errors.Is(err, geecache.ErrNotFound) {
			t.Fatalf("%q must be missing, got %v", key, err)
		}
	}
	keys[0] = "changed"
	if source.Keys()[0] == "changed" {
		t.Fatal("Keys must not expose mutable source state")
	}
}
