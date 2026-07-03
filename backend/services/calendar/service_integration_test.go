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
	userModels "github.com/moto-nrw/project-phoenix/models/users"
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

func TestCalendarServiceIntegration_AttendeeOverviewVisibility(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestStaffWithAccount(t, db, "Overview", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestStaffWithAccount(t, db, "Overview", "Teacher")
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

func TestCalendarServiceIntegration_AttendeeOverviewAccessRules(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestStaffWithAccount(t, db, "Private", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestStaffWithAccount(t, db, "Private", "Invitee")
	outsider, outsiderAccount := testpkg.CreateTestStaffWithAccount(t, db, "Private", "Outsider")
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
	organizer, organizerAccount := testpkg.CreateTestStaffWithAccount(t, db, "Target", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestStaffWithAccount(t, db, "Target", "Invitee")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	t.Cleanup(func() {
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
	require.Len(t, detail.Targets, 3)

	parentEvents, err := service.ListMyParentEvents(testpkg.TenantContext(1), parentChain.AccountID, timezone.NewDate(2026, 2, 12), timezone.NewDate(2026, 2, 12))
	require.NoError(t, err)
	require.NotEmpty(t, parentEvents)
	assert.True(t, parentEvents[0].CanViewOverview)
	assert.False(t, parentEvents[0].CanRespond)
	require.NotNil(t, parentEvents[0].ResponseStatus)
	assert.Equal(t, calModels.ResponseStatusInfo, *parentEvents[0].ResponseStatus)
}

func TestCalendarServiceIntegration_InvalidCreateTargets(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestStaffWithAccount(t, db, "Invalid", "Targets")
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
	organizer, organizerAccount := testpkg.CreateTestStaffWithAccount(t, db, "Invalid", "Recurrence")
	invitedStaff, invitedAccount := testpkg.CreateTestStaffWithAccount(t, db, "Invalid", "RecurrenceInvitee")
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

func TestCalendarServiceIntegration_ResponseAndOverviewErrors(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestStaffWithAccount(t, db, "Errors", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestStaffWithAccount(t, db, "Errors", "Invitee")
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
	organizer, organizerAccount := testpkg.CreateTestStaffWithAccount(t, db, "Repo", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestStaffWithAccount(t, db, "Repo", "Invitee")
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

	emptyGuardianAppointments, err := repos.CalendarAppointment.ListVisibleForGuardianProfiles(ctx, nil, timezone.NewDate(2026, 2, 13), timezone.NewDate(2026, 2, 13))
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
