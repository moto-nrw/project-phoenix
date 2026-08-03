package calendar_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	calendarSvc "github.com/moto-nrw/project-phoenix/services/calendar"
	"github.com/moto-nrw/project-phoenix/services/notifications"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type reminderCaptureNotifier struct {
	events []notifications.Event
	err    error
}

func (n *reminderCaptureNotifier) Notify(_ context.Context, event notifications.Event) error {
	if n.err != nil {
		return n.err
	}
	n.events = append(n.events, event)
	return nil
}

func (n *reminderCaptureNotifier) NotifySynchronously(ctx context.Context, event notifications.Event) error {
	return n.Notify(ctx, event)
}

type reminderPreferences struct {
	notifications.PreferenceService
}

func (reminderPreferences) FilterNotOptedOut(_ context.Context, _ string, accountIDs []int64) ([]int64, error) {
	return accountIDs, nil
}

func (reminderPreferences) FilterOptedIn(_ context.Context, _ string, accountIDs []int64) ([]int64, error) {
	return accountIDs, nil
}

// optedOutAppointmentPreferences opts one account out of exactly one
// notification type. The lifecycle mail and the reminder are separate types
// behind the same explicit-opt-out gate, so a type-blind double would suppress
// both and no test could tell which gate it proved.
type optedOutAppointmentPreferences struct {
	notifications.PreferenceService
	optedOutType string
}

func (p optedOutAppointmentPreferences) FilterNotOptedOut(_ context.Context, notificationType string, accountIDs []int64) ([]int64, error) {
	if notificationType == p.optedOutType {
		return nil, nil
	}
	return accountIDs, nil
}

func (optedOutAppointmentPreferences) FilterOptedIn(_ context.Context, _ string, accountIDs []int64) ([]int64, error) {
	return accountIDs, nil
}

// berlinInstant builds the moment a Berlin wall-clock time happens on a date,
// which is what the reminder window is expressed in.
func berlinInstant(t *testing.T, date timezone.Date, hour, minute int) time.Time {
	t.Helper()
	midnight := date.BerlinMidnight()
	return time.Date(midnight.Year(), midnight.Month(), midnight.Day(), hour, minute, 0, 0, midnight.Location())
}

// The reminder scan walks the real query, the real occurrence expansion and the
// real recipient resolution. Only the outbox is a recorder.
func TestCalendarServiceIntegration_AppointmentReminders(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	outbox := &recordingOutbox{}
	service := setupCalendarServiceWithOutbox(t, db, outbox)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Reminder", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	ctx := calendarContext(organizerAccount.ID)
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
	require.Len(t, outbox.enqueued, 1, "the published notice")

	startsAt := berlinInstant(t, appointmentDate, 18, 0)

	t.Run("a window that does not contain the start queues nothing", func(t *testing.T) {
		before := len(outbox.enqueued)
		queued, err := service.EnqueueDueAppointmentReminders(ctx, startsAt.Add(-2*time.Hour), startsAt.Add(-time.Hour))
		require.NoError(t, err)
		assert.Zero(t, queued)
		assert.Len(t, outbox.enqueued, before)
	})

	t.Run("a window containing the start queues one reminder per guardian", func(t *testing.T) {
		queued, err := service.EnqueueDueAppointmentReminders(ctx, startsAt.Add(-5*time.Minute), startsAt.Add(5*time.Minute))
		require.NoError(t, err)
		require.Equal(t, 1, queued)

		last := outbox.enqueued[len(outbox.enqueued)-1]
		assert.Equal(t, platformModels.EmailKindAppointmentReminder, last.Kind)
		assert.Equal(t, platformModels.EmailRelatedTypeAppointment, last.RelatedEntityType)
		assert.Equal(t, detail.Appointment.ID, last.RelatedEntityID)
		assert.NotEmpty(t, last.IdempotencyKey,
			"without a key a second tick would mail the same occurrence again")
		assert.Contains(t, last.Payload, "recipient_email")
		assert.Equal(t, "Elternabend", last.Payload["title"])
		assert.Contains(t, last.Payload["when_text"], "02.04.2026")
	})

	t.Run("the window is half-open at the upper bound", func(t *testing.T) {
		// An occurrence exactly at `to` belongs to the NEXT window. Without this
		// the adjacent windows the scheduler produces would both claim it.
		before := len(outbox.enqueued)
		queued, err := service.EnqueueDueAppointmentReminders(ctx, startsAt.Add(-time.Hour), startsAt)
		require.NoError(t, err)
		assert.Zero(t, queued)
		assert.Len(t, outbox.enqueued, before)
	})

	t.Run("an inverted window is refused rather than scanned", func(t *testing.T) {
		queued, err := service.EnqueueDueAppointmentReminders(ctx, startsAt, startsAt.Add(-time.Hour))
		require.NoError(t, err)
		assert.Zero(t, queued)
	})
}

func TestCalendarServiceIntegration_AppointmentReminderEmailHonorsOptOut(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	outbox := &recordingOutbox{}
	notifier := &reminderCaptureNotifier{}
	cfg := calendarTestConfig(db)
	cfg.Outbox = outbox
	cfg.ParentsURL = "https://parents.test"
	cfg.Preferences = optedOutAppointmentPreferences{optedOutType: notifications.TypeParentAppointmentReminder}
	cfg.ReminderNotifier = notifier
	service := calendarSvc.NewService(cfg)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Reminder", "Consent")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	ctx := calendarContext(organizerAccount.ID)
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
	require.Len(t, outbox.enqueued, 1, "the lifecycle e-mail uses explicit opt-out semantics")

	startsAt := berlinInstant(t, appointmentDate, 18, 0)
	queued, err := service.EnqueueDueAppointmentReminders(ctx, startsAt.Add(-5*time.Minute), startsAt.Add(5*time.Minute))
	require.NoError(t, err)
	assert.Zero(t, queued, "reminders must respect an explicit guardian opt-out")
	assert.Len(t, outbox.enqueued, 1)
	require.Len(t, notifier.events, 1, "an e-mail opt-out must not suppress an opted-in push")
	assert.Equal(t, notifications.TypeParentAppointmentReminder, notifier.events[0].Type)
}

func TestCalendarServiceIntegration_AppointmentReminderPushesWithoutGuardianEmail(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	outbox := &recordingOutbox{}
	notifier := &reminderCaptureNotifier{}
	cfg := calendarTestConfig(db)
	cfg.Outbox = outbox
	cfg.ParentsURL = "https://parents.test"
	cfg.Notifier = notifier
	cfg.ReminderNotifier = notifier
	cfg.Preferences = reminderPreferences{}
	service := calendarSvc.NewService(cfg)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Reminder", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	ctx := calendarContext(organizerAccount.ID)
	_, err := db.NewUpdate().
		TableExpr(`users.guardian_profiles`).
		Set(`email = NULL`).
		Where(`id = ?`, parentChain.GuardianProfileID).
		Exec(ctx)
	require.NoError(t, err)

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
	notifier.events = nil // Ignore the appointment publication push.

	startsAt := berlinInstant(t, appointmentDate, 18, 0)
	queued, err := service.EnqueueDueAppointmentReminders(ctx, startsAt.Add(-5*time.Minute), startsAt.Add(5*time.Minute))
	require.NoError(t, err)
	assert.Zero(t, queued, "an e-mail-less guardian has no outbox e-mail to queue")
	require.Len(t, notifier.events, 1)
	assert.Equal(t, notifications.TypeParentAppointmentReminder, notifier.events[0].Type)
	assert.Equal(t, []int64{parentChain.AccountID}, notifier.events[0].Audience.GuardianAccountIDs)

	queued, err = service.EnqueueDueAppointmentReminders(ctx, startsAt.Add(-5*time.Minute), startsAt.Add(5*time.Minute))
	require.NoError(t, err)
	assert.Zero(t, queued)
	assert.Len(t, notifier.events, 1, "the overlapping scheduler scan must not repeat the push")
}

func TestCalendarServiceIntegration_AppointmentReminderRetriesPushAfterDispatchFailure(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	outbox := &recordingOutbox{}
	notifier := &reminderCaptureNotifier{err: errors.New("temporary notification failure")}
	cfg := calendarTestConfig(db)
	cfg.Outbox = outbox
	cfg.ParentsURL = "https://parents.test"
	cfg.Notifier = notifier
	cfg.ReminderNotifier = notifier
	cfg.Preferences = reminderPreferences{}
	service := calendarSvc.NewService(cfg)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Reminder", "Retry")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	ctx := calendarContext(organizerAccount.ID)
	_, err := db.NewUpdate().
		TableExpr(`users.guardian_profiles`).
		Set(`email = NULL`).
		Where(`id = ?`, parentChain.GuardianProfileID).
		Exec(ctx)
	require.NoError(t, err)

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
	notifier.events = nil

	startsAt := berlinInstant(t, appointmentDate, 18, 0)
	queued, err := service.EnqueueDueAppointmentReminders(ctx, startsAt.Add(-5*time.Minute), startsAt.Add(5*time.Minute))
	require.Error(t, err)
	assert.Zero(t, queued)
	assert.Empty(t, notifier.events, "a failed dispatch must not be marked delivered")

	notifier.err = nil
	queued, err = service.EnqueueDueAppointmentReminders(ctx, startsAt.Add(-5*time.Minute), startsAt.Add(5*time.Minute))
	require.NoError(t, err)
	assert.Zero(t, queued)
	require.Len(t, notifier.events, 1, "the next scan must retry the unclaimed push")
	assert.Equal(t, []int64{parentChain.AccountID}, notifier.events[0].Audience.GuardianAccountIDs)
}

// E-mail and push are separate channels. An installation without an e-mail
// outbox must still remind the guardians who agreed to hear it on their device;
// gating the scan on the outbox would silence the push half along with it.
func TestCalendarServiceIntegration_AppointmentReminderPushesWithoutOutbox(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Reminder", "NoOutbox")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	ctx := calendarContext(organizerAccount.ID)
	appointmentDate := timezone.NewDate(2026, 4, 2)
	detail, err := setupCalendarServiceWithOutbox(t, db, &recordingOutbox{}).
		CreateStaffAppointment(ctx, calendarSvc.CreateAppointmentRequest{
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

	notifier := &reminderCaptureNotifier{}
	cfg := calendarTestConfig(db)
	cfg.Outbox = nil
	cfg.ParentsURL = "https://parents.test"
	cfg.ReminderNotifier = notifier
	cfg.Preferences = reminderPreferences{}

	startsAt := berlinInstant(t, appointmentDate, 18, 0)
	queued, err := calendarSvc.NewService(cfg).
		EnqueueDueAppointmentReminders(ctx, startsAt.Add(-5*time.Minute), startsAt.Add(5*time.Minute))
	require.NoError(t, err)
	assert.Zero(t, queued, "without an outbox there is no e-mail to queue")
	require.Len(t, notifier.events, 1, "the push must not depend on the e-mail outbox")
	assert.Equal(t, notifications.TypeParentAppointmentReminder, notifier.events[0].Type)
	assert.Equal(t, []int64{parentChain.AccountID}, notifier.events[0].Audience.GuardianAccountIDs)
}

// A guardian without a registered device (or a school without VAPID keys) is an
// ordinary state, not a failure: the scan stays quiet and the e-mail goes out.
// The push claim must not survive it though — nothing was delivered, so a later
// scan has to be free to send once a device exists.
func TestCalendarServiceIntegration_AppointmentReminderRetriesPushAfterMissingSubscribers(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	outbox := &recordingOutbox{}
	notifier := &reminderCaptureNotifier{err: notifications.ErrNoWebPushSubscribers}
	cfg := calendarTestConfig(db)
	cfg.Outbox = outbox
	cfg.ParentsURL = "https://parents.test"
	cfg.ReminderNotifier = notifier
	cfg.Preferences = reminderPreferences{}
	service := calendarSvc.NewService(cfg)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Reminder", "NoPush")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	appointmentDate := timezone.NewDate(2026, 4, 2)
	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
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
	ctx := calendarContext(organizerAccount.ID)
	queued, err := service.EnqueueDueAppointmentReminders(ctx, startsAt.Add(-5*time.Minute), startsAt.Add(5*time.Minute))
	require.NoError(t, err)
	assert.Equal(t, 1, queued)

	notifier.err = nil
	queued, err = service.EnqueueDueAppointmentReminders(ctx, startsAt.Add(-5*time.Minute), startsAt.Add(5*time.Minute))
	require.NoError(t, err)
	assert.Zero(t, queued, "the outbox key keeps the second scan from queueing the e-mail again")
	require.Len(t, notifier.events, 1, "a missing push audience must not retain the claim")
	assert.Equal(t, []int64{parentChain.AccountID}, notifier.events[0].Audience.GuardianAccountIDs)
}

func TestCalendarServiceIntegration_AppointmentLifecycleEmailHonorsOptOut(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	outbox := &recordingOutbox{}
	cfg := calendarTestConfig(db)
	cfg.Outbox = outbox
	cfg.ParentsURL = "https://parents.test"
	cfg.Preferences = optedOutAppointmentPreferences{optedOutType: notifications.TypeParentAppointment}
	service := calendarSvc.NewService(cfg)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Reminder", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	appointmentDate := timezone.NewDate(2026, 4, 2)
	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
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
	assert.Empty(t, outbox.enqueued, "a guardian who opted out must not receive the appointment e-mail")
}

// A cancelled appointment must not remind anyone: the guardians already got the
// cancellation notice, and a reminder for it would contradict it.
func TestCalendarServiceIntegration_CancelledAppointmentIsNotReminded(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	outbox := &recordingOutbox{}
	service := setupCalendarServiceWithOutbox(t, db, outbox)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "CancelRem", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	ctx := calendarContext(organizerAccount.ID)
	appointmentDate := timezone.NewDate(2026, 4, 9)
	detail, err := service.CreateStaffAppointment(ctx, calendarSvc.CreateAppointmentRequest{
		Title:        "Ausflug",
		StartDate:    appointmentDate,
		EndDate:      appointmentDate,
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(12, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		SendEmail:    true,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })

	_, err = service.CancelStaffAppointment(ctx, detail.Appointment.ID)
	require.NoError(t, err)

	before := len(outbox.enqueued)
	startsAt := berlinInstant(t, appointmentDate, 9, 0)
	queued, err := service.EnqueueDueAppointmentReminders(ctx, startsAt.Add(-5*time.Minute), startsAt.Add(5*time.Minute))
	require.NoError(t, err)
	assert.Zero(t, queued)
	assert.Len(t, outbox.enqueued, before, "a cancelled appointment has nothing to remind about")
}

// An appointment whose organizer did NOT tick "Eltern benachrichtigen" is
// outside the reminder's reach: the reminder is a second notice on a channel the
// organizer opted into, not a new one.
func TestCalendarServiceIntegration_SilentAppointmentIsNotReminded(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	outbox := &recordingOutbox{}
	service := setupCalendarServiceWithOutbox(t, db, outbox)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "SilentRem", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	ctx := calendarContext(organizerAccount.ID)
	appointmentDate := timezone.NewDate(2026, 4, 16)
	detail, err := service.CreateStaffAppointment(ctx, calendarSvc.CreateAppointmentRequest{
		Title:        "Interner Termin",
		StartDate:    appointmentDate,
		EndDate:      appointmentDate,
		StartTime:    wallClock(15, 0),
		EndTime:      wallClock(16, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		SendEmail:    false,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })
	require.Empty(t, outbox.enqueued, "no publish mail either")

	startsAt := berlinInstant(t, appointmentDate, 15, 0)
	queued, err := service.EnqueueDueAppointmentReminders(ctx, startsAt.Add(-5*time.Minute), startsAt.Add(5*time.Minute))
	require.NoError(t, err)
	assert.Zero(t, queued)
	assert.Empty(t, outbox.enqueued)
}

// Every occurrence of a series gets its own reminder, and each carries its own
// date — a reminder that advertised the series start would be wrong for all but
// the first.
func TestCalendarServiceIntegration_RecurringAppointmentRemindsPerOccurrence(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	outbox := &recordingOutbox{}
	service := setupCalendarServiceWithOutbox(t, db, outbox)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "SeriesRem", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	ctx := calendarContext(organizerAccount.ID)
	seriesStart := timezone.NewDate(2026, 5, 6) // a Wednesday
	endsOn := timezone.NewDate(2026, 5, 27)
	detail, err := service.CreateStaffAppointment(ctx, calendarSvc.CreateAppointmentRequest{
		Title:        "Mittwochs-AG",
		StartDate:    seriesStart,
		EndDate:      seriesStart,
		StartTime:    wallClock(14, 0),
		EndTime:      wallClock(15, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		SendEmail:    true,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     calModels.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"wednesday"},
			EndsOn:        &endsOn,
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })

	secondOccurrence := timezone.NewDate(2026, 5, 13)
	startsAt := berlinInstant(t, secondOccurrence, 14, 0)

	before := len(outbox.enqueued)
	queued, err := service.EnqueueDueAppointmentReminders(ctx, startsAt.Add(-5*time.Minute), startsAt.Add(5*time.Minute))
	require.NoError(t, err)
	require.Equal(t, 1, queued, "only the occurrence inside the window")

	last := outbox.enqueued[len(outbox.enqueued)-1]
	assert.Equal(t, platformModels.EmailKindAppointmentReminder, last.Kind)
	assert.Contains(t, last.Payload["when_text"], "13.05.2026",
		"the reminder advertises its own occurrence, not the series start")

	// The same window again must add nothing: the idempotency key is what makes a
	// re-run, an overlapping window or a second scheduler process harmless.
	sameWindowCount := len(outbox.enqueued)
	queued, err = service.EnqueueDueAppointmentReminders(ctx, startsAt.Add(-5*time.Minute), startsAt.Add(5*time.Minute))
	require.NoError(t, err)
	assert.Zero(t, queued)
	assert.Len(t, outbox.enqueued, sameWindowCount)

	assert.Greater(t, len(outbox.enqueued), before)
}

// A count-bounded series with a huge interval is valid input: only
// occurrence_count is capped, interval_count is not. The candidate query bounds
// such a series by computing its final date, and an unguarded computation runs
// past the date range — which fails the scan for the whole tenant, not just for
// this appointment, so nobody in that school gets a reminder that tick.
func TestCalendarServiceIntegration_ExtremeRecurrenceIntervalDoesNotBreakTheScan(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	outbox := &recordingOutbox{}
	service := setupCalendarServiceWithOutbox(t, db, outbox)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "ExtremeRem", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	ctx := calendarContext(organizerAccount.ID)
	appointmentDate := timezone.NewDate(2026, 4, 2)
	// The series starts before the scan window, so it only reaches the candidate
	// query through its recurrence bound — the arm that does the date arithmetic.
	seriesStart := timezone.NewDate(2025, 4, 2)
	occurrenceCount := calModels.MaxRecurrenceOccurrenceCount
	extreme, err := service.CreateStaffAppointment(ctx, calendarSvc.CreateAppointmentRequest{
		Title:        "Jahrhundertreihe",
		StartDate:    seriesStart,
		EndDate:      seriesStart,
		StartTime:    wallClock(18, 0),
		EndTime:      wallClock(19, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		SendEmail:    true,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:       calModels.RecurrenceFrequencyYearly,
			IntervalCount:   10000,
			OccurrenceCount: &occurrenceCount,
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, extreme.Appointment.ID) })

	// A second, ordinary appointment in the same window: the point is that it
	// still gets its reminder while the extreme series sits in the same scan.
	plain, err := service.CreateStaffAppointment(ctx, calendarSvc.CreateAppointmentRequest{
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
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, plain.Appointment.ID) })

	startsAt := berlinInstant(t, appointmentDate, 18, 0)
	queued, err := service.EnqueueDueAppointmentReminders(ctx, startsAt.Add(-5*time.Minute), startsAt.Add(5*time.Minute))
	require.NoError(t, err, "the scan must survive a series whose bound overshoots the date range")
	assert.GreaterOrEqual(t, queued, 1, "the ordinary appointment is still reminded")
}

func TestCalendarServiceIntegration_ReminderForMovedRecurringOccurrence(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	outbox := &recordingOutbox{}
	service := setupCalendarServiceWithOutbox(t, db, outbox)
	repos := repositories.NewFactory(db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "MovedReminder", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	ctx := calendarContext(organizerAccount.ID)
	seriesStart := timezone.NewDate(2026, 1, 7)
	endsOn := timezone.NewDate(2026, 1, 7)
	detail, err := service.CreateStaffAppointment(ctx, calendarSvc.CreateAppointmentRequest{
		Title:        "Verschobener Termin",
		StartDate:    seriesStart,
		EndDate:      seriesStart,
		StartTime:    wallClock(14, 0),
		EndTime:      wallClock(15, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		SendEmail:    true,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     calModels.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"wednesday"},
			EndsOn:        &endsOn,
		},
		Targets: []calendarSvc.AppointmentTarget{{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID}},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })

	movedDate := timezone.NewDate(2026, 2, 4)
	require.NoError(t, repos.CalendarOccurrenceOverride.Create(ctx, &calModels.AppointmentOccurrenceOverride{
		AppointmentID:  detail.Appointment.ID,
		OccurrenceDate: seriesStart,
		StartDate:      &movedDate,
		EndDate:        &movedDate,
	}))

	startsAt := berlinInstant(t, movedDate, 14, 0)
	queued, err := service.EnqueueDueAppointmentReminders(ctx, startsAt.Add(-5*time.Minute), startsAt.Add(5*time.Minute))
	require.NoError(t, err)
	assert.Equal(t, 1, queued)
	assert.Contains(t, outbox.enqueued[len(outbox.enqueued)-1].Payload["when_text"], "04.02.2026")
}
