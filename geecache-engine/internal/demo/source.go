// Package demo provides a finite, read-only source shared by all demo nodes.
package demo

import (
	"context"
	"fmt"
	"github.com/X1Kun/simpleCache/geecache-engine/internal/cache"
	"sort"
	"time"
)

const AutoKeys = 10000

type Source struct {
	values map[string]string
	delay  time.Duration
}

func New(delay time.Duration) *Source {
	if delay < 0 {
		panic("demo delay must be nonnegative")
	}
	values := map[string]string{"Tom": "630", "Jack": "589", "Sam": "567"}
	for i := 0; i < AutoKeys; i++ {
		key := fmt.Sprintf("Auto-%d", i)
		values[key] = "Value-for-" + key
	}
	return &Source{values: values, delay: delay}
}

func (s *Source) Keys() []string {
	keys := make([]string, 0, len(s.values))
	for key := range s.values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (s *Source) Get(ctx context.Context, key string) ([]byte, error) {
	timer := time.NewTimer(s.delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
	}
	value, ok := s.values[key]
	if !ok {
		return nil, fmt.Errorf("%w: %s", cache.ErrNotFound, key)
	}
	return []byte(value), nil
}
