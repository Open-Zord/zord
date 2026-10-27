// Package cache is a Zord battery: a small, dependency-free in-memory cache
// with per-entry TTL. `zord package add cache` copies this package into a
// project's pkg/cache and wires NewCache() into the registry under RegistryKey.
package cache

import (
	"sync"
	"time"
)

// RegistryKey is the key under which the cache is registered in the registry.
const RegistryKey = "cache"

// entry is a stored value with an optional expiration. A zero expiresAt means
// the entry never expires.
type entry struct {
	value     any
	expiresAt time.Time
}

// Cache is a concurrency-safe in-memory key/value store with lazy TTL
// expiration — expired entries are dropped on read, so no background goroutine
// is needed.
type Cache struct {
	mu    sync.RWMutex
	items map[string]entry
}

// NewCache returns an empty cache ready to use.
func NewCache() *Cache {
	return &Cache{items: make(map[string]entry)}
}

// Set stores value under key. A ttl > 0 expires the entry after that duration;
// a ttl <= 0 stores it without expiration.
func (c *Cache) Set(key string, value any, ttl time.Duration) {
	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}
	c.mu.Lock()
	c.items[key] = entry{value: value, expiresAt: expiresAt}
	c.mu.Unlock()
}

// Get returns the value stored under key and whether it was present and
// unexpired. An expired entry is treated as absent and removed.
func (c *Cache) Get(key string) (any, bool) {
	c.mu.RLock()
	e, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if !e.expiresAt.IsZero() && time.Now().After(e.expiresAt) {
		c.Delete(key)
		return nil, false
	}
	return e.value, true
}

// Delete removes key from the cache (no-op if absent).
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
}

// Len returns the number of entries currently stored, including any that are
// expired but not yet evicted.
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}
