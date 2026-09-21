package identityaccess

import (
	"context"
	"sync"
)

type requestIdentityCacheContextKey struct{}

type requestIdentityKey struct {
	tenantID  int64
	accountID int64
}

// RequestIdentityCache is the request-scoped memo of the caller-context
// identity chain (#2099). Entries are keyed by (tenant, account), so a cache
// attached to a context that touches several tenants never serves one
// tenant's identity to another. What an entry memoizes, and when, is the
// caller context's contract (CallerIdentities); the cache only keys and
// guards the entries. There is deliberately no process-wide cache and no
// TTL: the cache dies with the request context.
type RequestIdentityCache struct {
	mu      sync.Mutex
	entries map[requestIdentityKey]any
}

// WithRequestIdentityCache attaches an empty request identity cache to ctx.
// It is idempotent: a context that already carries a cache is returned
// unchanged, so stacked attachment points share one cache.
func WithRequestIdentityCache(ctx context.Context) context.Context {
	if RequestIdentityCacheFrom(ctx) != nil {
		return ctx
	}
	return context.WithValue(ctx, requestIdentityCacheContextKey{}, &RequestIdentityCache{
		entries: make(map[requestIdentityKey]any),
	})
}

// RequestIdentityCacheFrom returns the cache attached to ctx, or nil.
func RequestIdentityCacheFrom(ctx context.Context) *RequestIdentityCache {
	cache, _ := ctx.Value(requestIdentityCacheContextKey{}).(*RequestIdentityCache)
	return cache
}

// Entry returns the entry of (tenant, account), creating it with create on
// first use.
func (c *RequestIdentityCache) Entry(tenantID, accountID int64, create func() any) any {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := requestIdentityKey{tenantID: tenantID, accountID: accountID}
	entry, ok := c.entries[key]
	if !ok {
		entry = create()
		c.entries[key] = entry
	}
	return entry
}

// Evict drops the entry of (tenant, account).
func (c *RequestIdentityCache) Evict(tenantID, accountID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, requestIdentityKey{tenantID: tenantID, accountID: accountID})
}
