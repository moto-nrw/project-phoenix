package calendar_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	calendarSvc "github.com/moto-nrw/project-phoenix/services/calendar"
	platformService "github.com/moto-nrw/project-phoenix/services/platform"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	usercontextSvc "github.com/moto-nrw/project-phoenix/services/usercontext"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func calendarTestConfig(db *bun.DB) calendarSvc.Config {
	repos := repositories.NewFactory(db)
	userContext := usercontextSvc.NewUserContextServiceWithRepos(
		usercontextSvc.UserContextRepositories{
			AccountRepo:        repos.Account,
			PersonRepo:         repos.Person,
			StaffRepo:          repos.Staff,
			TeacherRepo:        repos.Teacher,
			StudentRepo:        repos.Student,
			EducationGroupRepo: repos.Group,
			ActivityGroupRepo:  repos.ActivityGroup,
			ActiveGroupRepo:    repos.ActiveGroup,
			VisitsRepo:         repos.ActiveVisit,
			SupervisorRepo:     repos.GroupSupervisor,
			ProfileRepo:        repos.Profile,
			SubstitutionRepo:   repos.GroupSubstitution,
		},
		slog.Default(),
	)

	return calendarSvc.Config{
		AppointmentRepo:      repos.CalendarAppointment,
		RecurrenceRepo:       repos.CalendarRecurrenceRule,
		RecipientRepo:        repos.CalendarAppointmentRecipient,
		RecipientStudentRepo: repos.CalendarAppointmentRecipientChild,
		TargetRepo:           repos.CalendarAppointmentTarget,
		OverrideRepo:         repos.CalendarOccurrenceOverride,
		StaffRepo:            repos.Staff,
		StudentRepo:          repos.Student,
		GuardianProfileRepo:  repos.GuardianProfile,
		StudentGuardianRepo:  repos.StudentGuardian,
		ChildRepo:            repos.ParentChild,
		GroupRepo:            repos.Group,
		InstanceStaffRepo:    repos.InstanceStaff,
		InstanceStudentRepo:  repos.InstanceStudent,
		ActivityInstanceRepo: repos.ActivityInstance,
		StaffShiftRepo:       repos.StaffShift,
		ShiftTypeRepo:        repos.ShiftType,
		CareDays: scheduleSvc.NewCareDayService(scheduleSvc.CareDayDependencies{
			ArrivalSchedules:  repos.StudentArrivalSchedule,
			ArrivalExceptions: repos.StudentArrivalException,
			PickupSchedules:   repos.StudentPickupSchedule,
			PickupExceptions:  repos.StudentPickupException,
		}),
		UserContext: userContext,
		DB:          db,
	}
}

func setupCalendarService(t *testing.T, db *bun.DB) calendarSvc.Service {
	t.Helper()
	return calendarSvc.NewService(calendarTestConfig(db))
}

func setupCalendarServiceWithOutbox(t *testing.T, db *bun.DB, outbox calendarSvc.OutboxEnqueuer) calendarSvc.Service {
	t.Helper()
	cfg := calendarTestConfig(db)
	cfg.Outbox = outbox
	cfg.ParentsURL = "https://parents.test"
	return calendarSvc.NewService(cfg)
}

type recordingOutbox struct {
	enqueued  []platformService.EnqueueRequest
	cancelled int
}

func (r *recordingOutbox) Enqueue(_ context.Context, req platformService.EnqueueRequest) (*platformModels.EmailOutbox, error) {
	r.enqueued = append(r.enqueued, req)
	return &platformModels.EmailOutbox{}, nil
}

func (r *recordingOutbox) CancelPendingByRelatedEntity(context.Context, string, int64, string) (int64, error) {
	r.cancelled++
	return 0, nil
}

func calendarContext(accountID int64) context.Context {
	return context.WithValue(testpkg.TenantContext(1), jwt.CtxClaims, jwt.AppClaims{
		ID:       int(accountID),
		TenantID: 1,
	})
}

func wallClock(h, m int) time.Time {
	return timezone.WallClock(time.Date(2024, 1, 1, h, m, 0, 0, time.UTC))
}

func cleanupCalendarAppointment(t *testing.T, db *bun.DB, appointmentID int64) {
	t.Helper()
	testpkg.CleanupTableRecords(t, db, "calendar.appointments", appointmentID)
}

func eventDates(events []calendarSvc.Event, source string) []string {
	dates := make([]string, 0, len(events))
	for _, event := range events {
		if event.Source == source {
			dates = append(dates, event.StartDate)
		}
	}
	sort.Strings(dates)
	return dates
}

func findRecipientByStaff(t *testing.T, detail *calendarSvc.AppointmentDetail, staffID int64) int64 {
	t.Helper()
	for _, recipient := range detail.Recipients {
		if recipient.StaffID != nil && *recipient.StaffID == staffID {
			return recipient.ID
		}
	}
	t.Fatalf("staff recipient %d not found", staffID)
	return 0
}

func findRecipientByGuardian(t *testing.T, detail *calendarSvc.AppointmentDetail, guardianID int64) int64 {
	t.Helper()
	id := findOptionalRecipientByGuardian(detail, guardianID)
	require.NotZero(t, id)
	return id
}

func findOptionalRecipientByGuardian(detail *calendarSvc.AppointmentDetail, guardianID int64) int64 {
	for _, recipient := range detail.Recipients {
		if recipient.GuardianProfileID != nil && *recipient.GuardianProfileID == guardianID {
			return recipient.ID
		}
	}
	return 0
}

func TestCalendarServiceIntegration_CreateRecurringAppointmentAndResponses(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Calendar", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "Calendar", "Invitee")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, invitedStaff.ID, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, invitedAccount.ID, organizerAccount.ID)
	})

	endsOn := timezone.NewDate(2026, 1, 12)
	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Recurring planning",
		Description:  testpkg.StrPtr("Weekly planning with staff and parents"),
		Location:     testpkg.StrPtr("Room 1"),
		StartDate:    timezone.NewDate(2026, 1, 5),
		EndDate:      timezone.NewDate(2026, 1, 5),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: calModels.DeliveryModeRSVPRequired,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     calModels.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"monday", "wednesday"},
			EndsOn:        &endsOn,
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeStaff, ID: &invitedStaff.ID},
			{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, detail)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })

	assert.Equal(t, organizer.ID, detail.Appointment.OrganizerStaffID)
	require.Len(t, detail.Recipients, 2)
	require.Len(t, detail.Targets, 2)
	require.NotNil(t, detail.Recurrence)
	assert.Equal(t, []string{"monday", "wednesday"}, detail.Recurrence.Weekdays)

	staffRecipientID := findRecipientByStaff(t, detail, invitedStaff.ID)
	guardianRecipientID := findRecipientByGuardian(t, detail, parentChain.GuardianProfileID)

	staffEvents, err := service.ListMyStaffEvents(
		calendarContext(invitedAccount.ID),
		timezone.NewDate(2026, 1, 5),
		timezone.NewDate(2026, 1, 12),
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"2026-01-05", "2026-01-07", "2026-01-12"}, eventDates(staffEvents, calModels.EventSourceAppointment))
	require.NotEmpty(t, staffEvents)
	assert.Equal(t, calModels.ResponseStatusPending, *staffEvents[0].ResponseStatus)
	assert.True(t, staffEvents[0].CanRespond)

	require.NoError(t, service.RespondToStaffInvitation(calendarContext(invitedAccount.ID), staffRecipientID, calModels.ResponseStatusAccepted))
	staffEvents, err = service.ListMyStaffEvents(
		calendarContext(invitedAccount.ID),
		timezone.NewDate(2026, 1, 5),
		timezone.NewDate(2026, 1, 5),
	)
	require.NoError(t, err)
	require.NotEmpty(t, staffEvents)
	assert.Equal(t, calModels.ResponseStatusAccepted, *staffEvents[0].ResponseStatus)

	parentEvents, err := service.ListMyParentEvents(
		testpkg.TenantContext(1),
		parentChain.AccountID,
		timezone.NewDate(2026, 1, 5),
		timezone.NewDate(2026, 1, 12),
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"2026-01-05", "2026-01-07", "2026-01-12"}, eventDates(parentEvents, calModels.EventSourceAppointment))
	require.NotEmpty(t, parentEvents)
	assert.Equal(t, calModels.ResponseStatusPending, *parentEvents[0].ResponseStatus)
	assert.True(t, parentEvents[0].CanRespond)

	require.NoError(t, service.RespondToParentInvitation(testpkg.TenantContext(1), parentChain.AccountID, guardianRecipientID, calModels.ResponseStatusDeclined))
	parentEvents, err = service.ListMyParentEvents(
		testpkg.TenantContext(1),
		parentChain.AccountID,
		timezone.NewDate(2026, 1, 5),
		timezone.NewDate(2026, 1, 5),
	)
	require.NoError(t, err)
	require.NotEmpty(t, parentEvents)
	assert.Equal(t, calModels.ResponseStatusDeclined, *parentEvents[0].ResponseStatus)
}

func TestCalendarServiceIntegration_InformationalAppointmentCannotBeAnswered(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Info", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "Info", "Invitee")
	t.Cleanup(func() {
		testpkg.CleanupStaffFixtures(t, db, invitedStaff.ID, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, invitedAccount.ID, organizerAccount.ID)
	})

	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Information only",
		StartDate:    timezone.NewDate(2026, 2, 2),
		EndDate:      timezone.NewDate(2026, 2, 2),
		StartTime:    wallClock(12, 0),
		EndTime:      wallClock(12, 30),
		DeliveryMode: calModels.DeliveryModeInformational,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })

	recipientID := findRecipientByStaff(t, detail, invitedStaff.ID)
	err = service.RespondToStaffInvitation(calendarContext(invitedAccount.ID), recipientID, calModels.ResponseStatusAccepted)
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrInvalidRequest))
}

func TestCalendarServiceIntegration_UpdateCancelDeleteLifecycle(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Lifecycle", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "Lifecycle", "Invitee")
	t.Cleanup(func() {
		testpkg.CleanupStaffFixtures(t, db, invitedStaff.ID, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, invitedAccount.ID, organizerAccount.ID)
	})

	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Original title",
		Location:     testpkg.StrPtr("Room 1"),
		StartDate:    timezone.NewDate(2026, 3, 2),
		EndDate:      timezone.NewDate(2026, 3, 2),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: calModels.DeliveryModeRSVPRequired,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })

	// A non-organizer may not edit the appointment.
	_, err = service.UpdateStaffAppointment(calendarContext(invitedAccount.ID), detail.Appointment.ID, calendarSvc.UpdateAppointmentRequest{
		Title:     "Hijack",
		StartDate: timezone.NewDate(2026, 3, 2),
		EndDate:   timezone.NewDate(2026, 3, 2),
		StartTime: wallClock(9, 0),
		EndTime:   wallClock(10, 0),
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrForbidden))

	// The organizer edits event details; recipients (RSVP audience) are preserved.
	updated, err := service.UpdateStaffAppointment(calendarContext(organizerAccount.ID), detail.Appointment.ID, calendarSvc.UpdateAppointmentRequest{
		Title:     "Updated title",
		Location:  testpkg.StrPtr("Room 2"),
		StartDate: timezone.NewDate(2026, 3, 3),
		EndDate:   timezone.NewDate(2026, 3, 3),
		StartTime: wallClock(11, 0),
		EndTime:   wallClock(12, 0),
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated title", updated.Appointment.Title)
	require.Len(t, updated.Recipients, 1)

	// The organizer can re-read the full detail (used to prefill the edit modal).
	detailAfter, err := service.GetStaffAppointmentDetail(calendarContext(organizerAccount.ID), detail.Appointment.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated title", detailAfter.Appointment.Title)
	assert.Equal(t, "Room 2", *detailAfter.Appointment.Location)
	// A non-organizer is refused.
	_, err = service.GetStaffAppointmentDetail(calendarContext(invitedAccount.ID), detail.Appointment.ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrForbidden))

	events, err := service.ListMyStaffEvents(calendarContext(invitedAccount.ID), timezone.NewDate(2026, 3, 1), timezone.NewDate(2026, 3, 10))
	require.NoError(t, err)
	require.NotEmpty(t, events)
	assert.Equal(t, "Updated title", events[0].Title)
	assert.Equal(t, "2026-03-03", events[0].StartDate)
	assert.False(t, events[0].Cancelled)

	// Cancelling marks the event but keeps it visible so parents/staff see it.
	_, err = service.CancelStaffAppointment(calendarContext(organizerAccount.ID), detail.Appointment.ID)
	require.NoError(t, err)
	events, err = service.ListMyStaffEvents(calendarContext(invitedAccount.ID), timezone.NewDate(2026, 3, 1), timezone.NewDate(2026, 3, 10))
	require.NoError(t, err)
	require.NotEmpty(t, events)
	assert.True(t, events[0].Cancelled)

	// Deleting removes it from every reader.
	require.NoError(t, service.DeleteStaffAppointment(calendarContext(organizerAccount.ID), detail.Appointment.ID))
	events, err = service.ListMyStaffEvents(calendarContext(invitedAccount.ID), timezone.NewDate(2026, 3, 1), timezone.NewDate(2026, 3, 10))
	require.NoError(t, err)
	assert.Empty(t, eventDates(events, calModels.EventSourceAppointment))
}

func TestCalendarServiceIntegration_GuardianNotifications(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	outbox := &recordingOutbox{}
	service := setupCalendarServiceWithOutbox(t, db, outbox)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Notify", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	// Create WITHOUT send_email: no mail is queued.
	silent, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Silent",
		StartDate:    timezone.NewDate(2026, 4, 1),
		EndDate:      timezone.NewDate(2026, 4, 1),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, silent.Appointment.ID) })
	assert.Empty(t, outbox.enqueued, "no email should be queued without send_email")

	// Create WITH send_email: one published mail to the guardian.
	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Elternabend",
		Location:     testpkg.StrPtr("Aula"),
		StartDate:    timezone.NewDate(2026, 4, 2),
		EndDate:      timezone.NewDate(2026, 4, 2),
		StartTime:    wallClock(18, 0),
		EndTime:      wallClock(19, 30),
		DeliveryMode: calModels.DeliveryModeInformational,
		SendEmail:    true,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })
	require.Len(t, outbox.enqueued, 1)
	published := outbox.enqueued[0]
	assert.Equal(t, platformModels.EmailKindAppointmentPublished, published.Kind)
	assert.Equal(t, platformModels.EmailRelatedTypeAppointment, published.RelatedEntityType)
	assert.Equal(t, detail.Appointment.ID, published.RelatedEntityID)
	assert.Equal(t, "Elternabend", published.Payload["title"])
	assert.NotEmpty(t, published.Payload["recipient_email"])

	// Cancelling queues a cancellation notice and clears pending mail.
	_, err = service.CancelStaffAppointment(calendarContext(organizerAccount.ID), detail.Appointment.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, outbox.cancelled, "cancel should clear pending mail once")
	require.Len(t, outbox.enqueued, 2)
	assert.Equal(t, platformModels.EmailKindAppointmentCancelled, outbox.enqueued[1].Kind)
}

func TestCalendarServiceIntegration_CancelHonoursEmailOptOutAndTransition(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	outbox := &recordingOutbox{}
	service := setupCalendarServiceWithOutbox(t, db, outbox)
	repos := repositories.NewFactory(db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "OptOut", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	newSilentAppointment := func(title string, day int) *calendarSvc.AppointmentDetail {
		detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
			Title:        title,
			StartDate:    timezone.NewDate(2026, 4, day),
			EndDate:      timezone.NewDate(2026, 4, day),
			StartTime:    wallClock(9, 0),
			EndTime:      wallClock(10, 0),
			DeliveryMode: calModels.DeliveryModeInformational,
			// No send_email: the guardian opted out of mail for this appointment.
			Targets: []calendarSvc.AppointmentTarget{
				{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
			},
		})
		require.NoError(t, err)
		t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })
		return detail
	}

	// Cancelling a send_email=false appointment must not mail guardians.
	silent := newSilentAppointment("Stiller Termin", 1)
	require.Empty(t, outbox.enqueued)
	_, err := service.CancelStaffAppointment(calendarContext(organizerAccount.ID), silent.Appointment.ID)
	require.NoError(t, err)
	assert.Empty(t, outbox.enqueued, "a send_email=false appointment must stay silent on cancellation")

	// The repo Cancel reports the transition exactly once — the mechanism that lets
	// concurrent callers gate notifications so no duplicate cancellation e-mail is
	// sent.
	fresh := newSilentAppointment("Zweiter", 2)
	first, err := repos.CalendarAppointment.Cancel(testpkg.TenantContext(1), fresh.Appointment.ID)
	require.NoError(t, err)
	assert.True(t, first, "first cancel performs the transition")
	second, err := repos.CalendarAppointment.Cancel(testpkg.TenantContext(1), fresh.Appointment.ID)
	require.NoError(t, err)
	assert.False(t, second, "a second (concurrent) cancel does not re-transition, so it must not notify")
}

func TestCalendarServiceIntegration_CancelAfterDeleteDoesNotTransition(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	repos := repositories.NewFactory(db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "DeleteRace", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	// send_email=true, so a successful cancellation WOULD notify guardians.
	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Wird gelöscht",
		StartDate:    timezone.NewDate(2026, 5, 1),
		EndDate:      timezone.NewDate(2026, 5, 1),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		SendEmail:    true,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })

	// Simulate a delete landing after a cancel request loaded the still-live
	// appointment: soft-delete it directly.
	require.NoError(t, repos.CalendarAppointment.SoftDelete(testpkg.TenantContext(1), detail.Appointment.ID))

	// The cancel transition must now match zero rows (deleted_at guard) and report
	// no transition, so CancelStaffAppointment skips the guardian notice.
	transitioned, err := repos.CalendarAppointment.Cancel(testpkg.TenantContext(1), detail.Appointment.ID)
	require.NoError(t, err)
	assert.False(t, transitioned, "cancel must not transition an appointment a concurrent delete removed")

	after, err := repos.CalendarAppointment.FindByID(testpkg.TenantContext(1), detail.Appointment.ID)
	require.NoError(t, err)
	require.NotNil(t, after)
	assert.NotNil(t, after.DeletedAt, "the appointment stays deleted")
	assert.Nil(t, after.CancelledAt, "and was not silently flipped to cancelled")
}

func TestCalendarServiceIntegration_AppointmentICS(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "ICS", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	endsOn := timezone.NewDate(2026, 5, 25)
	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Wöchentliche AG",
		Location:     testpkg.StrPtr("Turnhalle"),
		StartDate:    timezone.NewDate(2026, 5, 4), // Monday
		EndDate:      timezone.NewDate(2026, 5, 4),
		StartTime:    wallClock(14, 0),
		EndTime:      wallClock(15, 30),
		DeliveryMode: calModels.DeliveryModeInformational,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     calModels.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"monday"},
			EndsOn:        &endsOn,
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })

	// Organizer can export.
	filename, content, err := service.StaffAppointmentICS(calendarContext(organizerAccount.ID), detail.Appointment.ID)
	require.NoError(t, err)
	assert.Equal(t, "Wchentliche-AG.ics", filename)
	assert.Contains(t, content, "BEGIN:VEVENT")
	assert.Contains(t, content, "SUMMARY:Wöchentliche AG")
	assert.Contains(t, content, "LOCATION:Turnhalle")
	assert.Contains(t, content, "DTSTART;TZID=Europe/Berlin:20260504T140000")
	assert.Contains(t, content, "RRULE:FREQ=WEEKLY;BYDAY=MO;UNTIL=20260525T")
	assert.NotContains(t, content, "EXDATE", "no exclusions before any occurrence is cancelled")

	// Cancelling a single occurrence emits an EXDATE so subscribed calendars
	// drop that date (matching the in-app view).
	require.NoError(t, service.CancelStaffAppointmentOccurrence(
		calendarContext(organizerAccount.ID), detail.Appointment.ID, timezone.NewDate(2026, 5, 11)))
	_, contentAfter, err := service.StaffAppointmentICS(calendarContext(organizerAccount.ID), detail.Appointment.ID)
	require.NoError(t, err)
	assert.Contains(t, contentAfter, "EXDATE;TZID=Europe/Berlin:20260511T140000")

	// The invited guardian can export, and also sees the exclusion.
	_, parentContent, err := service.ParentAppointmentICS(testpkg.TenantContext(1), parentChain.AccountID, detail.Appointment.ID)
	require.NoError(t, err)
	assert.Contains(t, parentContent, "SUMMARY:Wöchentliche AG")
	assert.Contains(t, parentContent, "EXDATE;TZID=Europe/Berlin:20260511T140000")

	// A stranger staff member cannot.
	other, otherAccount := testpkg.CreateTestCalendarStaff(t, db, "ICS", "Stranger")
	t.Cleanup(func() {
		testpkg.CleanupStaffFixtures(t, db, other.ID)
		testpkg.CleanupAuthFixtures(t, db, otherAccount.ID)
	})
	_, _, err = service.StaffAppointmentICS(calendarContext(otherAccount.ID), detail.Appointment.ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrNotFound))
}

func TestCalendarServiceIntegration_SubscriptionFeed(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	cfg := calendarTestConfig(db)
	repos := repositories.NewFactory(db)
	cfg.AccountRepo = repos.Account
	cfg.ParentsURL = "https://parents.test"
	service := calendarSvc.NewService(cfg)

	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Feed", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Sommerfest",
		StartDate:    timezone.TodayDate().AddDays(7),
		EndDate:      timezone.TodayDate().AddDays(7),
		StartTime:    wallClock(15, 0),
		EndTime:      wallClock(18, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })

	// First request generates the token and returns the subscription URLs once.
	httpsURL, webcalURL, err := service.ParentCalendarFeedURL(testpkg.TenantContext(1), parentChain.AccountID)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(httpsURL, "https://parents.test/api/calendar-feed/"))
	assert.True(t, strings.HasPrefix(webcalURL, "webcal://parents.test/api/calendar-feed/"))

	// Show-once: only the token hash is stored, so a second request cannot
	// re-display the raw URL — it returns empty (the UI then offers a rotate).
	httpsURL2, webcalURL2, err := service.ParentCalendarFeedURL(testpkg.TenantContext(1), parentChain.AccountID)
	require.NoError(t, err)
	assert.Empty(t, httpsURL2)
	assert.Empty(t, webcalURL2)

	token := strings.TrimPrefix(httpsURL, "https://parents.test/api/calendar-feed/")

	// Security: the DB stores only the SHA-256 hash of the token, never the raw
	// capability token — a DB read/backup exposes no replayable feed URL.
	var storedFeedToken string
	require.NoError(t, db.NewSelect().
		Table("auth.accounts").
		Column("calendar_feed_token").
		Where("id = ?", parentChain.AccountID).
		Scan(context.Background(), &storedFeedToken))
	rawSum := sha256.Sum256([]byte(token))
	assert.Equal(t, hex.EncodeToString(rawSum[:]), storedFeedToken, "only the token hash is persisted")
	assert.NotEqual(t, token, storedFeedToken, "the raw token must never be stored")

	filename, content, err := service.ParentCalendarFeedByToken(testpkg.TenantContext(1), token)
	require.NoError(t, err)
	assert.Equal(t, "moto-kalender.ics", filename)
	assert.Contains(t, content, "BEGIN:VCALENDAR")
	assert.Contains(t, content, "SUMMARY:Sommerfest")
	assert.NotContains(t, content, "EXDATE")

	// A recurring appointment with a cancelled single occurrence emits an EXDATE
	// in the feed, so subscribers drop that date.
	recurStart := timezone.TodayDate().AddDays(7)
	recurEnd := recurStart.AddDays(28)
	recurring, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Wöchentliches Treffen",
		StartDate:    recurStart,
		EndDate:      recurStart,
		StartTime:    wallClock(16, 0),
		EndTime:      wallClock(17, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     calModels.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{strings.ToLower(recurStart.Weekday().String())},
			EndsOn:        &recurEnd,
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, recurring.Appointment.ID) })
	require.NoError(t, service.CancelStaffAppointmentOccurrence(
		calendarContext(organizerAccount.ID), recurring.Appointment.ID, recurStart.AddDays(7)))

	_, contentWithExdate, err := service.ParentCalendarFeedByToken(testpkg.TenantContext(1), token)
	require.NoError(t, err)
	assert.Contains(t, contentWithExdate, "SUMMARY:Wöchentliches Treffen")
	assert.Contains(t, contentWithExdate, "EXDATE;TZID=Europe/Berlin:")

	// An unknown token is a plain not-found.
	_, _, err = service.ParentCalendarFeedByToken(testpkg.TenantContext(1), "nope-not-a-real-token")
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrNotFound))

	// Rotation invalidates the old token.
	httpsURL3, _, err := service.RotateParentCalendarFeed(testpkg.TenantContext(1), parentChain.AccountID)
	require.NoError(t, err)
	assert.NotEqual(t, httpsURL, httpsURL3)
	_, _, err = service.ParentCalendarFeedByToken(testpkg.TenantContext(1), token)
	require.Error(t, err)
}

func TestCalendarServiceIntegration_DeleteFeedVisibleLeavesTombstone(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	cfg := calendarTestConfig(db)
	repos := repositories.NewFactory(db)
	cfg.AccountRepo = repos.Account
	cfg.ParentsURL = "https://parents.test"
	service := calendarSvc.NewService(cfg)

	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Tombstone", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Elternabend",
		StartDate:    timezone.TodayDate().AddDays(7),
		EndDate:      timezone.TodayDate().AddDays(7),
		StartTime:    wallClock(18, 0),
		EndTime:      wallClock(19, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	// The appointment is tombstoned, not hard-deleted, so clean it up explicitly.
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })

	httpsURL, _, err := service.ParentCalendarFeedURL(testpkg.TenantContext(1), parentChain.AccountID)
	require.NoError(t, err)
	token := strings.TrimPrefix(httpsURL, "https://parents.test/api/calendar-feed/")

	// The interactive calendar window is capped (maxCalendarWindowDays); use a
	// small one that still spans the appointment (today + 7).
	viewFrom := timezone.TodayDate()
	viewTo := timezone.TodayDate().AddDays(30)

	// Before deletion the appointment is a normal (non-cancelled) feed event and
	// shows up in the parent's interactive calendar.
	_, content, err := service.ParentCalendarFeedByToken(testpkg.TenantContext(1), token)
	require.NoError(t, err)
	assert.Contains(t, content, "SUMMARY:Elternabend")
	assert.NotContains(t, content, "STATUS:CANCELLED")

	before, err := service.ListMyParentEvents(testpkg.TenantContext(1), parentChain.AccountID, viewFrom, viewTo)
	require.NoError(t, err)
	assert.NotEmpty(t, eventDates(before, calModels.EventSourceAppointment))

	require.NoError(t, service.DeleteStaffAppointment(calendarContext(organizerAccount.ID), detail.Appointment.ID))

	// The subscription feed keeps re-exporting the same UID as a durable
	// STATUS:CANCELLED tombstone with a bumped SEQUENCE, so subscribers purge it
	// instead of retaining a stale event.
	_, afterDelete, err := service.ParentCalendarFeedByToken(testpkg.TenantContext(1), token)
	require.NoError(t, err)
	assert.Contains(t, afterDelete, "SUMMARY:Elternabend")
	assert.Contains(t, afterDelete, "STATUS:CANCELLED")
	assert.Contains(t, afterDelete, "SEQUENCE:")

	// But the delete is a real delete for every interactive surface: the organizer
	// can no longer open it, and it is gone from the parent's calendar.
	_, err = service.GetStaffAppointmentDetail(calendarContext(organizerAccount.ID), detail.Appointment.ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrNotFound))

	afterParent, err := service.ListMyParentEvents(testpkg.TenantContext(1), parentChain.AccountID, viewFrom, viewTo)
	require.NoError(t, err)
	assert.Empty(t, eventDates(afterParent, calModels.EventSourceAppointment))
}

func TestCalendarServiceIntegration_EditRacingCancellationConflicts(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	repos := repositories.NewFactory(db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Reactivate", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "Reactivate", "Teacher")
	t.Cleanup(func() {
		testpkg.CleanupStaffFixtures(t, db, invitedStaff.ID, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, invitedAccount.ID, organizerAccount.ID)
	})

	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Original",
		StartDate:    timezone.NewDate(2026, 3, 2),
		EndDate:      timezone.NewDate(2026, 3, 2),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: calModels.DeliveryModeRSVPRequired,
		Targets:      []calendarSvc.AppointmentTarget{{Type: calModels.TargetTypeStaff, ID: &invitedStaff.ID}},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })
	// The snapshot returned by create carries CancelledAt == nil — it stands in for
	// a content edit loaded BEFORE the cancellation below.
	require.Nil(t, detail.Appointment.CancelledAt)

	// Cancel it (persists cancelled_at via the dedicated conditional update).
	_, err = service.CancelStaffAppointment(calendarContext(organizerAccount.ID), detail.Appointment.ID)
	require.NoError(t, err)

	// Flushing the stale edit must NOT touch the terminal row: the conditional
	// Update matches zero rows and reports a lifecycle conflict, so the edit
	// neither reactivates the appointment nor overwrites its content.
	stale := *detail.Appointment
	stale.Title = "Edited concurrently"
	err = repos.CalendarAppointment.Update(testpkg.TenantContext(1), &stale)
	require.Error(t, err)
	assert.ErrorIs(t, err, calModels.ErrAppointmentLifecycleConflict)

	after, err := repos.CalendarAppointment.FindByID(testpkg.TenantContext(1), detail.Appointment.ID)
	require.NoError(t, err)
	require.NotNil(t, after)
	assert.NotNil(t, after.CancelledAt, "the cancellation stands")
	assert.Equal(t, "Original", after.Title, "the racing edit did not apply")
}

func TestCalendarServiceIntegration_CancelledTombstoneSurvivesLookbackWindow(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	cfg := calendarTestConfig(db)
	repos := repositories.NewFactory(db)
	cfg.AccountRepo = repos.Account
	cfg.ParentsURL = "https://parents.test"
	service := calendarSvc.NewService(cfg)

	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "OldCancel", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	// A guardian-facing appointment whose date is already far beyond the feed's
	// 30-day lookback, so the live feed query never returns it.
	pastDate := timezone.TodayDate().AddDays(-60)
	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Vergangener Termin",
		StartDate:    pastDate,
		EndDate:      pastDate,
		StartTime:    wallClock(15, 0),
		EndTime:      wallClock(16, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		Targets:      []calendarSvc.AppointmentTarget{{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID}},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })

	httpsURL, _, err := service.ParentCalendarFeedURL(testpkg.TenantContext(1), parentChain.AccountID)
	require.NoError(t, err)
	token := strings.TrimPrefix(httpsURL, "https://parents.test/api/calendar-feed/")

	// Before cancellation: outside the lookback, so absent from the feed.
	_, content, err := service.ParentCalendarFeedByToken(testpkg.TenantContext(1), token)
	require.NoError(t, err)
	assert.NotContains(t, content, "SUMMARY:Vergangener Termin")

	// Cancel it. Even though its date aged out of the lookback, the feed must
	// re-export it as a durable STATUS:CANCELLED tombstone (retained by cancel
	// time), so a subscriber who was offline still receives the cancellation.
	_, err = service.CancelStaffAppointment(calendarContext(organizerAccount.ID), detail.Appointment.ID)
	require.NoError(t, err)

	_, afterCancel, err := service.ParentCalendarFeedByToken(testpkg.TenantContext(1), token)
	require.NoError(t, err)
	assert.Contains(t, afterCancel, "SUMMARY:Vergangener Termin")
	assert.Contains(t, afterCancel, "STATUS:CANCELLED")
}

func TestCalendarServiceIntegration_AllSchoolParentsTarget(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "School", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Schulfest",
		StartDate:    timezone.NewDate(2026, 6, 20),
		EndDate:      timezone.NewDate(2026, 6, 20),
		StartTime:    wallClock(10, 0),
		EndTime:      wallClock(14, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeAllSchoolParents},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })

	// The whole-school target resolved to the seeded guardian.
	require.Len(t, detail.Targets, 1)
	assert.Equal(t, calModels.TargetTypeAllSchoolParents, detail.Targets[0].TargetType)
	id := findOptionalRecipientByGuardian(detail, parentChain.GuardianProfileID)
	assert.NotZero(t, id, "all_school_parents should reach the school's guardian")

	// The parent sees it in their calendar.
	events, err := service.ListMyParentEvents(
		testpkg.TenantContext(1),
		parentChain.AccountID,
		timezone.NewDate(2026, 6, 20),
		timezone.NewDate(2026, 6, 20),
	)
	require.NoError(t, err)
	require.NotEmpty(t, events)
	assert.Equal(t, "Schulfest", events[0].Title)
}

func TestCalendarServiceIntegration_AllSchoolParentsExcludesInactiveStudents(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "SchoolActive", "Organizer")
	activeChain := testpkg.CreateTestParentGuardianChain(t, db)
	formerChain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, activeChain)
		testpkg.CleanupParentGuardianChain(t, db, formerChain)
		testpkg.CleanupStaffFixtures(t, db, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	// Mark the second family's child as a former (inactive) student.
	_, err := db.NewUpdate().
		Table("users.students").
		Set("status = ?", string(userModels.StudentStatusInactive)).
		Where("id = ?", formerChain.StudentID).
		Exec(context.Background())
	require.NoError(t, err)

	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Schulfest",
		StartDate:    timezone.NewDate(2026, 6, 20),
		EndDate:      timezone.NewDate(2026, 6, 20),
		StartTime:    wallClock(10, 0),
		EndTime:      wallClock(14, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeAllSchoolParents},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })

	// The active family's guardian is reached; the former family's is not.
	assert.NotZero(t, findOptionalRecipientByGuardian(detail, activeChain.GuardianProfileID),
		"an active student's guardian must receive the whole-school appointment")
	assert.Zero(t, findOptionalRecipientByGuardian(detail, formerChain.GuardianProfileID),
		"a former (inactive) student's guardian must NOT receive it")
}

func TestCalendarServiceIntegration_CancelSingleOccurrence(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Occurrence", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "Occurrence", "Invitee")
	t.Cleanup(func() {
		testpkg.CleanupStaffFixtures(t, db, invitedStaff.ID, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, invitedAccount.ID, organizerAccount.ID)
	})

	endsOn := timezone.NewDate(2026, 1, 19)
	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Weekly standup",
		StartDate:    timezone.NewDate(2026, 1, 5), // Monday
		EndDate:      timezone.NewDate(2026, 1, 5),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(9, 30),
		DeliveryMode: calModels.DeliveryModeRSVPRequired,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     calModels.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"monday"},
			EndsOn:        &endsOn,
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })

	events, err := service.ListMyStaffEvents(calendarContext(invitedAccount.ID), timezone.NewDate(2026, 1, 5), timezone.NewDate(2026, 1, 19))
	require.NoError(t, err)
	assert.Equal(t, []string{"2026-01-05", "2026-01-12", "2026-01-19"}, eventDates(events, calModels.EventSourceAppointment))

	// Cancel the middle occurrence.
	require.NoError(t, service.CancelStaffAppointmentOccurrence(calendarContext(organizerAccount.ID), detail.Appointment.ID, timezone.NewDate(2026, 1, 12)))
	events, err = service.ListMyStaffEvents(calendarContext(invitedAccount.ID), timezone.NewDate(2026, 1, 5), timezone.NewDate(2026, 1, 19))
	require.NoError(t, err)
	assert.Equal(t, []string{"2026-01-05", "2026-01-19"}, eventDates(events, calModels.EventSourceAppointment))

	// Cancelling the same occurrence again is idempotent.
	require.NoError(t, service.CancelStaffAppointmentOccurrence(calendarContext(organizerAccount.ID), detail.Appointment.ID, timezone.NewDate(2026, 1, 12)))
}

func TestCalendarServiceIntegration_AttendeeOverviewVisibility(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Overview", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "Overview", "Teacher")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, invitedStaff.ID, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, invitedAccount.ID, organizerAccount.ID)
	})

	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:              "Overview visible to all",
		StartDate:          timezone.NewDate(2026, 2, 10),
		EndDate:            timezone.NewDate(2026, 2, 10),
		StartTime:          wallClock(9, 0),
		EndTime:            wallClock(10, 0),
		DeliveryMode:       calModels.DeliveryModeRSVPRequired,
		OverviewVisibility: calModels.OverviewVisibilityAll,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeStaff, ID: &invitedStaff.ID},
			{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })

	staffRecipientID := findRecipientByStaff(t, detail, invitedStaff.ID)
	require.NoError(t, service.RespondToStaffInvitation(calendarContext(invitedAccount.ID), staffRecipientID, calModels.ResponseStatusAccepted))

	staffOverview, err := service.GetStaffAppointmentOverview(calendarContext(invitedAccount.ID), detail.Appointment.ID)
	require.NoError(t, err)
	require.NotNil(t, staffOverview)
	assert.Equal(t, calModels.OverviewVisibilityAll, staffOverview.OverviewVisibility)
	assert.Equal(t, calModels.DeliveryModeRSVPRequired, staffOverview.DeliveryMode)
	require.Len(t, staffOverview.Attendees, 2)
	assert.Equal(t, calModels.RecipientTypeGuardianProfile, staffOverview.Attendees[0].RecipientType)
	assert.Contains(t, staffOverview.Attendees[0].Name, "Schneider")
	assert.Equal(t, calModels.ResponseStatusPending, staffOverview.Attendees[0].Status)
	assert.Equal(t, calModels.RecipientTypeStaff, staffOverview.Attendees[1].RecipientType)
	assert.Contains(t, staffOverview.Attendees[1].Name, "Teacher")
	assert.Equal(t, calModels.ResponseStatusAccepted, staffOverview.Attendees[1].Status)
	require.NotNil(t, staffOverview.Attendees[1].RespondedAt)

	parentOverview, err := service.GetParentAppointmentOverview(testpkg.TenantContext(1), parentChain.AccountID, detail.Appointment.ID)
	require.NoError(t, err)
	require.Len(t, parentOverview.Attendees, 2)

	staffEvents, err := service.ListMyStaffEvents(calendarContext(invitedAccount.ID), timezone.NewDate(2026, 2, 10), timezone.NewDate(2026, 2, 10))
	require.NoError(t, err)
	require.NotEmpty(t, staffEvents)
	assert.True(t, staffEvents[0].CanViewOverview)

	parentEvents, err := service.ListMyParentEvents(testpkg.TenantContext(1), parentChain.AccountID, timezone.NewDate(2026, 2, 10), timezone.NewDate(2026, 2, 10))
	require.NoError(t, err)
	require.NotEmpty(t, parentEvents)
	assert.True(t, parentEvents[0].CanViewOverview)
}

func TestCalendarServiceIntegration_DeletedAppointmentUnreachableViaOverviewAndRSVP(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Deleted", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "Deleted", "Teacher")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, invitedStaff.ID, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, invitedAccount.ID, organizerAccount.ID)
	})

	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:              "Wird gelöscht",
		StartDate:          timezone.NewDate(2026, 2, 10),
		EndDate:            timezone.NewDate(2026, 2, 10),
		StartTime:          wallClock(9, 0),
		EndTime:            wallClock(10, 0),
		DeliveryMode:       calModels.DeliveryModeRSVPRequired,
		OverviewVisibility: calModels.OverviewVisibilityAll,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeStaff, ID: &invitedStaff.ID},
			{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	// Soft-deleted, so it must be cleaned up explicitly.
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })

	staffRecipientID := findRecipientByStaff(t, detail, invitedStaff.ID)
	guardianRecipientID := findRecipientByGuardian(t, detail, parentChain.GuardianProfileID)

	// Sanity: while live, overview and RSVP are reachable.
	_, err = service.GetStaffAppointmentOverview(calendarContext(invitedAccount.ID), detail.Appointment.ID)
	require.NoError(t, err)
	require.NoError(t, service.RespondToStaffInvitation(calendarContext(invitedAccount.ID), staffRecipientID, calModels.ResponseStatusAccepted))

	// It targets a guardian, so delete tombstones (soft-deletes) it.
	require.NoError(t, service.DeleteStaffAppointment(calendarContext(organizerAccount.ID), detail.Appointment.ID))

	// Every interactive detail/overview/RSVP path — staff and parent — must now
	// treat the retained appointment/recipient IDs as not found.
	_, err = service.GetStaffAppointmentDetail(calendarContext(organizerAccount.ID), detail.Appointment.ID)
	assert.ErrorIs(t, err, calendarSvc.ErrNotFound)

	_, err = service.GetStaffAppointmentOverview(calendarContext(organizerAccount.ID), detail.Appointment.ID)
	assert.ErrorIs(t, err, calendarSvc.ErrNotFound)
	_, err = service.GetStaffAppointmentOverview(calendarContext(invitedAccount.ID), detail.Appointment.ID)
	assert.ErrorIs(t, err, calendarSvc.ErrNotFound)
	_, err = service.GetParentAppointmentOverview(testpkg.TenantContext(1), parentChain.AccountID, detail.Appointment.ID)
	assert.ErrorIs(t, err, calendarSvc.ErrNotFound)

	err = service.RespondToStaffInvitation(calendarContext(invitedAccount.ID), staffRecipientID, calModels.ResponseStatusDeclined)
	assert.ErrorIs(t, err, calendarSvc.ErrNotFound)
	err = service.RespondToParentInvitation(testpkg.TenantContext(1), parentChain.AccountID, guardianRecipientID, calModels.ResponseStatusDeclined)
	assert.ErrorIs(t, err, calendarSvc.ErrNotFound)
}

func TestCalendarServiceIntegration_AttendeeOverviewAccessRules(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Private", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "Private", "Invitee")
	outsider, outsiderAccount := testpkg.CreateTestCalendarStaff(t, db, "Private", "Outsider")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, outsider.ID, invitedStaff.ID, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, outsiderAccount.ID, invitedAccount.ID, organizerAccount.ID)
	})

	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:              "Staff-only overview",
		StartDate:          timezone.NewDate(2026, 2, 11),
		EndDate:            timezone.NewDate(2026, 2, 11),
		StartTime:          wallClock(11, 0),
		EndTime:            wallClock(12, 0),
		DeliveryMode:       calModels.DeliveryModeInformational,
		OverviewVisibility: calModels.OverviewVisibilityStaff,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeStaff, ID: &invitedStaff.ID},
			{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })

	organizerOverview, err := service.GetStaffAppointmentOverview(calendarContext(organizerAccount.ID), detail.Appointment.ID)
	require.NoError(t, err)
	require.Len(t, organizerOverview.Attendees, 2)
	for _, attendee := range organizerOverview.Attendees {
		assert.Equal(t, calModels.ResponseStatusInfo, attendee.Status)
	}

	_, err = service.GetStaffAppointmentOverview(calendarContext(invitedAccount.ID), detail.Appointment.ID)
	require.NoError(t, err)

	_, err = service.GetParentAppointmentOverview(testpkg.TenantContext(1), parentChain.AccountID, detail.Appointment.ID)
	assert.True(t, errors.Is(err, calendarSvc.ErrForbidden))

	_, err = service.GetStaffAppointmentOverview(calendarContext(outsiderAccount.ID), detail.Appointment.ID)
	assert.True(t, errors.Is(err, calendarSvc.ErrNotFound))

	parentEvents, err := service.ListMyParentEvents(testpkg.TenantContext(1), parentChain.AccountID, timezone.NewDate(2026, 2, 11), timezone.NewDate(2026, 2, 11))
	require.NoError(t, err)
	require.NotEmpty(t, parentEvents)
	assert.False(t, parentEvents[0].CanViewOverview)
}

func TestCalendarServiceIntegration_RecipientOptionsAndGroupedTargets(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Target", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "Target", "Invitee")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	inactiveParentChain := testpkg.CreateTestParentGuardianChain(t, db)
	_, err := db.ExecContext(
		context.Background(),
		`UPDATE auth.account_tenants SET status = ? WHERE account_id = ? AND tenant_id = ?`,
		authModels.AccountTenantStatusInactive,
		inactiveParentChain.AccountID,
		inactiveParentChain.TenantID,
	)
	require.NoError(t, err)
	invisibleGuardian := &userModels.GuardianProfile{
		FirstName:              "Hidden",
		LastName:               "Target",
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	invisibleGuardian.SetTenantID(1)
	_, err = db.NewInsert().Model(invisibleGuardian).ModelTableExpr(`users.guardian_profiles`).Exec(context.Background())
	require.NoError(t, err)
	accountlessGuardian := &userModels.GuardianProfile{
		FirstName:              "Accountless",
		LastName:               "Target",
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	accountlessGuardian.SetTenantID(1)
	_, err = db.NewInsert().Model(accountlessGuardian).ModelTableExpr(`users.guardian_profiles`).Exec(context.Background())
	require.NoError(t, err)
	accountlessLink := &userModels.StudentGuardian{
		StudentID:         parentChain.StudentID,
		GuardianProfileID: accountlessGuardian.ID,
		RelationshipType:  "parent",
	}
	authorize.ApplyStudentGuardianRole(accountlessLink, authorize.GuardianRoleLegalGuardian)
	accountlessLink.SetTenantID(1)
	_, err = db.NewInsert().Model(accountlessLink).ModelTableExpr(`users.students_guardians`).Exec(context.Background())
	require.NoError(t, err)

	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "users.students_guardians", accountlessLink.ID)
		testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", accountlessGuardian.ID)
		testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", invisibleGuardian.ID)
		testpkg.CleanupParentGuardianChain(t, db, inactiveParentChain)
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, invitedStaff.ID, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, invitedAccount.ID, organizerAccount.ID)
	})

	options, err := service.RecipientOptions(calendarContext(organizerAccount.ID), "target", 20)
	require.NoError(t, err)
	assert.NotEmpty(t, options.Staff)
	assert.NotNil(t, options.Parents)
	assert.NotNil(t, options.Groups)
	assert.NotNil(t, options.Classes)
	assert.NotNil(t, options.Students)
	assert.NotContains(t, parentOptionIDs(options.Parents), strconv.FormatInt(invisibleGuardian.ID, 10))
	assert.NotContains(t, parentOptionIDs(options.Parents), strconv.FormatInt(inactiveParentChain.GuardianProfileID, 10))
	assert.NotContains(t, parentOptionIDs(options.Parents), strconv.FormatInt(accountlessGuardian.ID, 10))

	schneiderOptions, err := service.RecipientOptions(calendarContext(organizerAccount.ID), "schneider", 20)
	require.NoError(t, err)
	assert.Contains(t, parentOptionIDs(schneiderOptions.Parents), strconv.FormatInt(parentChain.GuardianProfileID, 10))
	assert.NotContains(t, parentOptionIDs(schneiderOptions.Parents), strconv.FormatInt(inactiveParentChain.GuardianProfileID, 10))

	hiddenOptions, err := service.RecipientOptions(calendarContext(organizerAccount.ID), "hidden", 20)
	require.NoError(t, err)
	assert.NotContains(t, parentOptionIDs(hiddenOptions.Parents), strconv.FormatInt(invisibleGuardian.ID, 10))

	childOptions, err := service.RecipientOptions(calendarContext(organizerAccount.ID), "felix", 20)
	require.NoError(t, err)
	require.NotEmpty(t, childOptions.Students)
	assert.Contains(t, childOptions.Students[0].Name, "Felix")

	limitedOptions, err := service.RecipientOptions(calendarContext(organizerAccount.ID), "", 1)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(limitedOptions.Staff), 1)
	assert.LessOrEqual(t, len(limitedOptions.Groups), 1)
	assert.LessOrEqual(t, len(limitedOptions.Classes), 1)
	assert.LessOrEqual(t, len(limitedOptions.Students), 1)

	defaultedOptions, err := service.RecipientOptions(calendarContext(organizerAccount.ID), "target", 999)
	require.NoError(t, err)
	assert.NotNil(t, defaultedOptions.Staff)

	className := "1a"
	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:              "Grouped target coverage",
		StartDate:          timezone.NewDate(2026, 2, 12),
		EndDate:            timezone.NewDate(2026, 2, 12),
		StartTime:          wallClock(13, 0),
		EndTime:            wallClock(14, 0),
		DeliveryMode:       calModels.DeliveryModeInformational,
		OverviewVisibility: calModels.OverviewVisibilityAll,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeAllStaff},
			{Type: calModels.TargetTypeParentsByClass, Value: &className},
			{Type: calModels.TargetTypeParentsByStudent, ID: &parentChain.StudentID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })

	require.NotEmpty(t, detail.Recipients)
	assert.NotZero(t, findRecipientByStaff(t, detail, organizer.ID))
	assert.NotZero(t, findRecipientByStaff(t, detail, invitedStaff.ID))
	assert.NotZero(t, findRecipientByGuardian(t, detail, parentChain.GuardianProfileID))
	assert.Zero(t, findOptionalRecipientByGuardian(detail, inactiveParentChain.GuardianProfileID))
	assert.Zero(t, findOptionalRecipientByGuardian(detail, accountlessGuardian.ID))
	require.Len(t, detail.Targets, 3)

	parentEvents, err := service.ListMyParentEvents(testpkg.TenantContext(1), parentChain.AccountID, timezone.NewDate(2026, 2, 12), timezone.NewDate(2026, 2, 12))
	require.NoError(t, err)
	require.NotEmpty(t, parentEvents)
	assert.True(t, parentEvents[0].CanViewOverview)
	assert.False(t, parentEvents[0].CanRespond)
	require.NotNil(t, parentEvents[0].ResponseStatus)
	assert.Equal(t, calModels.ResponseStatusInfo, *parentEvents[0].ResponseStatus)
}

func parentOptionIDs(options []calendarSvc.ParentOption) []string {
	ids := make([]string, 0, len(options))
	for _, option := range options {
		ids = append(ids, option.ID)
	}
	return ids
}

func TestCalendarServiceIntegration_InvalidCreateTargets(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Invalid", "Targets")
	const foreignTenantID int64 = 2
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	otherTenantStaff := testpkg.CreateTestStaffForTenant(t, db, foreignTenantID, "Other", "Tenant")
	invisibleGuardian := &userModels.GuardianProfile{
		FirstName:              "No",
		LastName:               "Portal",
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	invisibleGuardian.SetTenantID(1)
	_, err := db.NewInsert().Model(invisibleGuardian).ModelTableExpr(`users.guardian_profiles`).Exec(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", invisibleGuardian.ID)
		testpkg.CleanupStaffFixtures(t, db, otherTenantStaff.ID, organizer.ID)
		testpkg.CleanupTenantTestData(t, db, foreignTenantID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM platform.schools WHERE id = ?`, foreignTenantID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM platform.organizations WHERE id = ?`, foreignTenantID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	className := " "
	invalidID := int64(0)
	missingID := int64(999999999)
	tests := []struct {
		name   string
		target calendarSvc.AppointmentTarget
	}{
		{name: "staff target missing id", target: calendarSvc.AppointmentTarget{Type: calModels.TargetTypeStaff}},
		{name: "staff target zero id", target: calendarSvc.AppointmentTarget{Type: calModels.TargetTypeStaff, ID: &invalidID}},
		{name: "staff target other tenant", target: calendarSvc.AppointmentTarget{Type: calModels.TargetTypeStaff, ID: &otherTenantStaff.ID}},
		{name: "guardian target missing id", target: calendarSvc.AppointmentTarget{Type: calModels.TargetTypeGuardianProfile}},
		{name: "guardian target missing row", target: calendarSvc.AppointmentTarget{Type: calModels.TargetTypeGuardianProfile, ID: &missingID}},
		{name: "guardian target without portal-visible student", target: calendarSvc.AppointmentTarget{Type: calModels.TargetTypeGuardianProfile, ID: &invisibleGuardian.ID}},
		{name: "student target missing id", target: calendarSvc.AppointmentTarget{Type: calModels.TargetTypeParentsByStudent}},
		{name: "group target missing id", target: calendarSvc.AppointmentTarget{Type: calModels.TargetTypeParentsByGroup}},
		{name: "class target blank value", target: calendarSvc.AppointmentTarget{Type: calModels.TargetTypeParentsByClass, Value: &className}},
		{name: "unknown target type", target: calendarSvc.AppointmentTarget{Type: "unknown_target"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
				Title:        "Invalid target",
				StartDate:    timezone.NewDate(2026, 2, 14),
				EndDate:      timezone.NewDate(2026, 2, 14),
				StartTime:    wallClock(8, 0),
				EndTime:      wallClock(9, 0),
				DeliveryMode: calModels.DeliveryModeRSVPRequired,
				Targets:      []calendarSvc.AppointmentTarget{tt.target},
			})
			require.Error(t, err)
			assert.True(t, errors.Is(err, calendarSvc.ErrInvalidRequest))
		})
	}
}

func TestCalendarServiceIntegration_InvalidRecurrenceDoesNotPersistAppointment(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Invalid", "Recurrence")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "Invalid", "RecurrenceInvitee")
	t.Cleanup(func() {
		testpkg.CleanupStaffFixtures(t, db, invitedStaff.ID, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, invitedAccount.ID, organizerAccount.ID)
	})

	title := "Invalid recurrence must not persist"
	_, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:     title,
		StartDate: timezone.NewDate(2026, 2, 20),
		EndDate:   timezone.NewDate(2026, 2, 20),
		StartTime: wallClock(8, 0),
		EndTime:   wallClock(9, 0),
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency: "hourly",
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrInvalidRequest))

	count, err := db.NewSelect().
		TableExpr(`calendar.appointments`).
		Where(`title = ?`, title).
		Count(context.Background())
	require.NoError(t, err)
	assert.Zero(t, count)
}

func TestCalendarServiceIntegration_MultiDayRecurrenceVisibleOnFinalOverlapDay(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	repos := repositories.NewFactory(db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "MultiDay", "Organizer")
	t.Cleanup(func() {
		testpkg.CleanupStaffFixtures(t, db, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	endsOn := timezone.NewDate(2026, 1, 31)
	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Overnight recurring appointment",
		StartDate:    timezone.NewDate(2026, 1, 31),
		EndDate:      timezone.NewDate(2026, 2, 1),
		StartTime:    wallClock(18, 0),
		EndTime:      wallClock(9, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency: calModels.RecurrenceFrequencyDaily,
			EndsOn:    &endsOn,
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeStaff, ID: &organizer.ID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })

	events, err := service.ListMyStaffEvents(
		calendarContext(organizerAccount.ID),
		timezone.NewDate(2026, 2, 1),
		timezone.NewDate(2026, 2, 1),
	)
	require.NoError(t, err)
	assert.Contains(t, eventDates(events, calModels.EventSourceAppointment), "2026-01-31")

	cancelledOverride := &calModels.AppointmentOccurrenceOverride{
		AppointmentID:  detail.Appointment.ID,
		OccurrenceDate: timezone.NewDate(2026, 1, 31),
		Cancelled:      true,
	}
	require.NoError(t, repos.CalendarOccurrenceOverride.Create(calendarContext(organizerAccount.ID), cancelledOverride))

	events, err = service.ListMyStaffEvents(
		calendarContext(organizerAccount.ID),
		timezone.NewDate(2026, 2, 1),
		timezone.NewDate(2026, 2, 1),
	)
	require.NoError(t, err)
	assert.NotContains(t, eventDates(events, calModels.EventSourceAppointment), "2026-01-31")
}

func TestCalendarServiceIntegration_ResponseAndOverviewErrors(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Errors", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "Errors", "Invitee")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, invitedStaff.ID, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, invitedAccount.ID, organizerAccount.ID)
	})

	rsvpDetail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Response errors",
		StartDate:    timezone.NewDate(2026, 2, 15),
		EndDate:      timezone.NewDate(2026, 2, 15),
		StartTime:    wallClock(10, 0),
		EndTime:      wallClock(11, 0),
		DeliveryMode: calModels.DeliveryModeRSVPRequired,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeStaff, ID: &invitedStaff.ID},
			{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, rsvpDetail.Appointment.ID) })

	staffRecipientID := findRecipientByStaff(t, rsvpDetail, invitedStaff.ID)
	guardianRecipientID := findRecipientByGuardian(t, rsvpDetail, parentChain.GuardianProfileID)

	err = service.RespondToStaffInvitation(calendarContext(invitedAccount.ID), staffRecipientID, calModels.ResponseStatusPending)
	assert.True(t, errors.Is(err, calendarSvc.ErrInvalidRequest))

	err = service.RespondToStaffInvitation(calendarContext(organizerAccount.ID), staffRecipientID, calModels.ResponseStatusAccepted)
	assert.True(t, errors.Is(err, calendarSvc.ErrNotFound))

	err = service.RespondToParentInvitation(testpkg.TenantContext(1), parentChain.AccountID, guardianRecipientID, calModels.ResponseStatusPending)
	assert.True(t, errors.Is(err, calendarSvc.ErrInvalidRequest))

	err = service.RespondToParentInvitation(testpkg.TenantContext(1), 0, guardianRecipientID, calModels.ResponseStatusAccepted)
	assert.True(t, errors.Is(err, calendarSvc.ErrForbidden))

	err = service.RespondToParentInvitation(testpkg.TenantContext(1), parentChain.AccountID, staffRecipientID, calModels.ResponseStatusAccepted)
	assert.True(t, errors.Is(err, calendarSvc.ErrNotFound))

	_, err = service.GetStaffAppointmentOverview(calendarContext(invitedAccount.ID), 999999999)
	assert.True(t, errors.Is(err, calendarSvc.ErrNotFound))

	_, err = service.GetParentAppointmentOverview(testpkg.TenantContext(1), parentChain.AccountID, 999999999)
	assert.True(t, errors.Is(err, calendarSvc.ErrNotFound))

	privateDetail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Organizer-only overview",
		StartDate:    timezone.NewDate(2026, 2, 16),
		EndDate:      timezone.NewDate(2026, 2, 16),
		StartTime:    wallClock(10, 0),
		EndTime:      wallClock(11, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, privateDetail.Appointment.ID) })

	_, err = service.GetStaffAppointmentOverview(calendarContext(invitedAccount.ID), privateDetail.Appointment.ID)
	assert.True(t, errors.Is(err, calendarSvc.ErrForbidden))

	infoDetail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Parent information only",
		StartDate:    timezone.NewDate(2026, 2, 17),
		EndDate:      timezone.NewDate(2026, 2, 17),
		StartTime:    wallClock(10, 0),
		EndTime:      wallClock(11, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, infoDetail.Appointment.ID) })

	infoGuardianRecipientID := findRecipientByGuardian(t, infoDetail, parentChain.GuardianProfileID)
	err = service.RespondToParentInvitation(testpkg.TenantContext(1), parentChain.AccountID, infoGuardianRecipientID, calModels.ResponseStatusAccepted)
	assert.True(t, errors.Is(err, calendarSvc.ErrInvalidRequest))
}

func TestCalendarServiceIntegration_RepositoryReadAndReplacePaths(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	repos := repositories.NewFactory(db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Repo", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "Repo", "Invitee")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, invitedStaff.ID, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, invitedAccount.ID, organizerAccount.ID)
	})

	endsOn := timezone.NewDate(2026, 2, 20)
	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Repository coverage",
		StartDate:    timezone.NewDate(2026, 2, 13),
		EndDate:      timezone.NewDate(2026, 2, 13),
		StartTime:    wallClock(15, 0),
		EndTime:      wallClock(16, 0),
		DeliveryMode: calModels.DeliveryModeRSVPRequired,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     calModels.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"friday"},
			EndsOn:        &endsOn,
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeStaff, ID: &invitedStaff.ID},
			{Type: calModels.TargetTypeParentsByStudent, ID: &parentChain.StudentID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })

	ctx := calendarContext(organizerAccount.ID)
	organized, err := repos.CalendarAppointment.ListOrganizedByStaff(ctx, organizer.ID, timezone.NewDate(2026, 2, 13), timezone.NewDate(2026, 2, 13))
	require.NoError(t, err)
	require.NotEmpty(t, organized)

	emptyGuardianAppointments, err := repos.CalendarAppointment.ListVisibleForGuardianProfiles(ctx, nil, nil, timezone.NewDate(2026, 2, 13), timezone.NewDate(2026, 2, 13))
	require.NoError(t, err)
	assert.Empty(t, emptyGuardianAppointments)

	recurrence, err := repos.CalendarRecurrenceRule.FindByAppointmentID(ctx, detail.Appointment.ID)
	require.NoError(t, err)
	require.NotNil(t, recurrence)
	assert.Equal(t, calModels.RecurrenceFrequencyWeekly, recurrence.Frequency)

	emptyRecurrences, err := repos.CalendarRecurrenceRule.FindByAppointmentIDs(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, emptyRecurrences)

	targets, err := repos.CalendarAppointmentTarget.FindByAppointmentID(ctx, detail.Appointment.ID)
	require.NoError(t, err)
	require.Len(t, targets, 2)

	recipients, err := repos.CalendarAppointmentRecipient.FindByAppointmentID(ctx, detail.Appointment.ID)
	require.NoError(t, err)
	recipientIDs := make([]int64, 0, len(recipients))
	for _, recipient := range recipients {
		recipientIDs = append(recipientIDs, recipient.ID)
	}
	links, err := repos.CalendarAppointmentRecipientChild.FindByRecipientIDs(ctx, recipientIDs)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, parentChain.StudentID, links[0].StudentID)

	emptyLinks, err := repos.CalendarAppointmentRecipientChild.FindByRecipientIDs(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, emptyLinks)

	staffRecipient := &calModels.AppointmentRecipient{
		RecipientType: calModels.RecipientTypeStaff,
		StaffID:       &organizer.ID,
		Status:        calModels.ResponseStatusPending,
	}
	require.NoError(t, repos.CalendarAppointmentRecipient.ReplaceForAppointment(ctx, detail.Appointment.ID, []*calModels.AppointmentRecipient{staffRecipient}))
	replacedRecipients, err := repos.CalendarAppointmentRecipient.FindByAppointmentID(ctx, detail.Appointment.ID)
	require.NoError(t, err)
	require.Len(t, replacedRecipients, 1)
	assert.Equal(t, organizer.ID, *replacedRecipients[0].StaffID)

	require.NoError(t, repos.CalendarAppointmentTarget.ReplaceForAppointment(ctx, detail.Appointment.ID, nil))
	targets, err = repos.CalendarAppointmentTarget.FindByAppointmentID(ctx, detail.Appointment.ID)
	require.NoError(t, err)
	assert.Empty(t, targets)

	require.NoError(t, repos.CalendarRecurrenceRule.DeleteByAppointmentID(ctx, detail.Appointment.ID))
	recurrence, err = repos.CalendarRecurrenceRule.FindByAppointmentID(ctx, detail.Appointment.ID)
	require.NoError(t, err)
	assert.Nil(t, recurrence)
}

func TestCalendarServiceIntegration_StaffCalendarIncludesAssignedTimetable(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	staff, account := testpkg.CreateTestCalendarStaff(t, db, "Timetable", "Staff")
	room := testpkg.CreateTestRoom(t, db, "Calendar Timetable Room")
	instance := testpkg.CreateTestActivityInstance(t, db, timezone.NewDate(2026, 3, 3), room.ID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:15",
		EndHHMM:   "15:45",
		Title:     "Assigned Betreuung",
	})
	assignment := testpkg.CreateTestInstanceStaff(t, db, instance.ID, staff.ID, testpkg.InstanceStaffOpts{IsPrimary: true})

	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.instance_staff", assignment.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", instance.ID)
		testpkg.CleanupActivityFixtures(t, db, room.ID)
		testpkg.CleanupStaffFixtures(t, db, staff.ID)
		testpkg.CleanupAuthFixtures(t, db, account.ID)
	})

	events, err := service.ListMyStaffEvents(calendarContext(account.ID), timezone.NewDate(2026, 3, 3), timezone.NewDate(2026, 3, 3))
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, calModels.EventSourceTimetable, events[0].Source)
	assert.Equal(t, "Assigned Betreuung", events[0].Title)
	assert.Equal(t, "14:15", events[0].StartTime)
	assert.Equal(t, "15:45", events[0].EndTime)
	assert.False(t, events[0].CanRespond)
}

func TestCalendarServiceIntegration_StaffCalendarIncludesShifts(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	staff, account := testpkg.CreateTestCalendarStaff(t, db, "Shift", "Staff")

	shiftType := &scheduleModels.ShiftType{Name: "Frühdienst", Color: "#F78C10", IsActive: true}
	shiftType.SetTenantID(1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.NewInsert().Model(shiftType).ModelTableExpr(`schedule.shift_types`).Exec(ctx)
	require.NoError(t, err)

	day := timezone.NewDate(2026, 5, 4)
	typed := testpkg.CreateTestStaffShift(t, db, staff.ID, day, testpkg.StaffShiftOpts{
		StartHHMM:   "08:00",
		EndHHMM:     "12:00",
		Notes:       "Vertretung Gruppe B",
		ShiftTypeID: &shiftType.ID,
	})
	untyped := testpkg.CreateTestStaffShift(t, db, staff.ID, day, testpkg.StaffShiftOpts{
		StartHHMM: "13:00",
		EndHHMM:   "16:00",
	})
	cancelled := testpkg.CreateTestStaffShift(t, db, staff.ID, day.AddDays(1), testpkg.StaffShiftOpts{
		Cancelled: true,
	})

	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.staff_shifts", typed.ID, untyped.ID, cancelled.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.shift_types", shiftType.ID)
		testpkg.CleanupStaffFixtures(t, db, staff.ID)
		testpkg.CleanupAuthFixtures(t, db, account.ID)
	})

	events, err := service.ListMyStaffEvents(calendarContext(account.ID), day, day.AddDays(1))
	require.NoError(t, err)
	require.Len(t, events, 2)

	assert.Equal(t, calModels.EventSourceShift, events[0].Source)
	assert.Equal(t, "Frühdienst", events[0].Title)
	assert.Equal(t, "08:00", events[0].StartTime)
	assert.Equal(t, "12:00", events[0].EndTime)
	require.NotNil(t, events[0].Description)
	assert.Equal(t, "Vertretung Gruppe B", *events[0].Description)
	assert.False(t, events[0].CanRespond)
	assert.False(t, events[0].CanEdit)

	assert.Equal(t, calModels.EventSourceShift, events[1].Source)
	assert.Equal(t, "Dienst", events[1].Title)
	assert.Equal(t, "13:00", events[1].StartTime)
	assert.Equal(t, "16:00", events[1].EndTime)
	assert.Nil(t, events[1].Description)
}

func TestCalendarServiceIntegration_ParentCalendarIncludesChildTimetable(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	room := testpkg.CreateTestRoom(t, db, "Parent Calendar Room")
	instance := testpkg.CreateTestActivityInstance(t, db, timezone.NewDate(2026, 4, 8), room.ID, testpkg.ActivityInstanceOpts{
		StartHHMM: "13:00",
		EndHHMM:   "16:00",
		Title:     "Child Betreuung",
	})
	studentLink := testpkg.CreateTestInstanceStudent(t, db, instance.ID, parentChain.StudentID, "")

	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.instance_students", studentLink.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", instance.ID)
		testpkg.CleanupActivityFixtures(t, db, room.ID)
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
	})

	events, err := service.ListMyParentEvents(testpkg.TenantContext(1), parentChain.AccountID, timezone.NewDate(2026, 4, 8), timezone.NewDate(2026, 4, 8))
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, calModels.EventSourceTimetable, events[0].Source)
	assert.Equal(t, "Child Betreuung", events[0].Title)
	assert.Equal(t, "13:00", events[0].StartTime)
	assert.Equal(t, "16:00", events[0].EndTime)
	require.NotNil(t, events[0].StudentID)
	assert.Equal(t, strconv.FormatInt(parentChain.StudentID, 10), *events[0].StudentID)
	require.NotNil(t, events[0].StudentName)
	assert.Contains(t, *events[0].StudentName, "Felix")
}

// The public subscription feed bypasses parent auth, so a deactivated account
// must lose feed access immediately — the token alone is not enough.
func TestCalendarServiceIntegration_FeedRejectsInactiveAccount(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	cfg := calendarTestConfig(db)
	repos := repositories.NewFactory(db)
	cfg.AccountRepo = repos.Account
	cfg.ParentsURL = "https://parents.test"
	service := calendarSvc.NewService(cfg)

	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Inactive", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Elterninfo",
		StartDate:    timezone.TodayDate().AddDays(3),
		EndDate:      timezone.TodayDate().AddDays(3),
		StartTime:    wallClock(15, 0),
		EndTime:      wallClock(16, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })

	httpsURL, _, err := service.ParentCalendarFeedURL(testpkg.TenantContext(1), parentChain.AccountID)
	require.NoError(t, err)
	token := strings.TrimPrefix(httpsURL, "https://parents.test/api/calendar-feed/")

	// While active, the feed serves the family's appointments.
	_, content, err := service.ParentCalendarFeedByToken(testpkg.TenantContext(1), token)
	require.NoError(t, err)
	assert.Contains(t, content, "SUMMARY:Elterninfo")

	// Deactivate the account (tenant mappings stay active).
	_, err = db.NewUpdate().
		ModelTableExpr("auth.accounts").
		Set("active = ?", false).
		Where("id = ?", parentChain.AccountID).
		Exec(context.Background())
	require.NoError(t, err)

	// The same token now behaves like an unknown token: a plain not-found, no leak.
	_, _, err = service.ParentCalendarFeedByToken(testpkg.TenantContext(1), token)
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrNotFound))
}

// Editing a recurring series is a whole-series operation: stale single-occurrence
// cancellations from the old cadence must not survive to suppress valid dates in
// the edited series.
func TestCalendarServiceIntegration_SeriesEditClearsOccurrenceOverrides(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "SeriesEdit", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "SeriesEdit", "Invitee")
	t.Cleanup(func() {
		testpkg.CleanupStaffFixtures(t, db, invitedStaff.ID, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, invitedAccount.ID, organizerAccount.ID)
	})

	endsOn := timezone.NewDate(2026, 1, 19)
	weekly := func() *calendarSvc.RecurrenceRequest {
		return &calendarSvc.RecurrenceRequest{
			Frequency:     calModels.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"monday"},
			EndsOn:        &endsOn,
		}
	}
	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Weekly standup",
		StartDate:    timezone.NewDate(2026, 1, 5), // Monday
		EndDate:      timezone.NewDate(2026, 1, 5),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(9, 30),
		DeliveryMode: calModels.DeliveryModeRSVPRequired,
		Recurrence:   weekly(),
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })

	// Cancel the middle occurrence, confirm it disappears.
	require.NoError(t, service.CancelStaffAppointmentOccurrence(calendarContext(organizerAccount.ID), detail.Appointment.ID, timezone.NewDate(2026, 1, 12)))
	events, err := service.ListMyStaffEvents(calendarContext(invitedAccount.ID), timezone.NewDate(2026, 1, 5), timezone.NewDate(2026, 1, 19))
	require.NoError(t, err)
	assert.Equal(t, []string{"2026-01-05", "2026-01-19"}, eventDates(events, calModels.EventSourceAppointment))

	// Edit the series (same cadence). The stale cancellation must be cleared, so
	// 2026-01-12 reappears in the edited series.
	_, err = service.UpdateStaffAppointment(calendarContext(organizerAccount.ID), detail.Appointment.ID, calendarSvc.UpdateAppointmentRequest{
		Title:      "Weekly standup (edited)",
		StartDate:  timezone.NewDate(2026, 1, 5),
		EndDate:    timezone.NewDate(2026, 1, 5),
		StartTime:  wallClock(9, 0),
		EndTime:    wallClock(9, 30),
		Recurrence: weekly(),
	})
	require.NoError(t, err)

	events, err = service.ListMyStaffEvents(calendarContext(invitedAccount.ID), timezone.NewDate(2026, 1, 5), timezone.NewDate(2026, 1, 19))
	require.NoError(t, err)
	assert.Equal(t, []string{"2026-01-05", "2026-01-12", "2026-01-19"}, eventDates(events, calModels.EventSourceAppointment))
}

// A weekly rule whose weekdays exclude the StartDate weekday must export a
// DTSTART on the first matching weekday, matching the in-app expansion, so
// external calendars don't render a phantom occurrence on the StartDate.
func TestCalendarServiceIntegration_RecurringICSStartsAtFirstMatchingWeekday(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "FirstDay", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "FirstDay", "Invitee")
	t.Cleanup(func() {
		testpkg.CleanupStaffFixtures(t, db, invitedStaff.ID, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, invitedAccount.ID, organizerAccount.ID)
	})

	endsOn := timezone.NewDate(2026, 5, 27)
	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Mittwochs-AG",
		StartDate:    timezone.NewDate(2026, 5, 4), // Monday — NOT a selected weekday
		EndDate:      timezone.NewDate(2026, 5, 4),
		StartTime:    wallClock(14, 0),
		EndTime:      wallClock(15, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     calModels.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"wednesday"},
			EndsOn:        &endsOn,
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })

	// The in-app expansion starts on the first Wednesday (2026-05-06), not the
	// Monday StartDate.
	events, err := service.ListMyStaffEvents(calendarContext(invitedAccount.ID), timezone.NewDate(2026, 5, 4), timezone.NewDate(2026, 5, 6))
	require.NoError(t, err)
	assert.Equal(t, []string{"2026-05-06"}, eventDates(events, calModels.EventSourceAppointment))

	// The ICS export must anchor DTSTART on that same first Wednesday.
	_, content, err := service.StaffAppointmentICS(calendarContext(organizerAccount.ID), detail.Appointment.ID)
	require.NoError(t, err)
	assert.Contains(t, content, "DTSTART;TZID=Europe/Berlin:20260506T140000")
	assert.NotContains(t, content, "20260504T140000", "must not anchor DTSTART on the non-matching StartDate")
	assert.Contains(t, content, "RRULE:FREQ=WEEKLY;BYDAY=WE")
}

// Editing an appointment must cancel any not-yet-sent notice queued by an earlier
// create/update, so the worker can't deliver stale details — regardless of
// whether the edit re-sends email.
func TestCalendarServiceIntegration_UpdateCancelsPendingNotifications(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	outbox := &recordingOutbox{}
	service := setupCalendarServiceWithOutbox(t, db, outbox)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "UpdateMail", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Elternabend",
		StartDate:    timezone.NewDate(2026, 4, 2),
		EndDate:      timezone.NewDate(2026, 4, 2),
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
	require.Len(t, outbox.enqueued, 1)

	base := calendarSvc.UpdateAppointmentRequest{
		Title:     "Elternabend (verschoben)",
		StartDate: timezone.NewDate(2026, 4, 3),
		EndDate:   timezone.NewDate(2026, 4, 3),
		StartTime: wallClock(18, 0),
		EndTime:   wallClock(19, 0),
	}

	// Edit WITHOUT re-sending: the stale pending mail is cancelled and no new mail
	// is queued.
	_, err = service.UpdateStaffAppointment(calendarContext(organizerAccount.ID), detail.Appointment.ID, base)
	require.NoError(t, err)
	assert.Equal(t, 1, outbox.cancelled, "update must cancel pending mail even when not re-sending")
	require.Len(t, outbox.enqueued, 1, "no new mail queued when send_email is off")

	// Edit WITH re-sending: cancels again, then queues a single update notice.
	resend := base
	resend.SendEmail = true
	_, err = service.UpdateStaffAppointment(calendarContext(organizerAccount.ID), detail.Appointment.ID, resend)
	require.NoError(t, err)
	assert.Equal(t, 2, outbox.cancelled)
	require.Len(t, outbox.enqueued, 2)
	assert.Equal(t, platformModels.EmailKindAppointmentUpdated, outbox.enqueued[1].Kind)
}

// The notification e-mail must advertise the first real occurrence, not the raw
// StartDate, for a weekly rule whose weekdays exclude the StartDate weekday —
// matching the in-app calendar and ICS export.
func TestCalendarServiceIntegration_RecurringEmailUsesFirstOccurrenceDate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	outbox := &recordingOutbox{}
	service := setupCalendarServiceWithOutbox(t, db, outbox)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "MailDate", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	endsOn := timezone.NewDate(2026, 5, 27)
	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Mittwochs-AG",
		StartDate:    timezone.NewDate(2026, 5, 4), // Monday — NOT a selected weekday
		EndDate:      timezone.NewDate(2026, 5, 4),
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

	require.Len(t, outbox.enqueued, 1)
	whenText, _ := outbox.enqueued[0].Payload["when_text"].(string)
	assert.Contains(t, whenText, "06.05.2026", "e-mail must advertise the first Wednesday")
	assert.NotContains(t, whenText, "04.05.2026", "e-mail must not advertise the non-matching Monday StartDate")
}

// A cancelled appointment must be non-respondable: the API shape flips
// CanRespond to false and the response endpoints reject RSVP changes.
func TestCalendarServiceIntegration_CancelledAppointmentNotRespondable(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "CancelRSVP", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "CancelRSVP", "Invitee")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, invitedStaff.ID, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, invitedAccount.ID, organizerAccount.ID)
	})

	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Elterngespräch",
		StartDate:    timezone.NewDate(2026, 6, 3),
		EndDate:      timezone.NewDate(2026, 6, 3),
		StartTime:    wallClock(16, 0),
		EndTime:      wallClock(16, 30),
		DeliveryMode: calModels.DeliveryModeRSVPRequired,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeStaff, ID: &invitedStaff.ID},
			{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })

	staffRecipientID := findRecipientByStaff(t, detail, invitedStaff.ID)
	parentRecipientID := findRecipientByGuardian(t, detail, parentChain.GuardianProfileID)

	// While active, responses are accepted and CanRespond is true.
	require.NoError(t, service.RespondToStaffInvitation(calendarContext(invitedAccount.ID), staffRecipientID, calModels.ResponseStatusAccepted))
	events, err := service.ListMyStaffEvents(calendarContext(invitedAccount.ID), timezone.NewDate(2026, 6, 3), timezone.NewDate(2026, 6, 3))
	require.NoError(t, err)
	require.NotEmpty(t, events)
	assert.True(t, events[0].CanRespond)

	// Cancel the appointment.
	_, err = service.CancelStaffAppointment(calendarContext(organizerAccount.ID), detail.Appointment.ID)
	require.NoError(t, err)

	// The API shape now forbids responding.
	events, err = service.ListMyStaffEvents(calendarContext(invitedAccount.ID), timezone.NewDate(2026, 6, 3), timezone.NewDate(2026, 6, 3))
	require.NoError(t, err)
	require.NotEmpty(t, events)
	assert.True(t, events[0].Cancelled)
	assert.False(t, events[0].CanRespond, "cancelled appointment must not be respondable")

	// Both response endpoints reject RSVP changes after cancellation.
	err = service.RespondToStaffInvitation(calendarContext(invitedAccount.ID), staffRecipientID, calModels.ResponseStatusDeclined)
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrInvalidRequest))

	err = service.RespondToParentInvitation(testpkg.TenantContext(1), parentChain.AccountID, parentRecipientID, calModels.ResponseStatusDeclined)
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrInvalidRequest))
}

// A cancelled appointment is terminal: editing it (which would re-notify while
// it stays cancelled) must be rejected.
func TestCalendarServiceIntegration_EditCancelledAppointmentRejected(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "EditCancel", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "EditCancel", "Invitee")
	t.Cleanup(func() {
		testpkg.CleanupStaffFixtures(t, db, invitedStaff.ID, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, invitedAccount.ID, organizerAccount.ID)
	})

	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Termin",
		StartDate:    timezone.NewDate(2026, 8, 3),
		EndDate:      timezone.NewDate(2026, 8, 3),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })

	_, err = service.CancelStaffAppointment(calendarContext(organizerAccount.ID), detail.Appointment.ID)
	require.NoError(t, err)

	_, err = service.UpdateStaffAppointment(calendarContext(organizerAccount.ID), detail.Appointment.ID, calendarSvc.UpdateAppointmentRequest{
		Title:     "Termin (editiert)",
		StartDate: timezone.NewDate(2026, 8, 4),
		EndDate:   timezone.NewDate(2026, 8, 4),
		StartTime: wallClock(9, 0),
		EndTime:   wallClock(10, 0),
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrInvalidRequest))
}

// Single-occurrence cancellation must reject non-recurring appointments and
// dates the series never generates.
func TestCalendarServiceIntegration_CancelOccurrenceValidation(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "OccValid", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "OccValid", "Invitee")
	t.Cleanup(func() {
		testpkg.CleanupStaffFixtures(t, db, invitedStaff.ID, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, invitedAccount.ID, organizerAccount.ID)
	})

	// Non-recurring appointment: cancelling an occurrence is rejected.
	single, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Einmalig",
		StartDate:    timezone.NewDate(2026, 9, 1),
		EndDate:      timezone.NewDate(2026, 9, 1),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, single.Appointment.ID) })

	err = service.CancelStaffAppointmentOccurrence(calendarContext(organizerAccount.ID), single.Appointment.ID, timezone.NewDate(2026, 9, 1))
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrInvalidRequest))

	// Recurring appointment: a date the series does not generate is rejected, a
	// generated date succeeds.
	endsOn := timezone.NewDate(2026, 9, 28)
	recurring, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Montags",
		StartDate:    timezone.NewDate(2026, 9, 7), // Monday
		EndDate:      timezone.NewDate(2026, 9, 7),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     calModels.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"monday"},
			EndsOn:        &endsOn,
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, recurring.Appointment.ID) })

	// 2026-09-08 is a Tuesday — not part of the Monday series.
	err = service.CancelStaffAppointmentOccurrence(calendarContext(organizerAccount.ID), recurring.Appointment.ID, timezone.NewDate(2026, 9, 8))
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrInvalidRequest))

	// 2026-09-14 is a generated Monday — accepted.
	require.NoError(t, service.CancelStaffAppointmentOccurrence(calendarContext(organizerAccount.ID), recurring.Appointment.ID, timezone.NewDate(2026, 9, 14)))
}

// A recurrence that generates no occurrence (weekly weekday outside its EndsOn
// window) must be rejected at create time rather than persisting a phantom.
func TestCalendarServiceIntegration_EmptyRecurrenceRejected(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "EmptyRec", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "EmptyRec", "Invitee")
	t.Cleanup(func() {
		testpkg.CleanupStaffFixtures(t, db, invitedStaff.ID, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, invitedAccount.ID, organizerAccount.ID)
	})

	endsOn := timezone.NewDate(2026, 5, 5) // Tuesday — no Wednesday in [Mon, Tue]
	_, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Nie",
		StartDate:    timezone.NewDate(2026, 5, 4), // Monday
		EndDate:      timezone.NewDate(2026, 5, 4),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     calModels.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"wednesday"},
			EndsOn:        &endsOn,
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrInvalidRequest))
}

// The exported ICS SEQUENCE advances on every change (edit and single-occurrence
// cancellation) so subscribers treat the event as a newer revision.
func TestCalendarServiceIntegration_ICSRevisionSequence(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Revision", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "Revision", "Invitee")
	t.Cleanup(func() {
		testpkg.CleanupStaffFixtures(t, db, invitedStaff.ID, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, invitedAccount.ID, organizerAccount.ID)
	})

	endsOn := timezone.NewDate(2026, 10, 26)
	weekly := func() *calendarSvc.RecurrenceRequest {
		return &calendarSvc.RecurrenceRequest{
			Frequency:     calModels.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"monday"},
			EndsOn:        &endsOn,
		}
	}
	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Reihe",
		StartDate:    timezone.NewDate(2026, 10, 5), // Monday
		EndDate:      timezone.NewDate(2026, 10, 5),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		Recurrence:   weekly(),
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })

	// Freshly created: revision 0, so no SEQUENCE line.
	_, content, err := service.StaffAppointmentICS(calendarContext(organizerAccount.ID), detail.Appointment.ID)
	require.NoError(t, err)
	assert.NotContains(t, content, "SEQUENCE:")

	// Editing bumps the revision → SEQUENCE:1.
	_, err = service.UpdateStaffAppointment(calendarContext(organizerAccount.ID), detail.Appointment.ID, calendarSvc.UpdateAppointmentRequest{
		Title:      "Reihe (v2)",
		StartDate:  timezone.NewDate(2026, 10, 5),
		EndDate:    timezone.NewDate(2026, 10, 5),
		StartTime:  wallClock(9, 0),
		EndTime:    wallClock(10, 0),
		Recurrence: weekly(),
	})
	require.NoError(t, err)
	_, content, err = service.StaffAppointmentICS(calendarContext(organizerAccount.ID), detail.Appointment.ID)
	require.NoError(t, err)
	assert.Contains(t, content, "SEQUENCE:1")
	assert.Contains(t, content, "LAST-MODIFIED:")

	// Cancelling a single occurrence bumps it again → SEQUENCE:2 (and an EXDATE).
	require.NoError(t, service.CancelStaffAppointmentOccurrence(calendarContext(organizerAccount.ID), detail.Appointment.ID, timezone.NewDate(2026, 10, 12)))
	_, content, err = service.StaffAppointmentICS(calendarContext(organizerAccount.ID), detail.Appointment.ID)
	require.NoError(t, err)
	assert.Contains(t, content, "SEQUENCE:2")
	assert.Contains(t, content, "EXDATE;TZID=Europe/Berlin:20261012T090000")
}

// A sparse-but-valid recurrence whose first occurrence is more than a year out
// (unbounded, so it genuinely produces occurrences) must NOT be rejected as
// empty.
func TestCalendarServiceIntegration_SparseRecurrenceAccepted(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Sparse", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "Sparse", "Invitee")
	t.Cleanup(func() {
		testpkg.CleanupStaffFixtures(t, db, invitedStaff.ID, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, invitedAccount.ID, organizerAccount.ID)
	})

	// Monthly, every 24 months, on day 30. Start 2026-01-31 (day 30 in Jan is
	// before the start), so the first real occurrence is 2028-01-30 — over a year
	// out but perfectly valid.
	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Alle zwei Jahre",
		StartDate:    timezone.NewDate(2026, 1, 31),
		EndDate:      timezone.NewDate(2026, 1, 31),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     calModels.RecurrenceFrequencyMonthly,
			IntervalCount: 24,
			MonthDays:     []int{30},
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })

	// The first occurrence lands on 2028-01-30, in-app.
	events, err := service.ListMyStaffEvents(calendarContext(invitedAccount.ID), timezone.NewDate(2028, 1, 30), timezone.NewDate(2028, 1, 30))
	require.NoError(t, err)
	assert.Equal(t, []string{"2028-01-30"}, eventDates(events, calModels.EventSourceAppointment))
}

// The subscription feed must not export a count-bounded recurring series whose
// occurrences are all before the feed lookback window (the SQL window treats a
// NULL ends_on as open-ended).
func TestCalendarServiceIntegration_FeedSkipsExpiredCountBoundedSeries(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	cfg := calendarTestConfig(db)
	repos := repositories.NewFactory(db)
	cfg.AccountRepo = repos.Account
	cfg.ParentsURL = "https://parents.test"
	service := calendarSvc.NewService(cfg)

	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "ExpiredFeed", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	// A weekly series bounded by occurrence_count=3, starting 200 days ago: all
	// three occurrences are well before the feed's 30-day lookback.
	count := 3
	pastStart := timezone.TodayDate().AddDays(-200)
	expired, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Abgelaufene Reihe",
		StartDate:    pastStart,
		EndDate:      pastStart,
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:       calModels.RecurrenceFrequencyWeekly,
			IntervalCount:   1,
			OccurrenceCount: &count,
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, expired.Appointment.ID) })

	// A current appointment inside the feed window.
	current, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Aktueller Termin",
		StartDate:    timezone.TodayDate().AddDays(5),
		EndDate:      timezone.TodayDate().AddDays(5),
		StartTime:    wallClock(15, 0),
		EndTime:      wallClock(16, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, current.Appointment.ID) })

	httpsURL, _, err := service.ParentCalendarFeedURL(testpkg.TenantContext(1), parentChain.AccountID)
	require.NoError(t, err)
	token := strings.TrimPrefix(httpsURL, "https://parents.test/api/calendar-feed/")
	_, content, err := service.ParentCalendarFeedByToken(testpkg.TenantContext(1), token)
	require.NoError(t, err)

	assert.Contains(t, content, "SUMMARY:Aktueller Termin", "current appointment must be in the feed")
	assert.NotContains(t, content, "SUMMARY:Abgelaufene Reihe", "expired count-bounded series must be excluded")
}

// Cancelling the same occurrence twice (as two concurrent requests would) must
// converge via the conflict-safe upsert instead of the second insert hitting the
// (tenant, appointment, date) unique constraint and returning a 500.
func TestCalendarServiceIntegration_OccurrenceCancelIsConflictSafe(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	overrideRepo := repositories.NewFactory(db).CalendarOccurrenceOverride
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Conflict", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "Conflict", "Invitee")
	t.Cleanup(func() {
		testpkg.CleanupStaffFixtures(t, db, invitedStaff.ID, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, invitedAccount.ID, organizerAccount.ID)
	})

	endsOn := timezone.NewDate(2026, 1, 26)
	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Montags",
		StartDate:    timezone.NewDate(2026, 1, 5), // Monday
		EndDate:      timezone.NewDate(2026, 1, 5),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(9, 30),
		DeliveryMode: calModels.DeliveryModeInformational,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     calModels.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"monday"},
			EndsOn:        &endsOn,
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })

	// The insert path is exercised twice for the same occurrence; the second call
	// takes the ON CONFLICT DO UPDATE branch and must NOT error.
	ctx := testpkg.TenantContext(1)
	require.NoError(t, overrideRepo.CancelOccurrence(ctx, detail.Appointment.ID, timezone.NewDate(2026, 1, 12)))
	require.NoError(t, overrideRepo.CancelOccurrence(ctx, detail.Appointment.ID, timezone.NewDate(2026, 1, 12)))

	// The occurrence is excluded from the series exactly once.
	events, err := service.ListMyStaffEvents(calendarContext(invitedAccount.ID), timezone.NewDate(2026, 1, 5), timezone.NewDate(2026, 1, 26))
	require.NoError(t, err)
	assert.Equal(t, []string{"2026-01-05", "2026-01-19", "2026-01-26"}, eventDates(events, calModels.EventSourceAppointment))
}

// A monthly rule that can never produce a valid occurrence (February-only on day
// 31) must be rejected at create time rather than persisting an invisible,
// phantom-exporting appointment.
func TestCalendarServiceIntegration_ImpossibleMonthlyRecurrenceRejected(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Impossible", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "Impossible", "Invitee")
	t.Cleanup(func() {
		testpkg.CleanupStaffFixtures(t, db, invitedStaff.ID, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, invitedAccount.ID, organizerAccount.ID)
	})

	_, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Nie",
		StartDate:    timezone.NewDate(2026, 2, 15), // February — reached every 12 months
		EndDate:      timezone.NewDate(2026, 2, 15),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     calModels.RecurrenceFrequencyMonthly,
			IntervalCount: 12,
			MonthDays:     []int{31}, // February never has a 31st
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrInvalidRequest))
}

func TestCalendarServiceIntegration_ParentCalendarHidesUnbookedPlannedTimetable(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	staff := testpkg.CreateTestStaff(t, db, "Care", "Planner")
	room := testpkg.CreateTestRoom(t, db, "Parent Calendar Unbooked Room")
	wednesday := timezone.NewDate(2026, 4, 8)
	instance := testpkg.CreateTestActivityInstance(t, db, wednesday, room.ID, testpkg.ActivityInstanceOpts{
		StartHHMM: "13:00",
		EndHHMM:   "16:00",
		Title:     "Child Betreuung",
	})
	studentLink := testpkg.CreateTestInstanceStudent(t, db, instance.ID, parentChain.StudentID, "")
	arrival := testpkg.CreateTestArrivalSchedule(t, db, parentChain.StudentID, scheduleModels.WeekdayMonday, staff.ID, "13:00")

	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.student_arrival_schedules", arrival.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.instance_students", studentLink.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", instance.ID)
		testpkg.CleanupActivityFixtures(t, db, room.ID)
		testpkg.CleanupStaffFixtures(t, db, staff.ID)
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
	})

	events, err := service.ListMyParentEvents(testpkg.TenantContext(1), parentChain.AccountID, wednesday, wednesday)
	require.NoError(t, err)
	assert.Empty(t, events)
}

// Reporting a child sick flips every still-expected slot of that day to
// 'absent' with status-day provenance — including slots on days the care plan
// never booked. That is not a decision about the slot, so the care-day filter
// must still apply: a sick note may not resurrect a care event for a Wednesday
// the child was never booked for (#1747 review).
func TestCalendarServiceIntegration_ParentCalendarHidesUnbookedStatusDayTimetable(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	staff := testpkg.CreateTestStaff(t, db, "Care", "Reporter")
	room := testpkg.CreateTestRoom(t, db, "Parent Calendar Status Day Room")
	wednesday := timezone.NewDate(2026, 4, 8)
	instance := testpkg.CreateTestActivityInstance(t, db, wednesday, room.ID, testpkg.ActivityInstanceOpts{
		StartHHMM: "13:00",
		EndHHMM:   "16:00",
		Title:     "Child Betreuung",
	})
	studentLink := testpkg.CreateTestInstanceStudent(t, db, instance.ID, parentChain.StudentID, "")
	arrival := testpkg.CreateTestArrivalSchedule(t, db, parentChain.StudentID, scheduleModels.WeekdayMonday, staff.ID, "13:00")

	ctx := testpkg.TenantContext(1)
	statusDay := &activeModels.StudentStatusDay{
		StudentID:  parentChain.StudentID,
		Date:       wednesday,
		Status:     activeModels.StudentStatusDaySick,
		ReportedAt: time.Now().UTC(),
		Source:     activeModels.StudentStatusSourceManual,
	}
	statusDay.SetTenantID(1)
	require.NoError(t, repositories.NewFactory(db).StudentStatusDay.UpsertReported(ctx, statusDay))

	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "active.student_status_days", statusDay.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.student_arrival_schedules", arrival.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.instance_students", studentLink.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", instance.ID)
		testpkg.CleanupActivityFixtures(t, db, room.ID)
		testpkg.CleanupStaffFixtures(t, db, staff.ID)
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
	})

	// Guard: the cascade really did take the row out of 'expected'.
	var status string
	var statusDayID *int64
	require.NoError(t, db.NewSelect().
		ColumnExpr("status, student_status_day_id").
		TableExpr("schedule.instance_students").
		Where("id = ?", studentLink.ID).
		Scan(ctx, &status, &statusDayID))
	require.Equal(t, scheduleModels.AttendanceStatusAbsent, status)
	require.NotNil(t, statusDayID)

	events, err := service.ListMyParentEvents(ctx, parentChain.AccountID, wednesday, wednesday)
	require.NoError(t, err)
	assert.Empty(t, events)
}

// The mirror image: once the block is completed the verdict is frozen in the
// stored marker. A care-plan edit afterwards must not retroactively erase a day
// the child actually attended — the same rule the attendance history follows.
func TestCalendarServiceIntegration_ParentCalendarKeepsCompletedTimetableWithoutMarker(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	staff := testpkg.CreateTestStaff(t, db, "Care", "Historian")
	room := testpkg.CreateTestRoom(t, db, "Parent Calendar Completed Room")
	wednesday := timezone.NewDate(2026, 4, 8)
	instance := testpkg.CreateTestActivityInstance(t, db, wednesday, room.ID, testpkg.ActivityInstanceOpts{
		Status:    scheduleModels.InstanceStatusCompleted,
		StartHHMM: "13:00",
		EndHHMM:   "16:00",
		Title:     "Child Betreuung",
	})
	studentLink := testpkg.CreateTestInstanceStudent(t, db, instance.ID, parentChain.StudentID, "")
	arrival := testpkg.CreateTestArrivalSchedule(t, db, parentChain.StudentID, scheduleModels.WeekdayMonday, staff.ID, "13:00")

	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.student_arrival_schedules", arrival.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.instance_students", studentLink.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", instance.ID)
		testpkg.CleanupActivityFixtures(t, db, room.ID)
		testpkg.CleanupStaffFixtures(t, db, staff.ID)
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
	})

	events, err := service.ListMyParentEvents(testpkg.TenantContext(1), parentChain.AccountID, wednesday, wednesday)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, calModels.EventSourceTimetable, events[0].Source)
}

// Cancelling a single occurrence must clear any pending create/update notice, so
// a queued email can't announce a date parents will no longer see (mirrors the
// cancel/delete/update stale-notification cleanup).
func TestCalendarServiceIntegration_CancelOccurrenceClearsPendingNotifications(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	outbox := &recordingOutbox{}
	service := setupCalendarServiceWithOutbox(t, db, outbox)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "OccMail", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, parentChain)
		testpkg.CleanupStaffFixtures(t, db, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	endsOn := timezone.NewDate(2026, 1, 26)
	detail, err := service.CreateStaffAppointment(calendarContext(organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Wöchentlich",
		StartDate:    timezone.NewDate(2026, 1, 5), // Monday
		EndDate:      timezone.NewDate(2026, 1, 5),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		SendEmail:    true,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     calModels.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"monday"},
			EndsOn:        &endsOn,
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupCalendarAppointment(t, db, detail.Appointment.ID) })
	require.Len(t, outbox.enqueued, 1, "create with send_email queues one notice")

	// Removing a single occurrence clears the still-pending notice.
	require.NoError(t, service.CancelStaffAppointmentOccurrence(calendarContext(organizerAccount.ID), detail.Appointment.ID, timezone.NewDate(2026, 1, 12)))
	assert.Equal(t, 1, outbox.cancelled, "single-occurrence cancel must clear pending notifications")
}
