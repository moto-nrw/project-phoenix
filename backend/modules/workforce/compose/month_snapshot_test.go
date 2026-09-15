package compose

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestMonthSnapshotCarryChainAndTenantIsolation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Month", "Snapshot")
	capability := buildWorkforce(t, db)
	input := workforce.StaffMonthBalanceSnapshot{
		StaffID: staff.ID, Year: 2026, Month: 1, ClosingBalanceMinutes: 125,
		ClosedBy: staff.ID, ClosedAt: time.Now(), Source: "admin",
	}
	january, err := capability.RecordClosedMonth(ctx, input)
	require.NoError(t, err)
	require.Equal(t, testpkg.Tenant(t), january.TenantID)
	input.Month = 2
	input.ClosingBalanceMinutes = 200
	february, err := capability.RecordClosedMonth(ctx, input)
	require.NoError(t, err)
	latest, err := capability.LatestClosedMonth(ctx, staff.ID, 2026, 1)
	require.NoError(t, err)
	require.Equal(t, january.ID, latest.ID)
	other := testpkg.NewTenantScope(t, db)
	latest, err = capability.LatestClosedMonth(other.Context(), staff.ID, 2026, 12)
	require.NoError(t, err)
	require.Nil(t, latest)
	affected, err := capability.ReopenMonthSnapshot(other.Context(), february.ID, staff.ID, time.Now(), "foreign")
	require.NoError(t, err)
	require.Zero(t, affected)
	affected, err = capability.ReopenMonthSnapshot(ctx, february.ID, staff.ID, time.Now(), "correction")
	require.NoError(t, err)
	require.EqualValues(t, 1, affected)
	latest, err = capability.LatestClosedMonth(ctx, staff.ID, 2026, 12)
	require.NoError(t, err)
	require.Equal(t, january.ID, latest.ID)
	require.Equal(t, 125, latest.ClosingBalanceMinutes)
	rows, err := capability.ClosedMonthSnapshots(ctx, 2026, 2)
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestMonthSnapshotRollbackAndMissingTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Rollback", "Snapshot")
	capability := buildWorkforce(t, db)
	input := workforce.StaffMonthBalanceSnapshot{
		StaffID: staff.ID, Year: 2026, Month: 1, ClosingBalanceMinutes: 125,
		ClosedBy: staff.ID, ClosedAt: time.Now(), Source: "admin",
	}
	failure := errors.New("later workflow step failed")
	err := testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		require.NoError(t, capability.LockStaffBalanceWrites(txCtx, staff.ID))
		_, err := capability.RecordClosedMonth(txCtx, input)
		require.NoError(t, err)
		current, err := capability.LatestClosedMonth(txCtx, staff.ID, 2026, 1)
		require.NoError(t, err)
		require.NotNil(t, current)
		return failure
	})
	require.ErrorIs(t, err, failure)
	current, err := capability.LatestClosedMonth(ctx, staff.ID, 2026, 1)
	require.NoError(t, err)
	require.Nil(t, current)
	created, err := capability.RecordClosedMonth(ctx, input)
	require.NoError(t, err)
	err = testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		affected, err := capability.ReopenMonthSnapshot(txCtx, created.ID, staff.ID, time.Now(), "rollback")
		require.NoError(t, err)
		require.EqualValues(t, 1, affected)
		return failure
	})
	require.ErrorIs(t, err, failure)
	current, err = capability.LatestClosedMonth(ctx, staff.ID, 2026, 1)
	require.NoError(t, err)
	require.NotNil(t, current)
	require.Equal(t, created.ID, current.ID)
	require.Equal(t, 125, current.ClosingBalanceMinutes)
	_, err = capability.LatestClosedMonth(context.Background(), staff.ID, 2026, 1)
	require.Error(t, err)
	_, err = capability.ClosedMonthSnapshots(context.Background(), 2026, 1)
	require.Error(t, err)
	_, err = capability.RecordClosedMonth(context.Background(), input)
	require.Error(t, err)
	_, err = capability.ReopenMonthSnapshot(context.Background(), created.ID, staff.ID, time.Now(), "unscoped")
	require.Error(t, err)
	input.Month = 13
	_, err = capability.RecordClosedMonth(ctx, input)
	require.EqualError(t, err, "month must be between 1 and 12")
}
