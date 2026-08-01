package config_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	configRepository "github.com/moto-nrw/project-phoenix/database/repositories/config"
	"github.com/moto-nrw/project-phoenix/models/config"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupRequestCacheDBTest(t *testing.T) (*bun.DB, int64, configService.SettingsService) {
	t.Helper()
	config.ResetRegistry()
	t.Cleanup(config.ResetRegistry)

	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = db.ExecContext(ctx, `DELETE FROM config.setting_audit WHERE tenant_id = ?`, tenantID)
		_, _ = db.ExecContext(ctx, `DELETE FROM config.setting_values WHERE tenant_id = ?`, tenantID)
		testpkg.CleanupTenantTestData(t, db, tenantID)
	})

	repository := configRepository.NewSettingValueRepository(db)
	auditRepository := configRepository.NewSettingAuditRepository(db)
	service := configService.NewSettingsService(repository, auditRepository, nil, db, slog.Default())
	return db, tenantID, service
}

// TestSlotListCutoffWriteSeesConcurrentCommitDespiteRequestCache is the #1565
// regression under the request cache: the cross-field validator re-reads the
// sibling cutoff AFTER taking the advisory lock, and the lock helpers flush
// the tenant's cache bucket. A request that memoized the old sibling value
// before the concurrent writer committed must still reject the now-inverted
// pair — a stale cache serving the pre-lock value would accept it.
func TestSlotListCutoffWriteSeesConcurrentCommitDespiteRequestCache(t *testing.T) {
	db, tenantID, service := setupRequestCacheDBTest(t)
	registerTestSetting(config.KeySlotListShortDayCutoff, config.FieldTime, "15:00")
	registerTestSetting(config.KeySlotListLongDayCutoff, config.FieldTime, "16:00")

	requestCtx := configService.WithSettingsRequestCache(
		tenant.WithTenantID(context.Background(), tenantID),
	)

	// The request memoizes the short cutoff default (15:00) before any writer.
	short, err := service.ResolveString(requestCtx, config.KeySlotListShortDayCutoff)
	require.NoError(t, err)
	require.Equal(t, "15:00", short)

	// A concurrent request commits short=15:30.
	repository := configRepository.NewSettingValueRepository(db)
	override := &config.SettingValue{
		SettingKey: config.KeySlotListShortDayCutoff,
		Value:      json.RawMessage(`"15:30"`),
	}
	override.SetTenantID(tenantID)
	require.NoError(t, repository.Upsert(
		tenant.WithTenantID(context.Background(), tenantID), override,
	))

	// The first request now writes long=15:15. Against its stale cache entry
	// (short=15:00) the pair looks valid; against the committed 15:30 it is
	// inverted and must be rejected.
	err = tenant.WithTenantTx(requestCtx, db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		return service.SetValue(txCtx, config.KeySlotListLongDayCutoff, "15:15", nil, nil)
	})
	require.Error(t, err, "the inverted pair must be rejected despite the request cache")
	var invalid *configService.InvalidValueError
	require.True(t, errors.As(err, &invalid), "expected InvalidValueError, got %v", err)
}

// TestLockClassCollectionPairFlushesGradeLevelMax pins the whole-bucket flush:
// the enrollment phase guard reads enrollment.grade_level_max under
// LockClassCollectionPair, a key outside the class-collection toggle pair. A
// per-key eviction list would miss it; the tenant-wide flush must force the
// post-lock read back to the database.
func TestLockClassCollectionPairFlushesGradeLevelMax(t *testing.T) {
	db, tenantID, service := setupRequestCacheDBTest(t)
	registerTestSetting(config.KeyEnrollmentGradeLevelMax, config.FieldNumber, 4)

	counter := &settingValuesSelectCounter{}
	db.AddQueryHook(counter)

	requestCtx := configService.WithSettingsRequestCache(
		tenant.WithTenantID(context.Background(), tenantID),
	)

	err := tenant.WithTenantTx(requestCtx, db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		value, resolveErr := service.ResolveInt(txCtx, config.KeyEnrollmentGradeLevelMax)
		require.NoError(t, resolveErr)
		require.Equal(t, 4, value)
		require.Equal(t, int32(1), counter.count.Load())

		// Cached: no additional query.
		_, resolveErr = service.ResolveInt(txCtx, config.KeyEnrollmentGradeLevelMax)
		require.NoError(t, resolveErr)
		require.Equal(t, int32(1), counter.count.Load())

		require.NoError(t, service.LockClassCollectionPair(txCtx))

		// Post-lock read must hit the database again.
		_, resolveErr = service.ResolveInt(txCtx, config.KeyEnrollmentGradeLevelMax)
		require.NoError(t, resolveErr)
		assert.Equal(t, int32(2), counter.count.Load(),
			"LockClassCollectionPair must flush grade_level_max from the request cache")
		return nil
	})
	require.NoError(t, err)
}
