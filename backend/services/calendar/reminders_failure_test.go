package calendar_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
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
	calModels.AppointmentRepository
}

func (failingCandidateScan) ListGuardianReminderCandidates(context.Context, timezone.Date, timezone.Date) ([]*calModels.Appointment, error) {
	return nil, errReminderStore
}

type failingCandidateLock struct {
	calModels.AppointmentRepository
}

func (failingCandidateLock) LockReminderCandidate(context.Context, int64) (*calModels.Appointment, error) {
	return nil, errReminderStore
}

type failingRecurrenceList struct {
	calModels.RecurrenceRuleRepository
}

func (failingRecurrenceList) FindByAppointmentIDs(context.Context, []int64) ([]*calModels.RecurrenceRule, error) {
	return nil, errReminderStore
}

type failingRecurrenceReload struct {
	calModels.RecurrenceRuleRepository
}

func (failingRecurrenceReload) FindByAppointmentID(context.Context, int64) (*calModels.RecurrenceRule, error) {
	return nil, errReminderStore
}

type failingMovedOverrides struct {
	calModels.AppointmentOccurrenceOverrideRepository
}

func (failingMovedOverrides) FindByAppointmentIDsAndStartDates(context.Context, []int64, []timezone.Date) ([]*calModels.AppointmentOccurrenceOverride, error) {
	return nil, errReminderStore
}

type failingOccurrenceOverrides struct {
	calModels.AppointmentOccurrenceOverrideRepository
}

func (failingOccurrenceOverrides) FindByAppointmentIDsAndOccurrenceDates(context.Context, []int64, []timezone.Date) ([]*calModels.AppointmentOccurrenceOverride, error) {
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
	db := testpkg.SetupTestDB(t)

	outbox := &recordingOutbox{}
	service := setupCalendarServiceWithOutbox(t, db, outbox)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Reminder", "Failure")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	ctx := calendarContext(t, organizerAccount.ID)
	appointmentDate := timezone.NewDate(2026, 4, 2)
	detail, err := service.CreateStaffAppointment(ctx, calendarSvc.CreateAppointmentRequest{
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
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })

	startsAt := berlinInstant(t, appointmentDate, 18, 0)
	from, to := startsAt.Add(-5*time.Minute), startsAt.Add(5*time.Minute)

	cases := map[string]func(cfg *calendarSvc.Config){
		"the candidate scan": func(cfg *calendarSvc.Config) {
			cfg.AppointmentRepo = failingCandidateScan{cfg.AppointmentRepo}
		},
		"loading the recurrence rules": func(cfg *calendarSvc.Config) {
			cfg.RecurrenceRepo = failingRecurrenceList{cfg.RecurrenceRepo}
		},
		"loading the moved occurrences": func(cfg *calendarSvc.Config) {
			cfg.OverrideRepo = failingMovedOverrides{cfg.OverrideRepo}
		},
		"re-locking the appointment": func(cfg *calendarSvc.Config) {
			cfg.AppointmentRepo = failingCandidateLock{cfg.AppointmentRepo}
		},
		"re-reading the recurrence rule": func(cfg *calendarSvc.Config) {
			cfg.RecurrenceRepo = failingRecurrenceReload{cfg.RecurrenceRepo}
		},
		"re-reading the occurrence override": func(cfg *calendarSvc.Config) {
			cfg.OverrideRepo = failingOccurrenceOverrides{cfg.OverrideRepo}
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
	db := testpkg.SetupTestDB(t)

	service := setupCalendarServiceWithOutbox(t, db, &recordingOutbox{})
	now := timezone.Now()

	queued, err := service.EnqueueDueAppointmentReminders(context.Background(), now, now.Add(time.Hour))
	require.Error(t, err)
	assert.ErrorContains(t, err, "tenant id is required")
	assert.Zero(t, queued)
}
