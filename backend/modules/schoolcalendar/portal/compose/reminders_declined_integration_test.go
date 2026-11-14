package compose_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"

	calModels "github.com/moto-ogs/project-phoenix/modules/appointments"
	calendarSvc "github.com/moto-ogs/project-phoenix/modules/schoolcalendar/portal"
	calendarCompose "github.com/moto-ogs/project-phoenix/modules/schoolcalendar/portal/compose"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A parent who declined a date gets no reminder for it, neither by e-mail nor
// by push; the dates they still come to are reminded as before (#7014). A
// declined series silences every date except the one answered on its own.
func TestCalendarServiceIntegration_DeclinedOccurrenceIsNotReminded(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	outbox := &recordingOutbox{}
	notifier := &reminderCaptureNotifier{}
	cfg := calendarTestConfig(t, db)
	cfg.Outbox = outbox
	cfg.ParentsURL = "https://parents.test"
	cfg.Notifier = notifier
	cfg.ReminderNotifier = notifier
	cfg.Preferences = reminderPreferences{}
	service := owners.NewCalendarPortal(cfg)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Declined", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	ctx := calendarContext(t, organizerAccount.ID)
	seriesStart := portalDate(2026, 5, 6) // a Wednesday
	endsOn := portalDate(2026, 5, 20)
	detail, err := service.CreateStaffAppointment(ctx, calendarSvc.CreateAppointmentRequest{
		Title:        "Elterncafé",
		StartDate:    seriesStart,
		EndDate:      seriesStart,
		StartTime:    wallClock(15, 0),
		EndTime:      wallClock(16, 0),
		DeliveryMode: calModels.DeliveryModeRSVPRequired,
		SendEmail:    true,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency: calModels.RecurrenceFrequencyWeekly, IntervalCount: 1,
			Weekdays: []string{"wednesday"}, EndsOn: &endsOn,
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	recipientID := findRecipientByGuardian(t, detail, parentChain.GuardianProfileID)
	parentCtx := testpkg.Ctx(t)
	answer := func(status string, occurrence calendarSvc.Date) {
		t.Helper()
		require.NoError(t, service.RespondToParentInvitation(parentCtx, parentChain.AccountID, recipientID,
			calendarSvc.InvitationAnswer{Status: status, OccurrenceDate: occurrence}))
	}
	// remind scans the window around one date and reports the queued mails and
	// the reminder pushes that went out.
	remind := func(date calendarSvc.Date) (int, int) {
		t.Helper()
		notifier.events = nil
		startsAt := berlinInstant(t, date, 15, 0)
		queued, err := api.NewCalendarReminderCommand(testpkg.NewDatabase(t, db), service).
			EnqueueDueAppointmentReminders(ctx, startsAt.Add(-5*time.Minute), startsAt.Add(5*time.Minute))
		require.NoError(t, err)
		pushes := 0
		for _, event := range notifier.events {
			if event.Type == notificationTypeParentAppointmentReminder {
				pushes++
			}
		}
		return queued, pushes
	}

	// The series is accepted, the date in the middle declined on its own.
	answer(calModels.ResponseStatusAccepted, "")
	answer(calModels.ResponseStatusDeclined, portalDate(2026, 5, 13))

	queued, pushes := remind(portalDate(2026, 5, 6))
	assert.Equal(t, 1, queued, "an accepted date is reminded by e-mail")
	assert.Equal(t, 1, pushes, "an accepted date is reminded by push")

	queued, pushes = remind(portalDate(2026, 5, 13))
	assert.Zero(t, queued, "the declined date gets no reminder e-mail")
	assert.Zero(t, pushes, "the declined date gets no reminder push")

	// The whole series is declined, then the last date accepted on its own.
	answer(calModels.ResponseStatusDeclined, "")
	answer(calModels.ResponseStatusAccepted, portalDate(2026, 5, 20))

	queued, pushes = remind(portalDate(2026, 5, 20))
	assert.Equal(t, 1, queued, "a date accepted against a declined series is reminded")
	assert.Equal(t, 1, pushes)
}

// A one-off appointment the parent declined gets no reminder either.
func TestCalendarServiceIntegration_DeclinedAppointmentIsNotReminded(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	outbox := &recordingOutbox{}
	service := setupCalendarServiceWithOutbox(t, db, outbox)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "DeclinedOnce", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	ctx := calendarContext(t, organizerAccount.ID)
	appointmentDate := portalDate(2026, 4, 2)
	detail, err := service.CreateStaffAppointment(ctx, calendarSvc.CreateAppointmentRequest{
		Title:        "Elternabend",
		StartDate:    appointmentDate,
		EndDate:      appointmentDate,
		StartTime:    wallClock(18, 0),
		EndTime:      wallClock(19, 0),
		DeliveryMode: calModels.DeliveryModeRSVPRequired,
		SendEmail:    true,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	recipientID := findRecipientByGuardian(t, detail, parentChain.GuardianProfileID)
	require.NoError(t, service.RespondToParentInvitation(testpkg.Ctx(t), parentChain.AccountID, recipientID,
		calendarSvc.InvitationAnswer{Status: calModels.ResponseStatusDeclined}))

	startsAt := berlinInstant(t, appointmentDate, 18, 0)
	before := len(outbox.enqueued)
	queued, err := api.NewCalendarReminderCommand(testpkg.NewDatabase(t, db), service).
		EnqueueDueAppointmentReminders(ctx, startsAt.Add(-5*time.Minute), startsAt.Add(5*time.Minute))
	require.NoError(t, err)
	assert.Zero(t, queued)
	assert.Len(t, outbox.enqueued, before)
}

// A parent who declines between the scan and the push dispatch gets no push:
// the dispatch reads the answers again in its own transaction (#7014).
func TestCalendarServiceIntegration_DeclineBeforePushDispatchDropsThePush(t *testing.T) {
	t.Parallel()

	db, module := setupCalendarModule(t)
	_, organizer := testpkg.CreateTestCalendarStaff(t, db, "DeclinedLate", "Organizer")
	parent := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := calendarContext(t, organizer.ID)
	date := portalDate(2026, 4, 2)
	_, err := module.Calendar.CreateStaffAppointment(ctx, calendarSvc.CreateAppointmentRequest{
		Title: "Elternabend", StartDate: date, EndDate: date,
		StartTime: wallClock(18, 0), EndTime: wallClock(19, 0),
		DeliveryMode: calModels.DeliveryModeRSVPRequired, SendEmail: true,
		Targets: []calendarSvc.AppointmentTarget{{Type: calModels.TargetTypeGuardianProfile, ID: &parent.GuardianProfileID}},
	})
	require.NoError(t, err)

	effects := module.Calendar.ReminderEffects()
	effects.ParentsURL = "https://parents.test"
	effects.FilterEmail = func(_ context.Context, ids []int64) ([]int64, error) { return ids, nil }
	effects.FilterPush = effects.FilterEmail
	// The scan reads the open answer; by the dispatch the parent has declined.
	audiences, scans := effects.Audiences, 0
	effects.Audiences = func(ctx context.Context, ids []int64) (map[int64]calendarCompose.GuardianNotificationAudience, error) {
		values, err := audiences(ctx, ids)
		scans++
		if err != nil || scans == 1 {
			return values, err
		}
		for id, value := range values {
			for profileID, answer := range value.Answers {
				answer.Status = calModels.ResponseStatusDeclined
				value.Answers[profileID] = answer
			}
			values[id] = value
		}
		return values, nil
	}
	pushes := 0
	effects.Push = func(context.Context, int64, []int64, []int64, string) (bool, error) {
		pushes++
		return true, nil
	}

	startsAt := berlinInstant(t, date, 18, 0)
	queued, err := api.NewCalendarReminderCommand(testpkg.NewDatabase(t, db), reminderEffectsSource{effects}).
		EnqueueDueAppointmentReminders(ctx, startsAt.Add(-5*time.Minute), startsAt.Add(5*time.Minute))
	require.NoError(t, err)
	assert.Equal(t, 1, queued, "the open answer at the scan queues the e-mail")
	assert.Equal(t, 2, scans, "the dispatch read the answers again")
	assert.Zero(t, pushes, "the decline before the dispatch drops the push")
}
