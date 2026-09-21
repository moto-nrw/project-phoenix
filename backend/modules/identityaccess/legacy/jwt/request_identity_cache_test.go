package jwt

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// WithRequestIdentityCache returns its input unchanged when a cache is
// already attached, so stacked middleware shares one cache (#2099).
func TestWithRequestIdentityCacheIsIdempotent(t *testing.T) {
	t.Parallel()

	assert.Nil(t, RequestIdentityCacheFrom(context.Background()))

	once := WithRequestIdentityCache(context.Background())
	require.NotNil(t, RequestIdentityCacheFrom(once))
	assert.Equal(t, once, WithRequestIdentityCache(once))
	assert.Same(t, RequestIdentityCacheFrom(once), RequestIdentityCacheFrom(WithRequestIdentityCache(once)))
}

type cacheTestEntry struct{ name string }

func TestRequestIdentityCacheEntryCreatesOnce(t *testing.T) {
	t.Parallel()

	cache := RequestIdentityCacheFrom(WithRequestIdentityCache(context.Background()))
	creates := 0
	create := func() any {
		creates++
		return &cacheTestEntry{name: "memo"}
	}

	first := cache.Entry(7, 42, create)
	second := cache.Entry(7, 42, create)

	assert.Equal(t, 1, creates)
	assert.Same(t, first, second)
}

func TestRequestIdentityCacheEvictDropsTheEntry(t *testing.T) {
	t.Parallel()

	cache := RequestIdentityCacheFrom(WithRequestIdentityCache(context.Background()))
	first := cache.Entry(7, 42, func() any { return &cacheTestEntry{name: "before"} })
	other := cache.Entry(7, 43, func() any { return &cacheTestEntry{name: "other account"} })

	cache.Evict(7, 42)

	fresh := cache.Entry(7, 42, func() any { return &cacheTestEntry{name: "after"} })
	assert.NotSame(t, first, fresh)
	assert.Equal(t, "after", fresh.(*cacheTestEntry).name)
	assert.Same(t, other, cache.Entry(7, 43, func() any { return &cacheTestEntry{} }),
		"Evict drops only the caller's entry")
}

func TestRequestIdentityCacheIsolatesTenantsAndAccounts(t *testing.T) {
	t.Parallel()

	cache := RequestIdentityCacheFrom(WithRequestIdentityCache(context.Background()))
	tenant7 := cache.Entry(7, 42, func() any { return &cacheTestEntry{name: "tenant 7"} })

	otherTenant := cache.Entry(8, 42, func() any { return &cacheTestEntry{name: "tenant 8"} })
	assert.NotSame(t, tenant7, otherTenant, "another tenant's entry must not be served")
	assert.Equal(t, "tenant 8", otherTenant.(*cacheTestEntry).name)

	otherAccount := cache.Entry(7, 43, func() any { return &cacheTestEntry{name: "account 43"} })
	assert.NotSame(t, tenant7, otherAccount, "another account's entry must not be served")
	assert.Equal(t, "account 43", otherAccount.(*cacheTestEntry).name)
}
