package calendar_test

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	calendarSvc "github.com/moto-nrw/project-phoenix/services/calendar"
	usercontextSvc "github.com/moto-nrw/project-phoenix/services/usercontext"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupCalendarService(t *testing.T, db *bun.DB) calendarSvc.Service {
	t.Helper()

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

	return calendarSvc.NewService(calendarSvc.Config{
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
		UserContext:          userContext,
		DB:                   db,
	})
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
	for _, recipient := range detail.Recipients {
		if recipient.GuardianProfileID != nil && *recipient.GuardianProfileID == guardianID {
			return recipient.ID
		}
	}
	t.Fatalf("guardian recipient %d not found", guardianID)
	return 0
}

func TestCalendarServiceIntegration_CreateRecurringAppointmentAndResponses(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestStaffWithAccount(t, db, "Calendar", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestStaffWithAccount(t, db, "Calendar", "Invitee")
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
	organizer, organizerAccount := testpkg.CreateTestStaffWithAccount(t, db, "Info", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestStaffWithAccount(t, db, "Info", "Invitee")
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

func TestCalendarServiceIntegration_StaffCalendarIncludesAssignedTimetable(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Timetable", "Staff")
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
	assert.Equal(t, parentChain.StudentID, *events[0].StudentID)
	require.NotNil(t, events[0].StudentName)
	assert.Contains(t, *events[0].StudentName, "Felix")
}
