package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	testpkg.Run(m)
}

func TestDeleteAppointmentsKeepsFreshTombstone(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Termin", "Tombstone")
	var appointmentID int64
	err := db.NewRaw(`INSERT INTO calendar.appointments
		(tenant_id, organizer_staff_id, title, start_date, end_date, start_time, end_time, delivery_mode, overview_visibility)
		VALUES (?, ?, 'Alter Termin', '2020-01-06', '2020-01-06', '09:00', '10:00', 'informational', 'organizer')
		RETURNING id`, testpkg.Tenant(t), staff.ID).Scan(ctx, &appointmentID)
	require.NoError(t, err)
	_, err = db.NewRaw(`UPDATE calendar.appointments SET cancelled_at = CURRENT_TIMESTAMP WHERE id = ?`, appointmentID).Exec(ctx)
	require.NoError(t, err)

	var transaction bun.IDB
	store := New(func(context.Context) (bun.IDB, int64, error) {
		return transaction, testpkg.Tenant(t), nil
	})
	err = testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
		transaction = tx
		deleted, err := store.DeleteAppointments(txCtx, []int64{appointmentID})
		require.NoError(t, err)
		assert.Zero(t, deleted)
		return nil
	})
	require.NoError(t, err)

	var tombstones int
	err = db.NewRaw(`SELECT COUNT(*) FROM calendar.appointments WHERE id = ? AND cancelled_at IS NOT NULL`, appointmentID).Scan(ctx, &tombstones)
	require.NoError(t, err)
	assert.Equal(t, 1, tombstones)
}

func TestLockRetentionCandidatesBlocksAppointmentExtension(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Termin", "Sperre")
	var appointmentID int64
	err := db.NewRaw(`INSERT INTO calendar.appointments
		(tenant_id, organizer_staff_id, title, start_date, end_date, start_time, end_time, delivery_mode, overview_visibility)
		VALUES (?, ?, 'Alter Termin', '2020-01-06', '2020-01-06', '09:00', '10:00', 'informational', 'organizer')
		RETURNING id`, testpkg.Tenant(t), staff.ID).Scan(ctx, &appointmentID)
	require.NoError(t, err)

	var transaction bun.IDB
	store := New(func(context.Context) (bun.IDB, int64, error) {
		return transaction, testpkg.Tenant(t), nil
	})
	locked := make(chan struct{})
	release := make(chan struct{})
	retentionDone := make(chan error, 1)
	go func() {
		retentionDone <- testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
			transaction = tx
			candidates, err := store.LockRetentionCandidates(txCtx, appointments.NewDate(2020, time.January, 7), []int64{appointmentID})
			if err != nil {
				return err
			}
			if len(candidates) != 1 {
				return fmt.Errorf("locked candidates = %d, want 1", len(candidates))
			}
			close(locked)
			<-release
			return nil
		})
	}()

	select {
	case <-locked:
	case err := <-retentionDone:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		require.FailNow(t, "timed out locking retention candidate")
	}

	extensionDone := make(chan error, 1)
	go func() {
		extensionDone <- testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
			_, err := tx.NewRaw(`UPDATE calendar.appointments SET end_date = '2030-01-01' WHERE id = ?`, appointmentID).Exec(txCtx)
			return err
		})
	}()
	select {
	case err := <-extensionDone:
		close(release)
		require.NoError(t, <-retentionDone)
		require.FailNow(t, "appointment extension bypassed retention lock", "returned early with %v", err)
	case <-time.After(200 * time.Millisecond):
		// Expected: the extension waits for the retention transaction.
	}

	close(release)
	require.NoError(t, <-retentionDone)
	select {
	case err := <-extensionDone:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		require.FailNow(t, "timed out extending appointment after retention lock")
	}
}
