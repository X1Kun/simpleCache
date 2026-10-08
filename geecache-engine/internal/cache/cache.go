package cache

import (
	"github.com/X1Kun/simpleCache/geecache-engine/internal/cache/lru"
	"sync"
	"time"
)

// cache adds synchronization and a byte limit to the LRU.
type cache struct {
	mu         sync.Mutex
	lru        *lru.Cache
	cacheBytes int64
}

// add inserts an entry while holding the cache lock.
// Initialize the LRU lazily, then insert the value.
func (c *cache) add(key string, value ByteView, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// lazy initialization
	if c.lru == nil {
		c.lru = lru.New(c.cacheBytes, nil)
	}
	c.lru.Add(key, value, ttl)
}

// get retrieves an entry while holding the cache lock.
// An uninitialized LRU is empty; successful reads update recency.
func (c *cache) get(key string) (ByteView, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil {
		return ByteView{}, false
	}
	if value, ok := c.lru.Get(key); ok {
		return value.(ByteView), true
	}
	return ByteView{}, false
}
