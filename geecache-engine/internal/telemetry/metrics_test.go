package telemetry

import (
	"context"
	"testing"

	"github.com/X1Kun/simpleCache/geecache-engine/internal/cache"
)

func TestLogicalCacheMetrics(t *testing.T) {
	m := New()
	g := cache.NewGroup("test", 8, cache.GetterFunc(func(context.Context, string) ([]byte, error) { return []byte("value"), nil }), cache.Options{})
	m.ObserveCache(g)
	for _, key := range []string{"a", "b"} {
		if _, err := g.Get(context.Background(), key); err != nil {
			t.Fatal(err)
		}
	}
	families, err := m.Registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]float64{}
	for _, family := range families {
		for _, metric := range family.Metric {
			if metric.Gauge != nil {
				values[family.GetName()] = metric.Gauge.GetValue()
			}
			if metric.Counter != nil {
				values[family.GetName()] = metric.Counter.GetValue()
			}
		}
	}
	for name, want := range map[string]float64{"simplecache_cache_bytes": 6, "simplecache_cache_capacity_bytes": 8, "simplecache_cache_entries": 1, "simplecache_cache_removals_total": 1} {
		if values[name] != want {
			t.Errorf("%s=%v want %v", name, values[name], want)
		}
	}
}
