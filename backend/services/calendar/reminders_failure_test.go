package calendar_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	"github.com/moto-nrw/project-phoenix/modules/appointments"
	calendarSvc "github.com/moto-nrw/project-phoenix/services/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errReminderStore is the failure injected into one repository call at a time.
var errReminderStore = errors.New("reminder store unavailable")

// Each decorator embeds the real repository and fails exactly one method, so a
// scan reaches the step under test with everything before it working.

type failingCandidateScan struct {
	appointments.Capability
}

func (failingCandidateScan) ListGuardianReminderCandidates(context.Context, appointments.Date, appointments.Date) ([]*appointments.Appointment, error) {
	return nil, errReminderStore
}

type failingCandidateLock struct {
	appointments.Capability
}

func (failingCandidateLock) FindReminderCandidateForUpdate(context.Context, int64) (*appointments.Appointment, error) {
	return nil, errReminderStore
}

type failingRecurrenceList struct {
	appointments.Capability
}

func (failingRecurrenceList) FindRecurrenceRules(context.Context, []int64) ([]*appointments.RecurrenceRule, error) {
	return nil, errReminderStore
}

type failingRecurrenceReload struct {
	appointments.Capability
	calls int
}

func (f *failingRecurrenceReload) FindRecurrenceRules(ctx context.Context, ids []int64) ([]*appointments.RecurrenceRule, error) {
	f.calls++
	if f.calls > 1 {
		return nil, errReminderStore
	}
	return f.Capability.FindRecurrenceRules(ctx, ids)
}

type failingMovedOverrides struct {
	appointments.Capability
}

func (failingMovedOverrides) FindOccurrenceOverridesByStartDates(context.Context, []int64, []appointments.Date) ([]*appointments.AppointmentOccurrenceOverride, error) {
	return nil, errReminderStore
}

type failingOccurrenceOverrides struct {
	appointments.Capability
}

func (failingOccurrenceOverrides) FindOccurrenceOverrides(context.Context, []int64, []appointments.Date) ([]*appointments.AppointmentOccurrenceOverride, error) {
	return nil, errReminderStore
}

type failingRecipientLookup struct {
	appointments.Capability
}

type failingAfterReminderClaim struct {
	appointments.Capability
	fail bool
}

func (f *failingAfterReminderClaim) ClaimReminderPushDelivery(ctx context.Context, appointmentID int64, revision int, occurrence appointments.Date, guardianID int64) (bool, error) {
	claimed, err := f.Capability.ClaimReminderPushDelivery(ctx, appointmentID, revision, occurrence, guardianID)
	if err == nil && claimed && f.fail {
		return false, errReminderStore
	}
	return claimed, err
}

type reminderEffectsSource struct{ effects calendarSvc.ReminderEffects }

func (s reminderEffectsSource) ReminderEffects() calendarSvc.ReminderEffects { return s.effects }

func TestCalendarServiceIntegration_ReminderPreparationRollsBackEmailAndPushClaim(t *testing.T) {
	t.Parallel()

	db, module := testutil.SetupCalendarModule(t)
	_, organizer := testpkg.CreateTestCalendarStaff(t, db, "Reminder", "Rollback")
	parent := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := calendarContext(t, organizer.ID)
	date := timezone.NewDate(2026, 4, 2)
	detail, err := module.Calendar.CreateStaffAppointment(ctx, calendarSvc.CreateAppointmentRequest{
		Title: "Elternabend", StartDate: date, EndDate: date,
		StartTime: wallClock(18, 0), EndTime: wallClock(19, 0),
		DeliveryMode: calModels.DeliveryModeInformational, SendEmail: true,
		Targets: []calendarSvc.AppointmentTarget{{Type: calModels.TargetTypeGuardianProfile, ID: &parent.GuardianProfileID}},
	})
	require.NoError(t, err)

	effects := module.Calendar.(calendarSvc.FullService).ReminderEffects()
	store := &failingAfterReminderClaim{Capability: effects.Appointments, fail: true}
	effects.Appointments = store
	effects.ParentsURL = "https://parents.test"
	effects.FilterEmail = func(_ context.Context, ids []int64) ([]int64, error) { return ids, nil }
	effects.FilterPush = effects.FilterEmail
	pushes := 0
	effects.Push = func(context.Context, int64, []int64, []int64, string) (bool, error) {
		pushes++
		return true, nil
	}
	command := testutil.ComposeCalendarReminderCommand(db, reminderEffectsSource{effects})
	startsAt := berlinInstant(t, date, 18, 0)
	from, to := startsAt.Add(-5*time.Minute), startsAt.Add(5*time.Minute)

	assertRows := func(expected int) {
		t.Helper()
		emails, err := db.NewSelect().TableExpr("platform.email_outbox").
			Where("tenant_id = ?", testpkg.Tenant(t)).Where("kind = ?", "appointment_reminder").Count(ctx)
		require.NoError(t, err)
		claims, err := db.NewSelect().TableExpr("calendar.appointment_reminder_push_deliveries").
			Where("tenant_id = ?", testpkg.Tenant(t)).Where("appointment_id = ?", detail.Appointment.ID).Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, expected, emails, "durable reminder emails")
		assert.Equal(t, expected, claims, "durable reminder push claims")
	}

	_, err = command.EnqueueDueAppointmentReminders(ctx, from, to)
	require.ErrorIs(t, err, errReminderStore)
	assertRows(0)
	assert.Zero(t, pushes, "a rolled-back preparation must not dispatch")

	store.fail = false
	queued, err := command.EnqueueDueAppointmentReminders(ctx, from, to)
	require.NoError(t, err)
	assert.Equal(t, 1, queued)
	assertRows(1)
	assert.Equal(t, 1, pushes)

	queued, err = command.EnqueueDueAppointmentReminders(ctx, from, to)
	require.NoError(t, err)
	// The production outbox facade returns the existing row's ID on a replay;
	// the historical command counts it, but must not create another delivery.
	assert.Equal(t, 1, queued)
	assertRows(1)
	assert.Equal(t, 1, pushes, "retry must not duplicate the committed push")

	t.Run("push-only binding remains independent of email", func(t *testing.T) {
		_, err := db.NewRaw("UPDATE calendar.appointments SET revision = revision + 1 WHERE id = ? AND tenant_id = ?", detail.Appointment.ID, testpkg.Tenant(t)).Exec(ctx)
		require.NoError(t, err)
		effects.Email = nil
		effects.FilterEmail = nil
		pushOnly := testutil.ComposeCalendarReminderCommand(db, reminderEffectsSource{effects})
		queued, err := pushOnly.EnqueueDueAppointmentReminders(ctx, from, to)
		require.NoError(t, err)
		assert.Zero(t, queued, "push-only delivery does not enqueue email")
		assert.Equal(t, 2, pushes, "push preferences and post-commit revalidation do not require an email filter")
	})
}

func (failingRecipientLookup) FindAppointmentRecipients(context.Context, int64) ([]*appointments.AppointmentRecipient, error) {
	return nil, errReminderStore
}

// A reminder scan runs unattended once a minute. Every step of it must report a
// storage failure to the scheduler instead of returning "nothing was due" —
// silently queueing zero reminders is indistinguishable from a healthy tick and
// the occurrence would simply never be reminded about.
func TestCalendarServiceIntegration_ReminderScanReportsStoreFailures(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	outbox := &recordingOutbox{}
	service := setupCalendarServiceWithOutbox(t, db, outbox)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Reminder", "Failure")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	ctx := calendarContext(t, organizerAccount.ID)
	appointmentDate := timezone.NewDate(2026, 4, 2)
	_, err := service.CreateStaffAppointment(ctx, calendarSvc.CreateAppointmentRequest{
		Title:        "Elternabend",
		StartDate:    appointmentDate,
		EndDate:      appointmentDate,
		StartTime:    wallClock(18, 0),
		EndTime:      wallClock(19, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		SendEmail:    true,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)

	startsAt := berlinInstant(t, appointmentDate, 18, 0)
	from, to := startsAt.Add(-5*time.Minute), startsAt.Add(5*time.Minute)

	cases := map[string]func(cfg *calendarSvc.Config){
		"the candidate scan": func(cfg *calendarSvc.Config) {
			cfg.Appointments = failingCandidateScan{cfg.Appointments}
		},
		"loading the recurrence rules": func(cfg *calendarSvc.Config) {
			cfg.Appointments = failingRecurrenceList{cfg.Appointments}
		},
		"loading the moved occurrences": func(cfg *calendarSvc.Config) {
			cfg.Appointments = failingMovedOverrides{cfg.Appointments}
		},
		"re-locking the appointment": func(cfg *calendarSvc.Config) {
			cfg.Appointments = failingCandidateLock{cfg.Appointments}
		},
		"re-reading the recurrence rule": func(cfg *calendarSvc.Config) {
			cfg.Appointments = &failingRecurrenceReload{Capability: cfg.Appointments}
		},
		"re-reading the occurrence override": func(cfg *calendarSvc.Config) {
			cfg.Appointments = failingOccurrenceOverrides{cfg.Appointments}
		},
		"resolving the recipients": func(cfg *calendarSvc.Config) {
			cfg.Appointments = failingRecipientLookup{cfg.Appointments}
		},
	}

	for name, breakStep := range cases {
		t.Run(name+" fails the tick", func(t *testing.T) {
			cfg := calendarTestConfig(db)
			cfg.Outbox = &recordingOutbox{}
			cfg.ParentsURL = "https://parents.test"
			cfg.Logger = slog.Default()
			breakStep(&cfg)

			queued, err := testutil.ComposeCalendarReminderCommand(db, calendarSvc.NewService(cfg)).EnqueueDueAppointmentReminders(ctx, from, to)
			require.ErrorIs(t, err, errReminderStore)
			assert.Zero(t, queued)
		})
	}
}

// The scheduler drives this per tenant. Without a tenant the scan would run
// unscoped across every school, so it refuses instead.
func TestCalendarServiceIntegration_ReminderScanRequiresATenant(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarServiceWithOutbox(t, db, &recordingOutbox{})
	now := timezone.Now()

	queued, err := testutil.ComposeCalendarReminderCommand(db, service).EnqueueDueAppointmentReminders(context.Background(), now, now.Add(time.Hour))
	require.Error(t, err)
	assert.ErrorContains(t, err, "tenant id is required")
	assert.Zero(t, queued)
}
