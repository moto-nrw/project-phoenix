package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
	appointmentsCompose "github.com/moto-nrw/project-phoenix/modules/appointments/compose"
	"github.com/moto-nrw/project-phoenix/modules/appointments/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
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
	module, err := appointmentsCompose.New(appointmentsCompose.Dependencies{DB: db, Observe: func(appointmentsCompose.Observation) {}})
	require.NoError(t, err)
	appointment, _, err := module.CreateAppointment(ctx, appointments.CreateAppointment{AppointmentFields: appointments.AppointmentFields{
		OrganizerStaffID: staff.ID,
		Title:            "Alter Termin",
		StartDate:        appointments.NewDate(2020, time.January, 6),
		EndDate:          appointments.NewDate(2020, time.January, 6),
		StartTime:        time.Date(1, time.January, 1, 9, 0, 0, 0, time.UTC),
		EndTime:          time.Date(1, time.January, 1, 10, 0, 0, 0, time.UTC),
		DeliveryMode:     appointments.DeliveryModeInformational,
	}})
	require.NoError(t, err)

	changed, err := module.CancelAppointment(ctx, appointment.ID)
	require.NoError(t, err)
	require.True(t, changed)

	store := postgres.New(func(ctx context.Context) (bun.IDB, int64, error) {
		tx, _ := tenant.TransactionFromContext(ctx)
		return tx.(bun.Tx), tenant.FromContext(ctx), nil
	})
	err = testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		deleted, err := store.DeleteAppointments(txCtx, []int64{appointment.ID})
		require.NoError(t, err)
		assert.Zero(t, deleted)
		return nil
	})
	require.NoError(t, err)

	tombstone, err := module.FindAppointment(ctx, appointment.ID)
	require.NoError(t, err)
	assert.NotNil(t, tombstone.CancelledAt)
}
