package usercontext

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func cachedCtx(accountID int, tenantID int64) context.Context {
	ctx := tenant.WithTenantID(context.Background(), tenantID)
	ctx = context.WithValue(ctx, jwt.CtxClaims, jwt.AppClaims{ID: accountID, TenantID: tenantID})
	return WithIdentityRequestCache(ctx)
}

// WithIdentityRequestCache returns its input unchanged when a cache is already
// attached, so stacked middleware shares one cache (#2099).
func TestWithIdentityRequestCacheIsIdempotent(t *testing.T) {
	once := WithIdentityRequestCache(context.Background())
	assert.Equal(t, once, WithIdentityRequestCache(once))
	assert.Same(t, identityRequestCacheFromContext(once),
		identityRequestCacheFromContext(WithIdentityRequestCache(once)))
}

func TestIdentityMemoRequiresCacheAndClaims(t *testing.T) {
	_, _, ok := identityMemo(context.Background())
	assert.False(t, ok, "no cache attached")

	_, _, ok = identityMemo(WithIdentityRequestCache(context.Background()))
	assert.False(t, ok, "cache attached but no authenticated account")

	cache, key, ok := identityMemo(cachedCtx(42, 7))
	require.True(t, ok)
	assert.NotNil(t, cache)
	assert.Equal(t, identityCacheKey{tenantID: 7, accountID: 42}, key)
}

func TestIdentityCacheStagesMissStoreHit(t *testing.T) {
	cache, key, ok := identityMemo(cachedCtx(42, 7))
	require.True(t, ok)

	_, hit := cache.personFor(key)
	assert.False(t, hit, "empty cache must miss")

	person := &users.Person{FirstName: "Memo", LastName: "Test"}
	cache.storePerson(key, person)
	got, hit := cache.personFor(key)
	require.True(t, hit)
	assert.Same(t, person, got)

	// A loaded stage with a nil value is a clean "not linked" outcome — a hit.
	cache.storeStaff(key, nil)
	staff, hit := cache.staffFor(key)
	require.True(t, hit)
	assert.Nil(t, staff)

	cache.storeTeacher(key, nil)
	teacher, hit := cache.teacherFor(key)
	require.True(t, hit)
	assert.Nil(t, teacher)
}

func TestIdentityCacheIsolatesTenantsAndAccounts(t *testing.T) {
	ctx := cachedCtx(42, 7)
	cache, key, ok := identityMemo(ctx)
	require.True(t, ok)
	cache.storePerson(key, &users.Person{FirstName: "Tenant7"})

	// Same cache instance, different tenant on the context: distinct key.
	_, otherTenantKey, ok := identityMemo(context.WithValue(tenant.WithTenantID(ctx, 8), jwt.CtxClaims, jwt.AppClaims{ID: 42, TenantID: 8}))
	require.True(t, ok)
	_, hit := cache.personFor(otherTenantKey)
	assert.False(t, hit, "another tenant's entry must not be served")

	// Different account, same tenant.
	_, hit = cache.personFor(identityCacheKey{tenantID: 7, accountID: 43})
	assert.False(t, hit, "another account's entry must not be served")
}

func TestInvalidateIdentityDropsAllStages(t *testing.T) {
	ctx := cachedCtx(42, 7)
	cache, key, ok := identityMemo(ctx)
	require.True(t, ok)

	cache.storePerson(key, &users.Person{})
	cache.storeStaff(key, &users.Staff{})
	cache.storeGroups(key, []*education.Group{{Name: "1a"}})
	cache.storeSubstituted(key, map[int64]bool{5: true})

	invalidateIdentity(ctx)

	for name, probe := range map[string]func() bool{
		"person": func() bool { _, hit := cache.personFor(key); return hit },
		"staff":  func() bool { _, hit := cache.staffFor(key); return hit },
		"groups": func() bool { _, hit := cache.groupsFor(key); return hit },
		"subs":   func() bool { _, hit := cache.substitutedFor(key); return hit },
	} {
		assert.False(t, probe(), "stage %s must be dropped after invalidate", name)
	}
}

// ogsgrouplive sorts the GetMyGroups result in place — a caller-visible
// mutation must never reorder or poison the memoized value.
func TestIdentityCacheGroupsAndSubsAreDefensivelyCopied(t *testing.T) {
	cache, key, ok := identityMemo(cachedCtx(42, 7))
	require.True(t, ok)

	stored := []*education.Group{{Name: "b"}, {Name: "a"}}
	cache.storeGroups(key, stored)
	// Mutating the slice we stored must not affect the cache either.
	stored[0], stored[1] = stored[1], stored[0]

	first, hit := cache.groupsFor(key)
	require.True(t, hit)
	require.Len(t, first, 2)
	assert.Equal(t, "b", first[0].Name)

	// Mutate the returned slice; a re-read must be unaffected.
	first[0], first[1] = first[1], first[0]
	second, hit := cache.groupsFor(key)
	require.True(t, hit)
	assert.Equal(t, "b", second[0].Name)

	subs := map[int64]bool{5: true}
	cache.storeSubstituted(key, subs)
	subs[6] = true // mutate the stored-from map
	got, hit := cache.substitutedFor(key)
	require.True(t, hit)
	assert.Equal(t, map[int64]bool{5: true}, got)

	got[7] = true // mutate the returned map
	again, hit := cache.substitutedFor(key)
	require.True(t, hit)
	assert.Equal(t, map[int64]bool{5: true}, again)
}
