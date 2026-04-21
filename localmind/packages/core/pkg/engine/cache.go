package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// CacheEntry represents a cached completion
type CacheEntry struct {
	Completion string
	Model      string
	CreatedAt  time.Time
	Hits       int
}

// CompletionCache is an LRU cache for completions
type CompletionCache struct {
	mu       sync.RWMutex
	entries  map[string]*CacheEntry
	order    []string // LRU order (oldest first)
	maxSize  int
	ttl      time.Duration
}

// NewCompletionCache creates a new completion cache
func NewCompletionCache(maxSize int, ttl time.Duration) *CompletionCache {
	return &CompletionCache{
		entries: make(map[string]*CacheEntry),
		order:   make([]string, 0, maxSize),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// DefaultCompletionCache returns a cache with default settings
func DefaultCompletionCache() *CompletionCache {
	return NewCompletionCache(100, 30*time.Second)
}

// generateKey creates a cache key from prefix and language
func generateKey(prefix, language string) string {
	// Use last 200 chars of prefix for cache key
	if len(prefix) > 200 {
		prefix = prefix[len(prefix)-200:]
	}
	
	data := prefix + "|" + language
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16]) // 32 char hex string
}

// Get retrieves a completion from cache
func (c *CompletionCache) Get(prefix, language string) (*CacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := generateKey(prefix, language)
	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}

	// Check TTL
	if time.Since(entry.CreatedAt) > c.ttl {
		c.removeKey(key)
		return nil, false
	}

	// Update hit count and move to end of LRU
	entry.Hits++
	c.moveToEnd(key)

	return entry, true
}

// Set stores a completion in cache
func (c *CompletionCache) Set(prefix, language, completion, model string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := generateKey(prefix, language)

	// If key exists, update it
	if _, ok := c.entries[key]; ok {
		c.entries[key] = &CacheEntry{
			Completion: completion,
			Model:      model,
			CreatedAt:  time.Now(),
			Hits:       0,
		}
		c.moveToEnd(key)
		return
	}

	// Evict oldest if at capacity
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	// Add new entry
	c.entries[key] = &CacheEntry{
		Completion: completion,
		Model:      model,
		CreatedAt:  time.Now(),
		Hits:       0,
	}
	c.order = append(c.order, key)
}

// Invalidate removes entries that match the prefix
func (c *CompletionCache) Invalidate(prefix, language string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := generateKey(prefix, language)
	c.removeKey(key)
}

// Clear removes all entries
func (c *CompletionCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)
	c.order = make([]string, 0, c.maxSize)
}

// Size returns the current number of entries
func (c *CompletionCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// moveToEnd moves a key to the end of LRU order
func (c *CompletionCache) moveToEnd(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			c.order = append(c.order, key)
			return
		}
	}
}

// removeKey removes a key from cache
func (c *CompletionCache) removeKey(key string) {
	delete(c.entries, key)
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			return
		}
	}
}

// evictOldest removes the oldest entry
func (c *CompletionCache) evictOldest() {
	if len(c.order) == 0 {
		return
	}
	oldest := c.order[0]
	c.order = c.order[1:]
	delete(c.entries, oldest)
}
