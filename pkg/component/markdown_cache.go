package component

import (
	"container/list"
	"context"
	"crypto/sha256"
	"sync"

	"github.com/hyperbricks/hyperbricks/pkg/markdown"
)

// Limits cover retained HTML bytes and entry count, not temporary conversion
// allocations. The cache belongs to one renderer/runtime and never stores input
// text, paths, request metadata, wrappers, or errors.
const markdownCacheBytes = 8 << 20
const markdownCacheEntries = 256

type markdownCacheEntry struct {
	key  [32]byte
	html string
}

type markdownCache struct {
	mu      sync.Mutex
	entries map[[32]byte]*list.Element
	lru     list.List
	bytes   int
}

func (c *markdownCache) render(content []byte, ctx context.Context) string {
	key := sha256.Sum256(content)
	c.mu.Lock()
	if e := c.entries[key]; e != nil {
		c.lru.MoveToFront(e)
		out := e.Value.(markdownCacheEntry).html
		c.mu.Unlock()
		return out
	}
	c.mu.Unlock()
	// Conversion does not hold the cache lock: unrelated documents can render
	// concurrently. Concurrent misses may convert the same input independently.
	out := markdown.Render(content)
	if len(out) > markdownCacheBytes || (ctx != nil && ctx.Err() != nil) {
		return out
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.entries[key]; e != nil {
		c.lru.MoveToFront(e)
		return e.Value.(markdownCacheEntry).html
	}
	if c.entries == nil {
		c.entries = make(map[[32]byte]*list.Element)
	}
	for c.lru.Len() >= markdownCacheEntries || c.bytes+len(out) > markdownCacheBytes {
		e := c.lru.Back()
		entry := e.Value.(markdownCacheEntry)
		delete(c.entries, entry.key)
		c.bytes -= len(entry.html)
		c.lru.Remove(e)
	}
	c.entries[key] = c.lru.PushFront(markdownCacheEntry{key: key, html: out})
	c.bytes += len(out)
	return out
}
