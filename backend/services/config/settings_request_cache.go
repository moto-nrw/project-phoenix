package config

import (
	"context"
	"sync"
)

type settingsRequestCacheContextKey struct{}

type requestCacheEntryKey struct {
	tenantID   int64
	settingKey string
}

// settingsRequestCache memoizes resolved setting values for the lifetime of a
// single request context. Entries are keyed by (tenant_id, setting_key), so a
// cache attached to a context that touches several tenants (operator flows,
// *ForTenant resolves) can never serve one tenant's value to another.
//
// Consistency contract (see also the Lock* helpers in settings_service.go):
//   - lifetime is one request; the cache dies with the request context
//   - SetValue/ResetValue evict the written key, so same-request reads after a
//     write (including side-effect hooks running in the same transaction) see
//     the new value
//   - the advisory-lock helpers (LockSlotListCutoffPair, LockClassCollectionPair)
//     flush the whole tenant bucket, so post-lock reads observe a concurrent
//     writer's committed state instead of a pre-lock memoized value
//   - writes committed by other transactions become visible at the latest with
//     the next request
type settingsRequestCache struct {
	mu      sync.Mutex
	entries map[requestCacheEntryKey]snapshotValue
}

// WithSettingsRequestCache attaches a request-scoped settings memo cache to
// ctx. Idempotent: a context that already carries a cache is returned
// unchanged, so stacked attachment points (router-wide and group-wide
// middleware) share one cache instead of shadowing each other.
func WithSettingsRequestCache(ctx context.Context) context.Context {
	if requestCacheFromContext(ctx) != nil {
		return ctx
	}
	return context.WithValue(ctx, settingsRequestCacheContextKey{}, &settingsRequestCache{
		entries: make(map[requestCacheEntryKey]snapshotValue),
	})
}

func requestCacheFromContext(ctx context.Context) *settingsRequestCache {
	cache, _ := ctx.Value(settingsRequestCacheContextKey{}).(*settingsRequestCache)
	return cache
}

// lookup returns the cached entries for keys plus the deduplicated list of
// keys that have no cache entry yet.
func (c *settingsRequestCache) lookup(tenantID int64, keys []string) (map[string]snapshotValue, []string) {
	hits := make(map[string]snapshotValue, len(keys))
	missing := make([]string, 0, len(keys))
	seenMissing := make(map[string]struct{}, len(keys))

	c.mu.Lock()
	defer c.mu.Unlock()
	for _, key := range keys {
		if _, hit := hits[key]; hit {
			continue
		}
		if _, miss := seenMissing[key]; miss {
			continue
		}
		if entry, ok := c.entries[requestCacheEntryKey{tenantID: tenantID, settingKey: key}]; ok {
			hits[key] = entry
			continue
		}
		seenMissing[key] = struct{}{}
		missing = append(missing, key)
	}
	return hits, missing
}

// store memoizes freshly resolved entries for tenantID.
func (c *settingsRequestCache) store(tenantID int64, values map[string]snapshotValue) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, entry := range values {
		c.entries[requestCacheEntryKey{tenantID: tenantID, settingKey: key}] = entry
	}
}

// evictKey drops a single (tenant, key) entry after a write to that key.
func (c *settingsRequestCache) evictKey(tenantID int64, key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, requestCacheEntryKey{tenantID: tenantID, settingKey: key})
}

// evictTenant drops every entry for tenantID. Called by the advisory-lock
// helpers: taking a lock is the freshness barrier of the cross-field guards,
// and which keys a guard re-reads under the lock is not this cache's business
// to enumerate (#1565/#1663).
func (c *settingsRequestCache) evictTenant(tenantID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key := range c.entries {
		if key.tenantID == tenantID {
			delete(c.entries, key)
		}
	}
}
