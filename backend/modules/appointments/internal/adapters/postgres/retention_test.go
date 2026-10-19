package postgres

import (
	"context"
	"testing"

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
