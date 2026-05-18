package runtime

import (
	"sync"
)

// tagCache holds a per-Remote mapping of inbound tag → panel-side
// numeric id. It is concurrency-safe.
//
// The cache is refresh-on-miss: when a caller asks for a tag the
// cache hasn't seen yet, the Remote calls /list, repopulates the
// entire cache, and the original lookup retries.
type tagCache struct {
	mu   sync.RWMutex
	tags map[string]int64
}

func newTagCache() *tagCache {
	return &tagCache{tags: make(map[string]int64)}
}

func (c *tagCache) Get(tag string) (int64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	id, ok := c.tags[tag]
	return id, ok
}

// Replace swaps the cache contents in one atomic operation.
func (c *tagCache) Replace(m map[string]int64) {
	c.mu.Lock()
	c.tags = m
	c.mu.Unlock()
}

// Set inserts / overwrites one entry. Used by AddInbound to register
// a freshly-created inbound's tag→id without forcing a full /list
// refresh.
func (c *tagCache) Set(tag string, id int64) {
	c.mu.Lock()
	c.tags[tag] = id
	c.mu.Unlock()
}

// Delete removes one entry. Used by DeleteInbound after the panel
// confirms.
func (c *tagCache) Delete(tag string) {
	c.mu.Lock()
	delete(c.tags, tag)
	c.mu.Unlock()
}

// Snapshot returns a copy of the cache contents (for diagnostics).
func (c *tagCache) Snapshot() map[string]int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]int64, len(c.tags))
	for k, v := range c.tags {
		out[k] = v
	}
	return out
}
