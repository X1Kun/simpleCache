package demo

import (
	"context"
	"errors"
	"github.com/X1Kun/simpleCache/geecache-engine/internal/cache"
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
		if _, err := source.Get(context.Background(), key); err != nil {
			t.Fatalf("enumerated key %q is missing: %v", key, err)
		}
	}
	group := cache.NewGroup(t.Name(), 1<<20, source, cache.Options{KnownKeys: keys})
	for key, want := range map[string]string{"Tom": "630", "Auto-0": "Value-for-Auto-0", "Auto-9999": "Value-for-Auto-9999"} {
		for i := 0; i < 2; i++ {
			value, err := group.Get(context.Background(), key)
			if err != nil || value.String() != want {
				t.Fatalf("%s: value=%q error=%v", key, value.String(), err)
			}
		}
	}
	for _, key := range []string{"Auto-10000", "missing", "Auto--1"} {
		if _, err := group.Get(context.Background(), key); !errors.Is(err, cache.ErrNotFound) {
			t.Fatalf("%q must be missing, got %v", key, err)
		}
	}
	keys[0] = "changed"
	if source.Keys()[0] == "changed" {
		t.Fatal("Keys must not expose mutable source state")
	}
}

func TestSizedFixtureDoesNotChangeDefaultValues(t *testing.T) {
	source := NewSized(0, 4096)
	for _, key := range []string{"Auto-0", "Auto-9999"} {
		value, err := source.Get(context.Background(), key)
		if err != nil || len(value) != 4096 || string(value) != AutoValue(key, 4096) {
			t.Fatalf("fixture %s: %v", key, err)
		}
	}
	value, err := New(0).Get(context.Background(), "Auto-0")
	if err != nil || string(value) != "Value-for-Auto-0" {
		t.Fatal("default source changed")
	}
	value, err = source.Get(context.Background(), "Tom")
	if err != nil || string(value) != "630" {
		t.Fatal("named demo value changed")
	}
}

func TestSizedFixtureRejectsHostScaleAllocations(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("oversized fixture accepted")
		}
	}()
	NewSized(0, MaxDiagnosticValueBytes+1)
}
