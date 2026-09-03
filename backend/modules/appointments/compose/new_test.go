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

	endsOn := appointments.NewDate(2030, time.February, 4)
	rule := &appointments.RecurrenceRule{
		AppointmentID: created.ID, Frequency: appointments.RecurrenceFrequencyWeekly,
		IntervalCount: 1, Weekdays: []string{"Monday", "monday"}, EndsOn: &endsOn,
	}
	require.NoError(t, module.CreateRecurrenceRule(ctx, rule))
	assert.Positive(t, rule.ID)
	assert.Equal(t, testpkg.Tenant(t), rule.TenantID)
	assert.Equal(t, []string{"monday"}, rule.Weekdays)
	foundRule, err := module.FindRecurrenceRule(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, rule, foundRule)
	rules, err := module.FindRecurrenceRules(ctx, []int64{created.ID})
	require.NoError(t, err)
	assert.Equal(t, []*appointments.RecurrenceRule{rule}, rules)

	movedDate := appointments.NewDate(2030, time.January, 8)
	startDate := appointments.NewDate(2030, time.January, 9)
	startClock := time.Date(1, time.January, 1, 8, 30, 0, 0, time.FixedZone("source", 3600))
	override := &appointments.AppointmentOccurrenceOverride{
		AppointmentID: created.ID, OccurrenceDate: movedDate, StartDate: &startDate, StartTime: &startClock,
	}
	require.NoError(t, module.CreateOccurrenceOverride(ctx, override))
	assert.Positive(t, override.ID)
	assert.Equal(t, testpkg.Tenant(t), override.TenantID)
	require.NotNil(t, override.StartTime)
	assert.Equal(t, 8, override.StartTime.Hour(), "wall-clock hour must not shift through Postgres")
	foundOverrides, err := module.FindOccurrenceOverrides(ctx, []int64{created.ID}, []appointments.Date{movedDate})
	require.NoError(t, err)
	assert.Equal(t, []*appointments.AppointmentOccurrenceOverride{override}, foundOverrides)
	movedOverrides, err := module.FindOccurrenceOverridesByStartDates(ctx, []int64{created.ID}, []appointments.Date{startDate})
	require.NoError(t, err)
	assert.Equal(t, foundOverrides, movedOverrides)
	cancelledOverrides, err := module.FindCancelledOccurrenceOverrides(ctx, []int64{created.ID})
	require.NoError(t, err)
	assert.Empty(t, cancelledOverrides)

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
	foreignRule, err := module.FindRecurrenceRule(otherCtx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, foreignRule)
	require.NoError(t, module.DeleteRecurrenceRule(otherCtx, created.ID))
	survivingRule, err := module.FindRecurrenceRule(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, rule, survivingRule, "another tenant cannot delete the recurrence rule")
	foreignOverrides, err := module.FindOccurrenceOverrides(otherCtx, []int64{created.ID}, []appointments.Date{movedDate})
	require.NoError(t, err)
	assert.Empty(t, foreignOverrides)
	_, err = module.CancelAppointmentOccurrence(otherCtx, created.ID, appointments.NewDate(2030, time.January, 14))
	require.Error(t, err, "another tenant cannot write an override for this appointment")
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

	cancelDate := appointments.NewDate(2030, time.January, 14)
	transitioned, err := module.CancelAppointmentOccurrence(ctx, created.ID, cancelDate)
	require.NoError(t, err)
	assert.True(t, transitioned)
	afterOccurrenceCancel, err := module.FindAppointment(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, updated.Revision+1, afterOccurrenceCancel.Revision)
	transitioned, err = module.CancelAppointmentOccurrence(ctx, created.ID, cancelDate)
	require.NoError(t, err)
	assert.False(t, transitioned, "cancelling one occurrence twice is idempotent")
	afterIdempotentRetry, err := module.FindAppointment(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, afterOccurrenceCancel.Revision, afterIdempotentRetry.Revision)
	cancelledOverrides, err = module.FindCancelledOccurrenceOverrides(ctx, []int64{created.ID})
	require.NoError(t, err)
	require.Len(t, cancelledOverrides, 1)
	assert.Equal(t, cancelDate, cancelledOverrides[0].OccurrenceDate)

	transitioned, err = module.CancelAppointment(ctx, created.ID)
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
	var successfulOccurrenceCancellations []Observation
	for _, observation := range log.seen {
		if observation.Operation == "cancel_appointment_occurrence" && observation.Err == nil {
			successfulOccurrenceCancellations = append(successfulOccurrenceCancellations, observation)
		}
	}
	require.Len(t, successfulOccurrenceCancellations, 2)
	assert.EqualValues(t, 2, successfulOccurrenceCancellations[0].Stats.Queries)
	assert.EqualValues(t, 2, successfulOccurrenceCancellations[0].Stats.Rows)
	assert.Zero(t, successfulOccurrenceCancellations[0].Stats.DuplicatePreventionConflicts)
	assert.EqualValues(t, 1, successfulOccurrenceCancellations[1].Stats.Queries)
	assert.Zero(t, successfulOccurrenceCancellations[1].Stats.Rows, "the duplicate-prevention conflict is an observable no-op")
	assert.EqualValues(t, 1, successfulOccurrenceCancellations[1].Stats.DuplicatePreventionConflicts)
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

func TestOccurrenceCancellationRollsBackWhenRevisionBumpFails(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Rita", "Revision")
	created, _, err := module.CreateAppointment(ctx, appointments.CreateAppointment{
		AppointmentFields: appointmentFields(staff.ID, "Atomare Absage"),
	})
	require.NoError(t, err)
	date := appointments.NewDate(2030, time.January, 14)

	_, err = db.ExecContext(context.Background(), `
		CREATE FUNCTION fail_appointment_revision_bump() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'injected revision failure';
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER fail_appointment_revision_bump
		BEFORE UPDATE OF revision ON calendar.appointments
		FOR EACH ROW EXECUTE FUNCTION fail_appointment_revision_bump();
	`)
	require.NoError(t, err)

	transitioned, err := module.CancelAppointmentOccurrence(ctx, created.ID, date)
	require.Error(t, err)
	assert.False(t, transitioned, "a rolled-back transition must not escape the command")
	overrides, findErr := module.FindOccurrenceOverrides(ctx, []int64{created.ID}, []appointments.Date{date})
	require.NoError(t, findErr)
	assert.Empty(t, overrides, "the override insert must roll back with the failed revision bump")
	afterFailure, findErr := module.FindAppointment(ctx, created.ID)
	require.NoError(t, findErr)
	assert.Equal(t, created.Revision, afterFailure.Revision)

	_, err = db.ExecContext(context.Background(), `DROP TRIGGER fail_appointment_revision_bump ON calendar.appointments`)
	require.NoError(t, err)
	transitioned, err = module.CancelAppointmentOccurrence(ctx, created.ID, date)
	require.NoError(t, err)
	assert.True(t, transitioned)
	overrides, err = module.FindOccurrenceOverrides(ctx, []int64{created.ID}, []appointments.Date{date})
	require.NoError(t, err)
	require.Len(t, overrides, 1, "retry creates exactly one override")
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
	rule := &appointments.RecurrenceRule{
		AppointmentID: created.ID, Frequency: appointments.RecurrenceFrequencyDaily, IntervalCount: 1,
	}
	require.NoError(t, module.CreateRecurrenceRule(testpkg.Ctx(t), rule))
	override := &appointments.AppointmentOccurrenceOverride{
		AppointmentID: created.ID, OccurrenceDate: created.StartDate, Cancelled: true,
	}
	require.NoError(t, module.CreateOccurrenceOverride(testpkg.Ctx(t), override))
	ctx, cancel := context.WithCancel(testpkg.Ctx(t))
	cancel()

	_, err = module.FindAppointment(ctx, created.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.NotErrorIs(t, err, appointments.ErrAppointmentNotFound)
	_, err = module.FindRecurrenceRule(ctx, created.ID)
	require.ErrorIs(t, err, context.Canceled)
	_, err = module.FindOccurrenceOverrides(ctx, []int64{created.ID}, []appointments.Date{created.StartDate})
	require.ErrorIs(t, err, context.Canceled)
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

func TestEmptyRecurrenceAndOverrideQueriesDoNotTouchTheDatabase(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)

	rules, err := module.FindRecurrenceRules(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, rules)
	overrides, err := module.FindOccurrenceOverridesByStartDates(context.Background(), []int64{1}, nil)
	require.NoError(t, err)
	assert.Empty(t, overrides)
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
