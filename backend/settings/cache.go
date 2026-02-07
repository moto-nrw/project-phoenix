package settings

import (
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/models/config"
)

// CacheConfig holds cache configuration options
type CacheConfig struct {
	// TTL is the time-to-live for cached entries
	TTL time.Duration
	// MaxEntries is the maximum number of entries (0 = unlimited)
	MaxEntries int
	// Enabled controls whether caching is active
	Enabled bool
}

// DefaultCacheConfig returns sensible defaults for the cache
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		TTL:        5 * time.Minute,
		MaxEntries: 1000,
		Enabled:    true,
	}
}

// cacheEntry holds a cached value with expiration
type cacheEntry struct {
	value     *config.ResolvedSetting
	expiresAt time.Time
}

// Cache provides thread-safe caching for resolved settings
type Cache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	config  CacheConfig
}

// NewCache creates a new settings cache
func NewCache(cfg CacheConfig) *Cache {
	c := &Cache{
		entries: make(map[string]*cacheEntry),
		config:  cfg,
	}

	// Start background cleanup goroutine
	if cfg.Enabled {
		go c.cleanupLoop()
	}

	return c
}

// cacheKey generates a unique key for a setting+scope combination
func cacheKey(key string, scopeCtx *config.ScopeContext) string {
	result := key
	if scopeCtx != nil {
		if scopeCtx.AccountID != nil {
			result += ":user:" + string(rune(*scopeCtx.AccountID))
		}
		if scopeCtx.DeviceID != nil {
			result += ":device:" + string(rune(*scopeCtx.DeviceID))
		}
	}
	return result
}

// Get retrieves a cached setting value
func (c *Cache) Get(key string, scopeCtx *config.ScopeContext) (*config.ResolvedSetting, bool) {
	if !c.config.Enabled {
		return nil, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	cKey := cacheKey(key, scopeCtx)
	entry, exists := c.entries[cKey]
	if !exists {
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		return nil, false
	}

	return entry.value, true
}

// Set stores a setting value in the cache
func (c *Cache) Set(key string, scopeCtx *config.ScopeContext, value *config.ResolvedSetting) {
	if !c.config.Enabled {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Enforce max entries limit
	if c.config.MaxEntries > 0 && len(c.entries) >= c.config.MaxEntries {
		c.evictOldest()
	}

	cKey := cacheKey(key, scopeCtx)
	c.entries[cKey] = &cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(c.config.TTL),
	}
}

// Invalidate removes a specific setting from the cache (all scope combinations)
func (c *Cache) Invalidate(key string) {
	if !c.config.Enabled {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Remove all entries that start with this key
	for cKey := range c.entries {
		if len(cKey) >= len(key) && cKey[:len(key)] == key {
			delete(c.entries, cKey)
		}
	}
}

// InvalidateForScope removes entries for a specific scope
func (c *Cache) InvalidateForScope(key string, scope config.Scope, scopeID *int64) {
	if !c.config.Enabled {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// For system scope, invalidate all entries for this key
	if scope == config.ScopeSystem {
		for cKey := range c.entries {
			if len(cKey) >= len(key) && cKey[:len(key)] == key {
				delete(c.entries, cKey)
			}
		}
		return
	}

	// For user/device scope, we need to invalidate specific scope entries
	// and all entries that might inherit from this scope
	for cKey := range c.entries {
		if len(cKey) >= len(key) && cKey[:len(key)] == key {
			delete(c.entries, cKey)
		}
	}
}

// Clear removes all entries from the cache
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*cacheEntry)
}

// Stats returns cache statistics
func (c *Cache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	expired := 0
	now := time.Now()
	for _, entry := range c.entries {
		if now.After(entry.expiresAt) {
			expired++
		}
	}

	return CacheStats{
		TotalEntries:   len(c.entries),
		ExpiredEntries: expired,
		Enabled:        c.config.Enabled,
	}
}

// CacheStats holds cache statistics
type CacheStats struct {
	TotalEntries   int
	ExpiredEntries int
	Enabled        bool
}

// evictOldest removes the oldest entry (must be called with lock held)
func (c *Cache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.entries {
		if oldestKey == "" || entry.expiresAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.expiresAt
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

// cleanupLoop periodically removes expired entries
func (c *Cache) cleanupLoop() {
	ticker := time.NewTicker(c.config.TTL / 2)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

// cleanup removes expired entries
func (c *Cache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, key)
		}
	}
}
