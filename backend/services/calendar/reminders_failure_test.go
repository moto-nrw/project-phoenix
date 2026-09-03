package calendar_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

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
}

func (failingRecurrenceReload) FindRecurrenceRule(context.Context, int64) (*appointments.RecurrenceRule, error) {
	return nil, errReminderStore
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
	calModels.AppointmentRecipientRepository
}

func (failingRecipientLookup) FindByAppointmentID(context.Context, int64) ([]*calModels.AppointmentRecipient, error) {
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
			cfg.Appointments = failingRecurrenceReload{cfg.Appointments}
		},
		"re-reading the occurrence override": func(cfg *calendarSvc.Config) {
			cfg.Appointments = failingOccurrenceOverrides{cfg.Appointments}
		},
		"resolving the recipients": func(cfg *calendarSvc.Config) {
			cfg.RecipientRepo = failingRecipientLookup{cfg.RecipientRepo}
		},
	}

	for name, breakStep := range cases {
		t.Run(name+" fails the tick", func(t *testing.T) {
			cfg := calendarTestConfig(db)
			cfg.Outbox = &recordingOutbox{}
			cfg.ParentsURL = "https://parents.test"
			cfg.Logger = slog.Default()
			breakStep(&cfg)

			queued, err := calendarSvc.NewService(cfg).EnqueueDueAppointmentReminders(ctx, from, to)
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

	queued, err := service.EnqueueDueAppointmentReminders(context.Background(), now, now.Add(time.Hour))
	require.Error(t, err)
	assert.ErrorContains(t, err, "tenant id is required")
	assert.Zero(t, queued)
}
