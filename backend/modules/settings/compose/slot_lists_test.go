package compose_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func newSlotListDBSettings(t *testing.T, db *bun.DB) config.BatchSettingsService {
	t.Helper()
	service := settingsCompose.NewSettingsService(
		settingsCompose.NewSettingValueRepository(testpkg.ConfigRuntime(db)),
		settingsCompose.NewSettingAuditRepository(testpkg.ConfigRuntime(db)),
		nil, testpkg.SettingsRuntime(t, db), slog.Default())
	return service.(config.BatchSettingsService)
}

func TestSlotListSettingsQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	service := newSlotListDBSettings(t, db)
	require.NoError(t, testpkg.TenantCommand4(t, ctx, db, service.SetValue, config.KeySlotListLongDayCutoff, "17:30", nil, nil))
	require.NoError(t, testpkg.TenantCommand4(t, ctx, db, service.SetValue, config.KeySlotListShortDayCutoff, "15:00", nil, nil))
	reader := settingsCompose.NewSlotListSettings(service)
	for _, memo := range []bool{false, true} {
		t.Run(map[bool]string{false: "no-memo", true: "request-memo"}[memo], func(t *testing.T) {
			requestCtx := ctx
			if memo {
				requestCtx = config.WithSettingsRequestCache(requestCtx)
			}
			counter := testpkg.CaptureQueriesForContext(t, db)
			requestCtx = counter.Context(requestCtx)
			err := testpkg.WithinTenantContext(t, requestCtx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
				short, long, err := reader.PickupCutoffs(txCtx)
				require.NoError(t, err)
				require.Equal(t, "15:00", short)
				require.Equal(t, "17:30", long)
				return nil
			})
			counter.Stop()
			require.NoError(t, err)
			require.Zero(t, counter.WriteRows())
			require.Len(t, counter.Selects("config.setting_values"), 1)
			querybudgets.AssertQueryBudget(t, "modules.settings.slot_lists.pickup_cutoffs", counter.Measurement(counter.Queries()))
		})
	}
}

func TestSlotListSettingsHoldsSharedLockUntilTransactionEnds(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	service := newSlotListDBSettings(t, db)
	reader := settingsCompose.NewSlotListSettings(service)
	schoolID := testpkg.Tenant(t)
	key := fmt.Sprintf("slot-list-cutoff:%d", schoolID)
	require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, schoolID, func(readCtx context.Context) error {
		short, long, err := reader.PickupCutoffs(readCtx)
		require.NoError(t, err)
		require.Equal(t, "14:30", short)
		require.Equal(t, "16:00", long)
		// A separate transaction proves both the exclusive conflict and
		// concurrent shared-reader admission without timing-based assertions.
		return testpkg.WithTenantTx(t, ctx, db, schoolID, func(otherCtx context.Context, tx bun.Tx) error {
			var exclusive, shared bool
			if err := tx.NewRaw(`SELECT pg_try_advisory_xact_lock(hashtextextended(?, 0))`, key).Scan(otherCtx, &exclusive); err != nil {
				return err
			}
			if err := tx.NewRaw(`SELECT pg_try_advisory_xact_lock_shared(hashtextextended(?, 0))`, key).Scan(otherCtx, &shared); err != nil {
				return err
			}
			require.False(t, exclusive, "a settings writer cannot pass the held reader lock")
			require.True(t, shared, "other list readers remain concurrent")
			return nil
		})
	}))
	// The normal writer proceeds after reader commit, and the next read is fresh.
	require.NoError(t, testpkg.TenantCommand4(t, ctx, db, service.SetValue, config.KeySlotListLongDayCutoff, "17:30", nil, nil))
	require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, schoolID, func(readCtx context.Context) error {
		_, long, err := reader.PickupCutoffs(readCtx)
		require.NoError(t, err)
		require.Equal(t, "17:30", long)
		return nil
	}))
}
