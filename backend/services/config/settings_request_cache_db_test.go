package config

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	configRepository "github.com/moto-nrw/project-phoenix/database/repositories/config"
	"github.com/moto-nrw/project-phoenix/models/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRequestCacheDBTest(t *testing.T) (*testpkg.DB, int64, SettingsService) {
	t.Helper()
	config.ResetRegistry()
	t.Cleanup(config.ResetRegistry)

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = db.ExecContext(ctx, `DELETE FROM config.setting_audit WHERE tenant_id = ?`, tenantID)
		_, _ = db.ExecContext(ctx, `DELETE FROM config.setting_values WHERE tenant_id = ?`, tenantID)
	})

	repository := configRepository.NewSettingValueRepository(testpkg.ConfigRuntime(db))
	auditRepository := configRepository.NewSettingAuditRepository(testpkg.ConfigRuntime(db))
	service := NewSettingsService(repository, auditRepository, nil, testpkg.SettingsRuntime(t, db), slog.Default())
	testpkg.SetTenantRuntime(t, service, db)
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

	requestCtx := WithSettingsRequestCache(
		testpkg.ContextForTenant(context.Background(), tenantID),
	)

	// The request memoizes the short cutoff default (15:00) before any writer.
	short, err := service.ResolveString(requestCtx, config.KeySlotListShortDayCutoff)
	require.NoError(t, err)
	require.Equal(t, "15:00", short)

	// A concurrent request commits short=15:30.
	repository := configRepository.NewSettingValueRepository(testpkg.ConfigRuntime(db))
	override := &config.SettingValue{
		SettingKey: config.KeySlotListShortDayCutoff,
		Value:      json.RawMessage(`"15:30"`),
	}
	override.SetTenantID(tenantID)
	require.NoError(t, repository.Upsert(
		testpkg.ContextForTenant(context.Background(), tenantID), override,
	))

	// The first request now writes long=15:15. Against its stale cache entry
	// (short=15:00) the pair looks valid; against the committed 15:30 it is
	// inverted and must be rejected.
	err = testpkg.WithinTenantContext(t, requestCtx, db, tenantID, func(txCtx context.Context) error {
		return service.SetValue(txCtx, config.KeySlotListLongDayCutoff, "15:15", nil, nil)
	})
	require.Error(t, err, "the inverted pair must be rejected despite the request cache")
	var invalid *InvalidValueError
	require.True(t, errors.As(err, &invalid), "expected InvalidValueError, got %v", err)
}

// TestResolveStringForTenantInTxBypassesRequestCache is the #2207 regression:
// the school mint guard re-reads security.mfa_mode inside its token
// transaction to catch a school switching MFA on mid-login. Every other
// resolve path is memoized per request — and the same request already read
// that key at the login gate — so a cached re-read would hand the guard back
// exactly the stale verdict it exists to replace.
func TestResolveStringForTenantInTxBypassesRequestCache(t *testing.T) {
	db, tenantID, service := setupRequestCacheDBTest(t)
	registerTestSetting(config.KeyMFAMode, config.FieldText, config.MFAModeOff)

	// The login gate reads the mode and memoizes "off" for this request.
	requestCtx := WithSettingsRequestCache(context.Background())
	mode, err := service.ResolveStringForTenant(requestCtx, tenantID, config.KeyMFAMode)
	require.NoError(t, err)
	require.Equal(t, config.MFAModeOff, mode)

	// An admin switches the school to required_all while the login is in flight.
	repository := configRepository.NewSettingValueRepository(testpkg.ConfigRuntime(db))
	override := &config.SettingValue{
		SettingKey: config.KeyMFAMode,
		Value:      json.RawMessage(`"` + config.MFAModeRequiredAll + `"`),
	}
	override.SetTenantID(tenantID)
	require.NoError(t, repository.Upsert(
		testpkg.ContextForTenant(context.Background(), tenantID), override,
	))

	// The cached path still answers with the pre-flip value...
	cached, err := service.ResolveStringForTenant(requestCtx, tenantID, config.KeyMFAMode)
	require.NoError(t, err)
	require.Equal(t, config.MFAModeOff, cached,
		"precondition: the request cache still serves the stale value")

	// ...while the in-transaction read, on the mint's own transaction, sees the
	// committed state and refuses to mint an MFA-free session.
	err = testpkg.WithinAdminContext(t, requestCtx, db, func(txCtx context.Context) error {
		fresh, resolveErr := service.ResolveStringForTenantInTx(txCtx, tenantID, config.KeyMFAMode)
		require.NoError(t, resolveErr)
		assert.Equal(t, config.MFAModeRequiredAll, fresh,
			"the in-transaction re-read must observe the concurrent commit")
		return nil
	})
	require.NoError(t, err)
}

// TestResolveStringForTenantInTxRequiresTransaction pins the guard rail: read
// outside a transaction, the query runs under the request's own role, where
// RLS on config.setting_values hides every row and the caller would silently
// be told the school has no override at all. That must be an error, not a
// wrong answer.
func TestResolveStringForTenantInTxRequiresTransaction(t *testing.T) {
	_, tenantID, service := setupRequestCacheDBTest(t)
	registerTestSetting(config.KeyMFAMode, config.FieldText, config.MFAModeOff)

	_, err := service.ResolveStringForTenantInTx(context.Background(), tenantID, config.KeyMFAMode)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ambient transaction")
}

// TestLockClassCollectionPairFlushesGradeLevelMax pins the whole-bucket flush:
// the enrollment phase guard reads enrollment.grade_level_max under
// LockClassCollectionPair, a key outside the class-collection toggle pair. A
// per-key eviction list would miss it; the tenant-wide flush must force the
// post-lock read back to the database.
func TestLockClassCollectionPairFlushesGradeLevelMax(t *testing.T) {
	db, tenantID, service := setupRequestCacheDBTest(t)
	registerTestSetting(config.KeyEnrollmentGradeLevelMax, config.FieldNumber, 4)

	selectCount := testpkg.CaptureSettingValueSelects(db)

	requestCtx := WithSettingsRequestCache(
		testpkg.ContextForTenant(context.Background(), tenantID),
	)

	err := testpkg.WithinTenantContext(t, requestCtx, db, tenantID, func(txCtx context.Context) error {
		value, resolveErr := service.ResolveInt(txCtx, config.KeyEnrollmentGradeLevelMax)
		require.NoError(t, resolveErr)
		require.Equal(t, 4, value)
		require.Equal(t, int32(1), selectCount())

		// Cached: no additional query.
		_, resolveErr = service.ResolveInt(txCtx, config.KeyEnrollmentGradeLevelMax)
		require.NoError(t, resolveErr)
		require.Equal(t, int32(1), selectCount())

		require.NoError(t, service.LockClassCollectionPair(txCtx))

		// Post-lock read must hit the database again.
		_, resolveErr = service.ResolveInt(txCtx, config.KeyEnrollmentGradeLevelMax)
		require.NoError(t, resolveErr)
		assert.Equal(t, int32(2), selectCount(),
			"LockClassCollectionPair must flush grade_level_max from the request cache")
		return nil
	})
	require.NoError(t, err)
}

// TestMFAModeWriteAndMintLockAreMutuallyExclusive is the #2207 review
// regression. Re-reading security.mfa_mode inside the mint transaction is only
// a read: an admin enabling MFA can commit right after it, and the login —
// which has not written its refresh token yet — still mints a session that
// never saw a second factor. The write and the mint therefore share a lock:
// while the write is uncommitted the mint cannot take its side of it, and once
// the write commits the mint proceeds and re-reads the new mode.
func TestMFAModeWriteAndMintLockAreMutuallyExclusive(t *testing.T) {
	db, tenantID, service := setupRequestCacheDBTest(t)
	registerTestSetting(config.KeyMFAMode, config.FieldText, config.MFAModeOff)

	mintLock := make(chan error, 1)
	writerCtx := testpkg.ContextForTenant(context.Background(), tenantID)

	err := testpkg.WithinTenantContext(t, writerCtx, db, tenantID, func(txCtx context.Context) error {
		require.NoError(t, service.SetValue(txCtx, config.KeyMFAMode, config.MFAModeRequiredAll, nil, nil))

		// A school login reaching its mint takes the shared side of the same
		// lock on its own transaction.
		go func() {
			mintLock <- testpkg.WithinAdminContext(t, context.Background(), db, func(mintCtx context.Context) error {
				return service.LockMFAPolicySharedForTenant(mintCtx, tenantID)
			})
		}()

		select {
		case lockErr := <-mintLock:
			t.Fatalf("a mint took the policy lock while the mfa_mode write was still uncommitted: %v", lockErr)
		case <-time.After(300 * time.Millisecond):
			// Blocked, which is the point.
		}
		return nil
	})
	require.NoError(t, err)

	select {
	case lockErr := <-mintLock:
		require.NoError(t, lockErr, "the mint must proceed once the write committed")
	case <-time.After(10 * time.Second):
		t.Fatal("the mint never acquired the policy lock after the write committed")
	}
}

// TestNonMFASettingWriteDoesNotBlockTheMintLock pins the other half: only the
// key a mint actually reads is serialized against it. Taking the policy lock
// for every setting write would stall every login in the school behind an
// unrelated admin edit.
func TestNonMFASettingWriteDoesNotBlockTheMintLock(t *testing.T) {
	db, tenantID, service := setupRequestCacheDBTest(t)
	registerTestSetting("test.unrelated", config.FieldNumber, 30)

	mintLock := make(chan error, 1)
	writerCtx := testpkg.ContextForTenant(context.Background(), tenantID)

	err := testpkg.WithinTenantContext(t, writerCtx, db, tenantID, func(txCtx context.Context) error {
		require.NoError(t, service.SetValue(txCtx, "test.unrelated", 45, nil, nil))

		go func() {
			mintLock <- testpkg.WithinAdminContext(t, context.Background(), db, func(mintCtx context.Context) error {
				return service.LockMFAPolicySharedForTenant(mintCtx, tenantID)
			})
		}()

		select {
		case lockErr := <-mintLock:
			require.NoError(t, lockErr)
		case <-time.After(5 * time.Second):
			t.Fatal("an unrelated setting write must not block a mint")
		}
		return nil
	})
	require.NoError(t, err)
}
