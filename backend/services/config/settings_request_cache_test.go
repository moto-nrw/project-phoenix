package config

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The request cache guarantees that within one request context every
// (tenant_id, key) pair is loaded from the repository at most once (#2065).

func TestRequestCacheSecondResolveIssuesNoRepoCall(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.cached_flag", config.FieldBoolean, true)

	repo := newMockValueRepo()
	svc := createService(repo, &mockAuditRepo{})
	var cachePaths []string
	svc.(interface{ SetLookupObserver(SettingsLookupObserver) }).SetLookupObserver(
		func(_ string, cache, outcome string, _ time.Duration) {
			cachePaths = append(cachePaths, cache+":"+outcome)
		},
	)
	ctx := WithSettingsRequestCache(tenantCtx(41))

	first, err := svc.ResolveBool(ctx, "test.cached_flag")
	require.NoError(t, err)
	second, err := svc.ResolveBool(ctx, "test.cached_flag")
	require.NoError(t, err)
	assert.Equal(t, first, second)
	assert.Equal(t, 1, repo.findManyCalls, "second resolve must be served from the request cache")
	assert.Equal(t, []string{"miss:ok", "hit:ok"}, cachePaths)

	// HasTenantOverride shares the cached entry too.
	hasOverride, err := svc.HasTenantOverride(ctx, "test.cached_flag")
	require.NoError(t, err)
	assert.False(t, hasOverride)
	assert.Equal(t, 1, repo.findManyCalls)
}

func TestRequestCachePartialHitQueriesOnlyMissingKeys(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.first", config.FieldBoolean, false)
	registerTestSetting("test.second", config.FieldNumber, 5)

	repo := newMockValueRepo()
	svc := createService(repo, &mockAuditRepo{})
	batch, ok := svc.(BatchSettingsService)
	require.True(t, ok)
	ctx := WithSettingsRequestCache(tenantCtx(41))

	_, err := svc.ResolveBool(ctx, "test.first")
	require.NoError(t, err)
	require.Equal(t, 1, repo.findManyCalls)

	snapshot, err := batch.ResolveMany(ctx, []string{"test.first", "test.second"})
	require.NoError(t, err)
	assert.Equal(t, 2, repo.findManyCalls, "only the missing key may trigger a repository query")
	require.Len(t, repo.findManyKeys, 2)
	assert.Equal(t, []string{"test.second"}, repo.findManyKeys[1], "cached keys must not be re-requested")

	integer, err := snapshot.Int("test.second")
	require.NoError(t, err)
	assert.Equal(t, 5, integer)
}

func TestRequestCacheIsolatesTenants(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.isolated", config.FieldText, "default")

	repo := newMockValueRepo()
	overrideA := &config.SettingValue{SettingKey: "test.isolated", Value: json.RawMessage(`"tenant-a"`)}
	overrideA.TenantID = 41
	repo.values[repo.key(41, overrideA.SettingKey)] = overrideA

	svc := createService(repo, &mockAuditRepo{})

	// One shared cache observed under two tenant contexts: the second tenant
	// must get its own repository read and its own value.
	base := WithSettingsRequestCache(tenantCtx(41))
	valueA, err := svc.ResolveString(base, "test.isolated")
	require.NoError(t, err)
	assert.Equal(t, "tenant-a", valueA)
	require.Equal(t, 1, repo.findManyCalls)

	ctxB := withTestTenantID(base, 42)
	valueB, err := svc.ResolveString(ctxB, "test.isolated")
	require.NoError(t, err)
	assert.Equal(t, "default", valueB)
	assert.Equal(t, 2, repo.findManyCalls, "tenant B must not be served tenant A's cache entry")
}

func TestRequestCacheCachesAbsentOverride(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.absent", config.FieldNumber, 30)

	repo := newMockValueRepo()
	svc := createService(repo, &mockAuditRepo{})
	ctx := WithSettingsRequestCache(tenantCtx(41))

	value, err := svc.ResolveInt(ctx, "test.absent")
	require.NoError(t, err)
	assert.Equal(t, 30, value)

	hasOverride, err := svc.HasTenantOverride(ctx, "test.absent")
	require.NoError(t, err)
	assert.False(t, hasOverride)
	assert.Equal(t, 1, repo.findManyCalls, "the absent-override result must be cached, not re-queried")
}

func TestRequestCacheCachesUnmarshalError(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.corrupt", config.FieldBoolean, false)

	repo := newMockValueRepo()
	corrupt := &config.SettingValue{SettingKey: "test.corrupt", Value: json.RawMessage(`{invalid`)}
	corrupt.TenantID = 41
	repo.values[repo.key(41, corrupt.SettingKey)] = corrupt

	svc := createService(repo, &mockAuditRepo{})
	ctx := WithSettingsRequestCache(tenantCtx(41))

	_, firstErr := svc.ResolveBool(ctx, "test.corrupt")
	require.Error(t, firstErr)
	_, secondErr := svc.ResolveBool(ctx, "test.corrupt")
	require.Error(t, secondErr)
	assert.Equal(t, firstErr.Error(), secondErr.Error(), "the cached error must be identical")
	assert.Equal(t, 1, repo.findManyCalls)
}

func TestWithSettingsRequestCacheIsIdempotent(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.idempotent", config.FieldBoolean, true)

	base := tenantCtx(41)
	once := WithSettingsRequestCache(base)
	twice := WithSettingsRequestCache(once)
	assert.Equal(t, once, twice, "re-attaching must return the same context (shared cache)")

	// Behavioral proof: a resolve through the doubly-wrapped context is served
	// by the same cache the singly-wrapped context populated.
	repo := newMockValueRepo()
	svc := createService(repo, &mockAuditRepo{})
	_, err := svc.ResolveBool(once, "test.idempotent")
	require.NoError(t, err)
	_, err = svc.ResolveBool(twice, "test.idempotent")
	require.NoError(t, err)
	assert.Equal(t, 1, repo.findManyCalls)
}

func TestResolveManyDoesNotCacheContextSnapshotValues(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.snapshot_key", config.FieldBoolean, false)

	repo := newMockValueRepo()
	svc := createService(repo, &mockAuditRepo{})
	batch, ok := svc.(BatchSettingsService)
	require.True(t, ok)

	cached := WithSettingsRequestCache(tenantCtx(41))
	snapshot, err := batch.ResolveMany(tenantCtx(41), []string{"test.snapshot_key"})
	require.NoError(t, err)
	require.Equal(t, 1, repo.findManyCalls)

	// Resolving through an explicit snapshot must not promote its (possibly
	// stale) values into the request cache.
	snapCtx := WithSettingsSnapshot(cached, snapshot)
	_, err = svc.ResolveBool(snapCtx, "test.snapshot_key")
	require.NoError(t, err)
	assert.Equal(t, 1, repo.findManyCalls, "snapshot serves the read")

	_, err = svc.ResolveBool(cached, "test.snapshot_key")
	require.NoError(t, err)
	assert.Equal(t, 2, repo.findManyCalls, "without the snapshot the cache must still be empty")
}

func TestResolveManyWithoutCacheInContextStillWorks(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.plain", config.FieldBoolean, true)

	repo := newMockValueRepo()
	svc := createService(repo, &mockAuditRepo{})

	// Scheduler / CLI contexts carry no request cache: every resolve queries.
	_, err := svc.ResolveBool(tenantCtx(41), "test.plain")
	require.NoError(t, err)
	_, err = svc.ResolveBool(tenantCtx(41), "test.plain")
	require.NoError(t, err)
	assert.Equal(t, 2, repo.findManyCalls)
}

func TestRequestCacheUnknownKeyFailsWithoutQuery(t *testing.T) {
	setupTest(t)

	repo := newMockValueRepo()
	svc := createService(repo, &mockAuditRepo{})
	ctx := WithSettingsRequestCache(tenantCtx(41))

	_, err := svc.Resolve(ctx, "nonexistent.key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Equal(t, 0, repo.findManyCalls, "unknown keys fail before any repository access")
}

func TestSetValueEvictsRequestCacheEntry(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.rewritten", config.FieldNumber, 10)

	repo := newMockValueRepo()
	svc := createService(repo, &mockAuditRepo{})
	ctx := WithSettingsRequestCache(tenantCtx(41))

	before, err := svc.ResolveInt(ctx, "test.rewritten")
	require.NoError(t, err)
	assert.Equal(t, 10, before)

	require.NoError(t, svc.SetValue(ctx, "test.rewritten", 20, nil, nil))

	after, err := svc.ResolveInt(ctx, "test.rewritten")
	require.NoError(t, err)
	assert.Equal(t, 20, after, "a same-request read after SetValue must see the new value")
}

func TestResetValueEvictsRequestCacheEntry(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.reset", config.FieldNumber, 10)

	repo := newMockValueRepo()
	override := &config.SettingValue{SettingKey: "test.reset", Value: json.RawMessage(`99`)}
	override.TenantID = 41
	repo.values[repo.key(41, override.SettingKey)] = override

	svc := createService(repo, &mockAuditRepo{})
	ctx := WithSettingsRequestCache(tenantCtx(41))

	before, err := svc.ResolveInt(ctx, "test.reset")
	require.NoError(t, err)
	assert.Equal(t, 99, before)

	require.NoError(t, svc.ResetValue(ctx, "test.reset", nil, nil))

	after, err := svc.ResolveInt(ctx, "test.reset")
	require.NoError(t, err)
	assert.Equal(t, 10, after, "a same-request read after ResetValue must see the registry default")
}

func TestLockHelpersFlushTenantCache(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.locked_a", config.FieldBoolean, false)
	registerTestSetting("test.locked_b", config.FieldNumber, 1)

	cases := []struct {
		name string
		lock func(svc SettingsService, ctx context.Context) error
	}{
		{"slot_list_cutoff", func(svc SettingsService, ctx context.Context) error {
			return svc.LockSlotListCutoffPair(ctx)
		}},
		{"slot_list_cutoff_shared", func(svc SettingsService, ctx context.Context) error {
			return svc.LockSlotListCutoffPairShared(ctx)
		}},
		{"class_collection", func(svc SettingsService, ctx context.Context) error {
			return svc.LockClassCollectionPair(ctx)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMockValueRepo()
			svc := createService(repo, &mockAuditRepo{})
			ctx := WithSettingsRequestCache(tenantCtx(41))

			// Prime the tenant bucket with two unrelated keys.
			_, err := svc.ResolveBool(ctx, "test.locked_a")
			require.NoError(t, err)
			_, err = svc.ResolveInt(ctx, "test.locked_b")
			require.NoError(t, err)
			require.Equal(t, 2, repo.findManyCalls)

			// Taking the lock — even without an ambient transaction — must
			// flush the whole tenant bucket (#1565/#1663 freshness barrier).
			require.NoError(t, tc.lock(svc, ctx))

			_, err = svc.ResolveBool(ctx, "test.locked_a")
			require.NoError(t, err)
			_, err = svc.ResolveInt(ctx, "test.locked_b")
			require.NoError(t, err)
			assert.Equal(t, 4, repo.findManyCalls, "post-lock reads must hit the repository again")
		})
	}
}

func TestResolveBoolForTenantSkipsTxOnFullCacheHit(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.for_tenant", config.FieldBoolean, true)

	repo := newMockValueRepo()
	// createService wires a nil *bun.DB: if ResolveBoolForTenant reached
	// WithTenantTx, the nil DB would blow up. Returning successfully from the
	// second call therefore proves the transaction was skipped.
	svc := createService(repo, &mockAuditRepo{})
	ctx := WithSettingsRequestCache(tenantCtx(41))

	_, err := svc.ResolveBool(ctx, "test.for_tenant")
	require.NoError(t, err)
	require.Equal(t, 1, repo.findManyCalls)

	value, err := svc.ResolveBoolForTenant(ctx, 41, "test.for_tenant")
	require.NoError(t, err)
	assert.True(t, value)
	assert.Equal(t, 1, repo.findManyCalls, "a full cache hit must skip the tenant transaction and the query")
}
