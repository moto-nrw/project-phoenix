package compose

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestFeedHistoryTenantIsolationAndRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff, _ := testpkg.CreateTestCalendarStaff(t, db, "Feed", "History")
	other, _ := otherTenantContext(t, db)
	cutoff := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	for i, cancelledAt := range []time.Time{cutoff.Add(-time.Second), cutoff} {
		_, err := db.NewRaw(`INSERT INTO calendar.staff_feed_tombstones
			(tenant_id, staff_id, source, source_id, title, event_date, start_time, end_time, cancelled_at)
			VALUES (?, ?, 'shift', ?, 'Retained shift', '2026-07-01', '09:15:00', '10:45:00', ?)`,
			testpkg.Tenant(t), staff.ID, staff.ID+int64(i), cancelledAt).Exec(ctx)
		require.NoError(t, err)
	}
	history := NewFeedHistory(db)
	rows, err := history.ListForStaffSince(ctx, staff.ID, cutoff.Add(-time.Second))
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, cutoff.Add(-time.Second), rows[0].CancelledAt.UTC())
	assert.Equal(t, "2026-07-01", rows[0].EventDate)
	assert.Equal(t, time.Date(1, time.January, 1, 9, 15, 0, 0, time.UTC), rows[0].StartTime)

	require.NoError(t, tenant.WithTenantTx(other, db, tenant.FromContext(other), func(txCtx context.Context, _ bun.Tx) error {
		invisible, err := history.ListForStaffSince(txCtx, staff.ID, cutoff.Add(-time.Second))
		require.NoError(t, err)
		assert.Empty(t, invisible)
		count, err := history.DeleteBefore(txCtx, cutoff)
		require.NoError(t, err)
		assert.Zero(t, count)
		return nil
	}))
	rollback := errors.New("rollback retention")
	err = tenant.WithTenantTx(ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		count, err := history.DeleteBefore(txCtx, cutoff)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	rows, err = history.ListForStaffSince(ctx, staff.ID, cutoff.Add(-time.Second))
	require.NoError(t, err)
	require.Len(t, rows, 2)
	count, err := history.DeleteBefore(ctx, cutoff)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	rows, err = history.ListForStaffSince(ctx, staff.ID, cutoff)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, cutoff, rows[0].CancelledAt.UTC())

	_, err = history.DeleteBefore(context.Background(), cutoff)
	require.ErrorContains(t, err, "tenant is required")
	_, err = history.ListForStaffSince(context.Background(), staff.ID, cutoff)
	require.ErrorContains(t, err, "tenant is required")
}
