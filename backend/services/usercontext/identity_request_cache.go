package usercontext

import (
	"context"
	"maps"
	"slices"
	"sync"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type identityRequestCacheContextKey struct{}

type identityCacheKey struct {
	tenantID  int64
	accountID int64
}

// identityRequestCache memoizes the identity chain (account → person → staff
// → teacher → groups/substitutions) for the lifetime of a single request
// context (issue #2099, modeled on the settings request cache from #2065).
// Entries are keyed by (tenant_id, account_id), so a cache attached to a
// context that touches several tenants can never serve one tenant's identity
// to another.
//
// Consistency contract:
//   - lifetime is one request; the cache dies with the request context
//   - only successful outcomes and clean not-found results (account not
//     linked to a person, person not staff, staff not a teacher) are
//     memoized; DB/transport errors and GetMyGroups partial failures never are
//   - UpdateCurrentProfile and UpdateAvatar drop the caller's whole entry
//     before their trailing GetCurrentProfile re-read, so a same-request read
//     after a self-write sees committed state
//   - deliberately exempt from eviction: group transfer/cancel
//     (api/groups) writes a substitution for a DIFFERENT staff member
//     (self-transfer is rejected) and renders without re-reading identity;
//     admin substitution endpoints, staff offboarding, and invitation
//     acceptance mutate another account's chain or run for an account that is
//     not authenticated in that request
//   - writes committed by other transactions become visible at the latest
//     with the next request
//   - entries are not date-keyed: timezone.TodayDate() is stable within a
//     request span, and the only long-lived context (SSE) resolves identity
//     once at connection setup and never re-reads
//   - there is deliberately NO process-wide cache and NO TTL
type identityRequestCache struct {
	mu      sync.Mutex
	entries map[identityCacheKey]*identityEntry
}

// identityEntry holds the memoized stages for one (tenant, account) pair.
// Each stage has its own loaded flag; a loaded stage with a nil value is a
// clean "not linked" outcome, not an error.
type identityEntry struct {
	accountLoaded bool
	account       *auth.Account
	personLoaded  bool
	person        *users.Person
	staffLoaded   bool
	staff         *users.Staff
	teacherLoaded bool
	teacher       *users.Teacher
	groupsLoaded  bool
	groups        []*education.Group
	subsLoaded    bool
	subs          map[int64]bool
}

// WithIdentityRequestCache attaches a request-scoped identity memo cache to
// ctx. Idempotent: a context that already carries a cache is returned
// unchanged, so stacked attachment points (router-wide and group-wide
// middleware) share one cache instead of shadowing each other.
func WithIdentityRequestCache(ctx context.Context) context.Context {
	if identityRequestCacheFromContext(ctx) != nil {
		return ctx
	}
	return context.WithValue(ctx, identityRequestCacheContextKey{}, &identityRequestCache{
		entries: make(map[identityCacheKey]*identityEntry),
	})
}

func identityRequestCacheFromContext(ctx context.Context) *identityRequestCache {
	cache, _ := ctx.Value(identityRequestCacheContextKey{}).(*identityRequestCache)
	return cache
}

// identityMemo resolves the request cache and the entry key for the current
// caller. ok is false when no cache is attached (scheduler, CLI, device auth,
// tests without the middleware) or when the context carries no authenticated
// account — both mean "resolve without memoization".
func identityMemo(ctx context.Context) (*identityRequestCache, identityCacheKey, bool) {
	cache := identityRequestCacheFromContext(ctx)
	if cache == nil {
		return nil, identityCacheKey{}, false
	}
	accountID := int64(jwt.ClaimsFromCtx(ctx).ID)
	if accountID <= 0 {
		return nil, identityCacheKey{}, false
	}
	return cache, identityCacheKey{tenantID: tenant.FromContext(ctx), accountID: accountID}, true
}

// invalidateIdentity drops every memoized stage for the current caller. The
// self-mutating profile methods call it before their trailing re-read.
func invalidateIdentity(ctx context.Context) {
	cache, key, ok := identityMemo(ctx)
	if !ok {
		return
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	delete(cache.entries, key)
}

// entryLocked returns the entry for key, creating it on demand. Callers must
// hold c.mu.
func (c *identityRequestCache) entryLocked(key identityCacheKey) *identityEntry {
	entry, ok := c.entries[key]
	if !ok {
		entry = &identityEntry{}
		c.entries[key] = entry
	}
	return entry
}

func (c *identityRequestCache) accountFor(key identityCacheKey) (*auth.Account, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || !entry.accountLoaded {
		return nil, false
	}
	return entry.account, true
}

func (c *identityRequestCache) storeAccount(key identityCacheKey, account *auth.Account) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := c.entryLocked(key)
	entry.account = account
	entry.accountLoaded = true
}

func (c *identityRequestCache) personFor(key identityCacheKey) (*users.Person, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || !entry.personLoaded {
		return nil, false
	}
	return entry.person, true
}

func (c *identityRequestCache) storePerson(key identityCacheKey, person *users.Person) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := c.entryLocked(key)
	entry.person = person
	entry.personLoaded = true
}

func (c *identityRequestCache) staffFor(key identityCacheKey) (*users.Staff, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || !entry.staffLoaded {
		return nil, false
	}
	return entry.staff, true
}

func (c *identityRequestCache) storeStaff(key identityCacheKey, staff *users.Staff) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := c.entryLocked(key)
	entry.staff = staff
	entry.staffLoaded = true
}

func (c *identityRequestCache) teacherFor(key identityCacheKey) (*users.Teacher, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || !entry.teacherLoaded {
		return nil, false
	}
	return entry.teacher, true
}

func (c *identityRequestCache) storeTeacher(key identityCacheKey, teacher *users.Teacher) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := c.entryLocked(key)
	entry.teacher = teacher
	entry.teacherLoaded = true
}

// groupsFor returns a shallow clone on every hit: ogsgrouplive sorts the
// returned slice in place, and a caller-visible sort must not reorder the
// memoized value.
func (c *identityRequestCache) groupsFor(key identityCacheKey) ([]*education.Group, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || !entry.groupsLoaded {
		return nil, false
	}
	return slices.Clone(entry.groups), true
}

func (c *identityRequestCache) storeGroups(key identityCacheKey, groups []*education.Group) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := c.entryLocked(key)
	entry.groups = slices.Clone(groups)
	entry.groupsLoaded = true
}

// substitutedFor clones for the same reason as groupsFor: the returned map
// must not be a live alias of the memoized one.
func (c *identityRequestCache) substitutedFor(key identityCacheKey) (map[int64]bool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || !entry.subsLoaded {
		return nil, false
	}
	return maps.Clone(entry.subs), true
}

func (c *identityRequestCache) storeSubstituted(key identityCacheKey, subs map[int64]bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := c.entryLocked(key)
	entry.subs = maps.Clone(subs)
	entry.subsLoaded = true
}
