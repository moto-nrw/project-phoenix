package compose

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type observationLog struct {
	mu   sync.Mutex
	seen []Observation
}

func (l *observationLog) record(observation Observation) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.seen = append(l.seen, observation)
}

func buildModule(t *testing.T, db *bun.DB, observe ...func(Observation)) *appointments.Module {
	t.Helper()
	observer := func(Observation) {}
	if len(observe) > 0 {
		observer = observe[0]
	}
	module, err := New(Dependencies{DB: db, Observe: observer})
	require.NoError(t, err)
	return module
}

func appointmentFields(staffID int64, title string) appointments.AppointmentFields {
	return appointments.AppointmentFields{
		OrganizerStaffID: staffID,
		Title:            title,
		StartDate:        appointments.NewDate(2030, time.January, 7),
		EndDate:          appointments.NewDate(2030, time.January, 7),
		StartTime:        time.Date(1, time.January, 1, 9, 0, 0, 0, time.UTC),
		EndTime:          time.Date(1, time.January, 1, 10, 0, 0, 0, time.UTC),
		DeliveryMode:     appointments.DeliveryModeInformational,
	}
}

func otherTenantContext(t *testing.T, db *bun.DB) (context.Context, int64) {
	t.Helper()
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	return tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), tenantID), tenantID
}

func TestModuleRunsAppointmentLifecycleAndIsolatesBothTables(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module := buildModule(t, db, log.record)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Ada", "Planerin")

	created, targets, err := module.CreateAppointment(ctx, appointments.CreateAppointment{
		AppointmentFields: appointmentFields(staff.ID, "  Planung  "),
		Targets:           []appointments.AppointmentTargetFields{{TargetType: appointments.TargetTypeAllStaff}},
	})
	require.NoError(t, err)
	assert.Positive(t, created.ID)
	assert.Equal(t, testpkg.Tenant(t), created.TenantID)
	assert.Equal(t, "Planung", created.Title)
	assert.Equal(t, appointments.OverviewVisibilityOrganizer, created.OverviewVisibility)
	require.Len(t, targets, 1)
	assert.Positive(t, targets[0].ID)
	assert.Equal(t, created.ID, targets[0].AppointmentID)

	found, err := module.FindAppointment(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created, found)
	foundTargets, err := module.FindAppointmentTargets(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, targets, foundTargets)

	visible, err := module.ListAppointmentsVisibleToStaff(ctx, staff.ID, created.StartDate, created.EndDate)
	require.NoError(t, err)
	require.Len(t, visible, 1)
	assert.Equal(t, created.ID, visible[0].ID)

	otherCtx, otherTenantID := otherTenantContext(t, db)
	otherStaff := testpkg.CreateTestStaffForTenant(t, db, otherTenantID, "Fremde", "Planerin")
	_, err = module.FindAppointment(otherCtx, created.ID)
	require.ErrorIs(t, err, appointments.ErrAppointmentNotFound)
	foreignTargets, err := module.FindAppointmentTargets(otherCtx, created.ID)
	require.NoError(t, err)
	assert.Empty(t, foreignTargets)
	_, err = module.UpdateAppointment(otherCtx, appointments.UpdateAppointment{
		ID:                created.ID,
		AppointmentFields: appointmentFields(otherStaff.ID, "Fremder Titel"),
	})
	require.ErrorIs(t, err, appointments.ErrAppointmentLifecycleConflict)

	updatedFields := appointmentFields(staff.ID, "Neue Planung")
	updatedFields.NotifyGuardians = true
	updated, err := module.UpdateAppointment(ctx, appointments.UpdateAppointment{ID: created.ID, AppointmentFields: updatedFields})
	require.NoError(t, err)
	assert.Equal(t, "Neue Planung", updated.Title)
	assert.True(t, updated.NotifyGuardians)
	assert.Equal(t, created.Revision+1, updated.Revision)

	transitioned, err := module.CancelAppointment(ctx, created.ID)
	require.NoError(t, err)
	assert.True(t, transitioned)
	transitioned, err = module.CancelAppointment(ctx, created.ID)
	require.NoError(t, err)
	assert.False(t, transitioned, "cancelling twice is idempotent")

	require.NoError(t, module.SoftDeleteAppointment(ctx, created.ID))
	hidden, err := module.ListAppointmentsVisibleToStaff(ctx, staff.ID, created.StartDate, created.EndDate)
	require.NoError(t, err)
	assert.Empty(t, hidden)
	deleted, err := module.FindAppointment(ctx, created.ID)
	require.NoError(t, err)
	assert.NotNil(t, deleted.DeletedAt, "feed tombstones remain directly readable")

	require.NoError(t, module.DeleteAppointment(ctx, created.ID))
	_, err = module.FindAppointment(ctx, created.ID)
	require.ErrorIs(t, err, appointments.ErrAppointmentNotFound)

	log.mu.Lock()
	defer log.mu.Unlock()
	require.NotEmpty(t, log.seen)
	assert.Equal(t, "create_appointment", log.seen[0].Operation)
	assert.EqualValues(t, 2, log.seen[0].Stats.Queries)
	assert.EqualValues(t, 2, log.seen[0].Stats.Rows)
}

func TestCompoundWritesRollbackAndRetryWithoutDuplicates(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Rita", "Retry")
	fields := appointmentFields(staff.ID, "Rollback-Termin")

	rolledBack, rolledBackTargets, err := module.CreateAppointment(ctx, appointments.CreateAppointment{
		AppointmentFields: fields,
		Targets:           []appointments.AppointmentTargetFields{{TargetType: "not_a_target"}},
	})
	require.Error(t, err)
	assert.Nil(t, rolledBack, "a rolled-back appointment must not escape the command")
	assert.Nil(t, rolledBackTargets, "rolled-back targets must not escape the command")
	visible, listErr := module.ListAppointmentsVisibleToStaff(ctx, staff.ID, fields.StartDate, fields.EndDate)
	require.NoError(t, listErr)
	assert.Empty(t, visible, "a target failure must roll back the appointment insert")

	created, originalTargets, err := module.CreateAppointment(ctx, appointments.CreateAppointment{
		AppointmentFields: fields,
		Targets:           []appointments.AppointmentTargetFields{{TargetType: appointments.TargetTypeAllStaff}},
	})
	require.NoError(t, err)
	require.Len(t, originalTargets, 1)
	visible, err = module.ListAppointmentsVisibleToStaff(ctx, staff.ID, fields.StartDate, fields.EndDate)
	require.NoError(t, err)
	require.Len(t, visible, 1, "retry creates exactly one appointment")

	_, err = module.ReplaceAppointmentTargets(ctx, created.ID, []appointments.AppointmentTargetFields{{TargetType: "not_a_target"}})
	require.Error(t, err)
	afterFailure, findErr := module.FindAppointmentTargets(ctx, created.ID)
	require.NoError(t, findErr)
	assert.Equal(t, originalTargets, afterFailure, "a failed replacement must roll back its delete")

	retriedTargets, err := module.ReplaceAppointmentTargets(ctx, created.ID, nil)
	require.NoError(t, err)
	assert.Empty(t, retriedTargets)
	afterRetry, err := module.FindAppointmentTargets(ctx, created.ID)
	require.NoError(t, err)
	assert.Empty(t, afterRetry)
}

func TestReadFailuresAreNotTurnedIntoNotFound(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	staff := testpkg.CreateTestStaff(t, db, "Kontext", "Abbruch")
	created, _, err := module.CreateAppointment(testpkg.Ctx(t), appointments.CreateAppointment{
		AppointmentFields: appointmentFields(staff.ID, "Lesefehler"),
	})
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(testpkg.Ctx(t))
	cancel()

	_, err = module.FindAppointment(ctx, created.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.NotErrorIs(t, err, appointments.ErrAppointmentNotFound)
}

func TestQueriesRejectMissingTenantContext(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)

	_, err := module.FindAppointment(context.Background(), 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant is required")
	assert.NotErrorIs(t, err, appointments.ErrAppointmentNotFound)
}

func TestNewRejectsMissingDependencies(t *testing.T) {
	t.Parallel()
	_, err := New(Dependencies{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all dependencies are required")
}

func TestAppointmentErrorsHaveStableCodes(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "not_found", appointments.ErrorCode(appointments.ErrAppointmentNotFound))
	assert.Equal(t, "invalid", appointments.ErrorCode(appointments.ErrInvalidAppointment))
	assert.Equal(t, "lifecycle_conflict", appointments.ErrorCode(appointments.ErrAppointmentLifecycleConflict))
	assert.Equal(t, "internal_error", appointments.ErrorCode(errors.New("database unavailable")))
}
