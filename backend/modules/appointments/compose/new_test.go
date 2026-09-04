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

func TestAppointmentRecipientsAreAtomicTenantScopedAndDeduplicateReminderPushes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module := buildModule(t, db, log.record)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Rita", "Empfang")
	student := testpkg.CreateTestStudent(t, db, "Mia", "Empfang", "1a")
	guardian := testpkg.CreateTestGuardianProfile(t, db, "recipient-owner")
	created, _, err := module.CreateAppointment(ctx, appointments.CreateAppointment{
		AppointmentFields: appointmentFields(staff.ID, "Empfängertest"),
	})
	require.NoError(t, err)

	recipients, links, err := module.CreateAppointmentRecipients(ctx, created.ID, []appointments.AppointmentRecipientFields{
		{RecipientType: appointments.RecipientTypeStaff, StaffID: &staff.ID, Status: appointments.ResponseStatusPending},
		{RecipientType: appointments.RecipientTypeGuardianProfile, GuardianProfileID: &guardian.ID, Status: appointments.ResponseStatusPending, StudentIDs: []int64{student.ID, student.ID}},
	})
	require.NoError(t, err)
	require.Len(t, recipients, 2)
	require.Len(t, links, 1, "duplicate student IDs must not create duplicate links")
	assert.Equal(t, testpkg.Tenant(t), recipients[0].TenantID)
	assert.Equal(t, testpkg.Tenant(t), links[0].TenantID)
	assert.Equal(t, student.ID, links[0].StudentID)
	assertRecipientPersistenceAndResponse(t, module, ctx, created.ID, student.ID, recipients, links)
	assertReminderClaimLifecycle(t, module, ctx, created, guardian.ID, log)
}

func assertRecipientPersistenceAndResponse(t *testing.T, module *appointments.Module, ctx context.Context, appointmentID, studentID int64, recipients []*appointments.AppointmentRecipient, links []*appointments.AppointmentRecipientStudent) {
	t.Helper()
	stored, err := module.FindAppointmentRecipients(ctx, appointmentID)
	require.NoError(t, err)
	assert.ElementsMatch(t, recipients, stored)
	storedByIDs, err := module.FindAppointmentRecipientsByAppointmentIDs(ctx, []int64{appointmentID})
	require.NoError(t, err)
	assert.ElementsMatch(t, recipients, storedByIDs)
	storedLinks, err := module.FindAppointmentRecipientStudents(ctx, []int64{links[0].RecipientID})
	require.NoError(t, err)
	assert.Equal(t, links, storedLinks)
	count, err := module.CountAppointmentRecipientStudents(ctx, studentID)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	guardianRecipient := recipients[1]
	if guardianRecipient.GuardianProfileID == nil {
		guardianRecipient = recipients[0]
	}
	require.NotNil(t, guardianRecipient.GuardianProfileID)
	require.NoError(t, module.UpdateAppointmentRecipientResponse(ctx, guardianRecipient.ID, appointments.ResponseStatusAccepted))
	responded, err := module.FindAppointmentRecipient(ctx, guardianRecipient.ID)
	require.NoError(t, err)
	assert.Equal(t, appointments.ResponseStatusAccepted, responded.Status)
	assert.NotNil(t, responded.RespondedAt)
}

func assertReminderClaimLifecycle(t *testing.T, module *appointments.Module, ctx context.Context, appointment *appointments.Appointment, guardianID int64, log *observationLog) {
	t.Helper()
	claimed, err := module.ClaimReminderPushDelivery(ctx, appointment.ID, appointment.Revision, appointment.StartDate, guardianID)
	require.NoError(t, err)
	assert.True(t, claimed)
	claimed, err = module.ClaimReminderPushDelivery(ctx, appointment.ID, appointment.Revision, appointment.StartDate, guardianID)
	require.NoError(t, err)
	assert.False(t, claimed, "the same reminder delivery must be claimed once")
	require.NoError(t, module.ReleaseReminderPushDelivery(ctx, appointment.ID, appointment.Revision, appointment.StartDate, guardianID))
	claimed, err = module.ClaimReminderPushDelivery(ctx, appointment.ID, appointment.Revision, appointment.StartDate, guardianID)
	require.NoError(t, err)
	assert.True(t, claimed, "a released failed delivery must be retryable")

	log.mu.Lock()
	defer log.mu.Unlock()
	var duplicateObserved bool
	for _, observation := range log.seen {
		if observation.Operation == "claim_reminder_push_delivery" && observation.Stats.DuplicatePreventionConflicts == 1 {
			duplicateObserved = true
		}
	}
	assert.True(t, duplicateObserved, "reminder duplicate prevention must be observable")
}

func TestNamedAppointmentRecipientTablesEnforceTwoTenantRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	firstCtx := testpkg.Ctx(t)
	firstTenantID := tenant.FromContext(firstCtx)
	firstGuardianRecipientID := seedAppointmentRecipientTables(t, db, module, firstCtx, firstTenantID, "first")

	secondCtx, secondTenantID := otherTenantContext(t, db)
	seedAppointmentRecipientTables(t, db, module, secondCtx, secondTenantID, "second")

	assertAppointmentRecipientTableCounts(t, db, firstCtx, firstTenantID)
	assertAppointmentRecipientTableCounts(t, db, secondCtx, secondTenantID)
	require.ErrorIs(t, module.UpdateAppointmentRecipientResponse(secondCtx, firstGuardianRecipientID, appointments.ResponseStatusDeclined), appointments.ErrAppointmentRecipientNotFound)
	firstRecipient, err := module.FindAppointmentRecipient(firstCtx, firstGuardianRecipientID)
	require.NoError(t, err)
	assert.Equal(t, appointments.ResponseStatusPending, firstRecipient.Status)
}

func seedAppointmentRecipientTables(t *testing.T, db *bun.DB, module *appointments.Module, ctx context.Context, tenantID int64, label string) int64 {
	t.Helper()
	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "RLS", label)
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "RLS", label, "1a")
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "RLS", label, label+"@recipient-rls.example")
	created, _, err := module.CreateAppointment(ctx, appointments.CreateAppointment{AppointmentFields: appointmentFields(staff.ID, "RLS "+label)})
	require.NoError(t, err)
	recipients, links, err := module.CreateAppointmentRecipients(ctx, created.ID, []appointments.AppointmentRecipientFields{
		{RecipientType: appointments.RecipientTypeStaff, StaffID: &staff.ID, Status: appointments.ResponseStatusPending},
		{RecipientType: appointments.RecipientTypeGuardianProfile, GuardianProfileID: &guardian.ID, Status: appointments.ResponseStatusPending, StudentIDs: []int64{student.ID}},
	})
	require.NoError(t, err)
	require.Len(t, links, 1)
	for _, recipient := range recipients {
		if recipient.GuardianProfileID != nil {
			return recipient.ID
		}
	}
	t.Fatal("guardian recipient was not persisted")
	return 0
}

func assertAppointmentRecipientTableCounts(t *testing.T, db *bun.DB, ctx context.Context, tenantID int64) {
	t.Helper()
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		scoped, activeTenantID, databaseErr := database(db)(txCtx)
		require.NoError(t, databaseErr)
		assert.Equal(t, tenantID, activeTenantID)
		for table, expected := range map[string]int{
			"calendar.appointment_recipients":         2,
			"calendar.appointment_recipient_students": 1,
		} {
			count, countErr := scoped.NewSelect().TableExpr(table).Count(txCtx)
			require.NoError(t, countErr)
			assert.Equal(t, expected, count, "%s must expose only the active tenant's rows", table)
		}
		return nil
	})
	require.NoError(t, err)
}

func TestAppointmentRecipientWritesRollbackAfterEachAuthoritativeWriteAndRetry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Rita", "Rollback")
	student := testpkg.CreateTestStudent(t, db, "Mia", "Rollback", "1a")
	guardian := testpkg.CreateTestGuardianProfile(t, db, "recipient-rollback")
	created, _, err := module.CreateAppointment(ctx, appointments.CreateAppointment{
		AppointmentFields: appointmentFields(staff.ID, "Empfänger-Rollback"),
	})
	require.NoError(t, err)
	fields := []appointments.AppointmentRecipientFields{{
		RecipientType: appointments.RecipientTypeGuardianProfile, GuardianProfileID: &guardian.ID,
		Status: appointments.ResponseStatusPending, StudentIDs: []int64{student.ID},
	}}

	_, err = db.ExecContext(context.Background(), `
		CREATE FUNCTION fail_appointment_recipient_insert() RETURNS trigger AS $$
		BEGIN RAISE EXCEPTION 'injected recipient failure'; END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER fail_appointment_recipient_insert
		BEFORE INSERT ON calendar.appointment_recipients
		FOR EACH ROW EXECUTE FUNCTION fail_appointment_recipient_insert();
	`)
	require.NoError(t, err)
	_, _, err = module.CreateAppointmentRecipients(ctx, created.ID, fields)
	require.Error(t, err)
	recipients, findErr := module.FindAppointmentRecipients(ctx, created.ID)
	require.NoError(t, findErr)
	assert.Empty(t, recipients)
	_, err = db.ExecContext(context.Background(), `DROP TRIGGER fail_appointment_recipient_insert ON calendar.appointment_recipients`)
	require.NoError(t, err)

	_, err = db.ExecContext(context.Background(), `
		CREATE FUNCTION fail_appointment_recipient_student_insert() RETURNS trigger AS $$
		BEGIN RAISE EXCEPTION 'injected recipient student failure'; END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER fail_appointment_recipient_student_insert
		BEFORE INSERT ON calendar.appointment_recipient_students
		FOR EACH ROW EXECUTE FUNCTION fail_appointment_recipient_student_insert();
	`)
	require.NoError(t, err)
	_, _, err = module.CreateAppointmentRecipients(ctx, created.ID, fields)
	require.Error(t, err)
	recipients, findErr = module.FindAppointmentRecipients(ctx, created.ID)
	require.NoError(t, findErr)
	assert.Empty(t, recipients, "a failed student-link insert must roll back the recipient insert")
	_, err = db.ExecContext(context.Background(), `DROP TRIGGER fail_appointment_recipient_student_insert ON calendar.appointment_recipient_students`)
	require.NoError(t, err)

	recipients, links, err := module.CreateAppointmentRecipients(ctx, created.ID, fields)
	require.NoError(t, err)
	require.Len(t, recipients, 1, "retry must create one recipient")
	require.Len(t, links, 1, "retry must create one student link")
	stored, err := module.FindAppointmentRecipients(ctx, created.ID)
	require.NoError(t, err)
	assert.Len(t, stored, 1)
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
	_, err = module.FindAppointmentRecipients(ctx, created.ID)
	require.ErrorIs(t, err, context.Canceled)
	_, err = module.FindAppointmentRecipient(ctx, created.ID)
	require.ErrorIs(t, err, context.Canceled)
	_, err = module.FindAppointmentRecipientStudents(ctx, []int64{created.ID})
	require.ErrorIs(t, err, context.Canceled)
	_, err = module.CountAppointmentRecipientStudents(ctx, created.ID)
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

	validID := testpkg.Tenant(t)
	_, err = module.FindAppointmentRecipients(context.Background(), validID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant is required")
	_, _, err = module.CreateAppointmentRecipients(context.Background(), validID, []appointments.AppointmentRecipientFields{{
		RecipientType: appointments.RecipientTypeStaff, StaffID: &validID, Status: appointments.ResponseStatusPending,
	}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant")
	_, err = module.ClaimReminderPushDelivery(context.Background(), validID, 0, appointments.NewDate(2030, time.January, 7), validID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant")
}

func TestRecipientCommandsRejectInvalidInputs(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	validID := testpkg.Tenant(t)
	validDate := appointments.NewDate(2030, time.January, 7)

	tests := map[string]func() error{
		"missing appointment": func() error {
			_, _, err := module.CreateAppointmentRecipients(context.Background(), 0, nil)
			return err
		},
		"staff with student": func() error {
			_, _, err := module.CreateAppointmentRecipients(context.Background(), validID, []appointments.AppointmentRecipientFields{{
				RecipientType: appointments.RecipientTypeStaff, StaffID: &validID,
				Status: appointments.ResponseStatusPending, StudentIDs: []int64{validID},
			}})
			return err
		},
		"invalid student ID": func() error {
			_, _, err := module.CreateAppointmentRecipients(context.Background(), validID, []appointments.AppointmentRecipientFields{{
				RecipientType: appointments.RecipientTypeGuardianProfile, GuardianProfileID: &validID,
				Status: appointments.ResponseStatusPending, StudentIDs: []int64{-1},
			}})
			return err
		},
		"invalid response": func() error {
			return module.UpdateAppointmentRecipientResponse(context.Background(), validID, "maybe")
		},
		"invalid claim": func() error {
			_, err := module.ClaimReminderPushDelivery(context.Background(), validID, -1, validDate, validID)
			return err
		},
		"invalid release": func() error {
			return module.ReleaseReminderPushDelivery(context.Background(), validID, 0, appointments.Date(""), validID)
		},
	}
	for name, run := range tests {
		t.Run(name, func(t *testing.T) {
			require.ErrorIs(t, run(), appointments.ErrInvalidAppointment)
		})
	}
}

func TestEmptyRecurrenceAndOverrideQueriesDoNotTouchTheDatabase(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)

	rules, err := module.FindRecurrenceRules(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, rules)
	overrides, err := module.FindOccurrenceOverridesByStartDates(context.Background(), nil, nil)
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
	assert.Equal(t, "not_found", appointments.ErrorCode(appointments.ErrAppointmentRecipientNotFound))
	assert.Equal(t, "invalid", appointments.ErrorCode(appointments.ErrInvalidAppointment))
	assert.Equal(t, "lifecycle_conflict", appointments.ErrorCode(appointments.ErrAppointmentLifecycleConflict))
	assert.Equal(t, "internal_error", appointments.ErrorCode(errors.New("database unavailable")))
}
