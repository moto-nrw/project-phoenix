package compose_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	calendarSvc "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// setupSchoolCalendarService composes the calendar with the School
// Calendar's closing days and holiday periods bound (#4798).
func setupSchoolCalendarService(t *testing.T, db *bun.DB) (calendarSvc.FullService, schoolcalendar.Capability) {
	t.Helper()
	cfg := calendarTestConfig(t, db)
	_, _, reposCalendar, _, _, _, _ := calendarTestRepositories(t, db)
	days := reposCalendar.SchoolCalendar()
	cfg.SchoolDays = days
	return owners.NewCalendarPortal(cfg), days
}

// createTestLehrkraft creates a staff member who uses moto schule only: the
// Lehrkraft system role, no calendar:own.
func createTestLehrkraft(t *testing.T, db *bun.DB, firstName, lastName string) (*testpkg.Staff, *testpkg.AccountFixture) {
	t.Helper()
	staff, account := testpkg.CreateTestStaffWithAccount(t, db, firstName, lastName)
	testpkg.AssignLehrkraftSystemRole(t, db, account.ID, testpkg.Tenant(t))
	return staff, account
}

func schoolTrip(title string, parents string, classes ...string) calendarSvc.SchoolAppointmentRequest {
	return calendarSvc.SchoolAppointmentRequest{
		Title:     title,
		StartDate: portalDate(2026, 3, 4),
		EndDate:   portalDate(2026, 3, 4),
		StartTime: wallClock(8, 0),
		EndTime:   wallClock(12, 30),
		EventType: appointments.SchoolEventTypeTrip,
		Parents:   parents,
		Classes:   classes,
	}
}

func findAppointmentEvent(events []calendarSvc.Event, appointmentID int64) *calendarSvc.Event {
	for i := range events {
		if events[i].AppointmentID != nil && *events[i].AppointmentID == strconv.FormatInt(appointmentID, 10) {
			return &events[i]
		}
	}
	return nil
}

func TestSchoolCalendarIntegration_SchultermineReachOGSAndChosenParents(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service, _ := setupSchoolCalendarService(t, db)
	lehrkraft, lehrkraftAccount := createTestLehrkraft(t, db, "Schul", "Lehrkraft")
	_, ogsAccount := testpkg.CreateTestCalendarStaff(t, db, "OGS", "Betreuung")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	testpkg.CreateTestClassTeacher(t, db, lehrkraft.ID, "1a")

	detail, err := testpkg.TenantQuery1(t, calendarContext(t, lehrkraftAccount.ID), db, service.CreateSchoolAppointment, schoolTrip("Zoo", calendarSvc.SchoolParentsClasses, "1a"))
	require.NoError(t, err)
	require.NotNil(t, detail.Appointment.SchoolEventType)
	assert.Equal(t, appointments.SchoolEventTypeTrip, *detail.Appointment.SchoolEventType)
	assert.Equal(t, appointments.DeliveryModeInformational, detail.Appointment.DeliveryMode)
	assert.NotZero(t, findOptionalRecipientByGuardian(detail, parentChain.GuardianProfileID), "the parents of class 1a see the Schultermin")

	ogsEvents, err := testpkg.TenantQuery2(t, calendarContext(t, ogsAccount.ID), db, service.ListMyStaffEvents, portalDate(2026, 3, 2), portalDate(2026, 3, 8))
	require.NoError(t, err)
	ogsEvent := findAppointmentEvent(ogsEvents, detail.Appointment.ID)
	require.NotNil(t, ogsEvent, "the OGS always sees a Schultermin")
	require.NotNil(t, ogsEvent.SchoolEventType)
	assert.False(t, ogsEvent.CanEdit, "the OGS cannot change a Schultermin")

	view, err := testpkg.TenantQuery2(t, calendarContext(t, lehrkraftAccount.ID), db, service.ListSchoolCalendar, portalDate(2026, 3, 2), portalDate(2026, 3, 8))
	require.NoError(t, err)
	own := findAppointmentEvent(view.Events, detail.Appointment.ID)
	require.NotNil(t, own)
	assert.True(t, own.CanEdit, "the Lehrkraft changes their own Schultermin")

	parentEvents, err := service.ListMyParentEvents(testpkg.Ctx(t), parentChain.AccountID, portalDate(2026, 3, 2), portalDate(2026, 3, 8))
	require.NoError(t, err)
	assert.NotNil(t, findAppointmentEvent(parentEvents, detail.Appointment.ID))
}

func TestSchoolCalendarIntegration_WithoutParentsTheFamiliesDoNotSeeIt(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service, _ := setupSchoolCalendarService(t, db)
	_, lehrkraftAccount := createTestLehrkraft(t, db, "Ohne", "Eltern")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	detail, err := testpkg.TenantQuery1(t, calendarContext(t, lehrkraftAccount.ID), db, service.CreateSchoolAppointment, schoolTrip("Konferenz", calendarSvc.SchoolParentsNone))
	require.NoError(t, err)
	assert.Zero(t, findOptionalRecipientByGuardian(detail, parentChain.GuardianProfileID))

	parentEvents, err := service.ListMyParentEvents(testpkg.Ctx(t), parentChain.AccountID, portalDate(2026, 3, 2), portalDate(2026, 3, 8))
	require.NoError(t, err)
	assert.Nil(t, findAppointmentEvent(parentEvents, detail.Appointment.ID))
}

func TestSchoolCalendarIntegration_ParentsOnlyOfOwnClasses(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service, _ := setupSchoolCalendarService(t, db)
	lehrkraft, lehrkraftAccount := createTestLehrkraft(t, db, "Fremde", "Klasse")
	testpkg.CreateTestClassTeacher(t, db, lehrkraft.ID, "2b")

	_, err := testpkg.TenantQuery1(t, calendarContext(t, lehrkraftAccount.ID), db, service.CreateSchoolAppointment, schoolTrip("Ausflug 1a", calendarSvc.SchoolParentsClasses, "1a"))
	require.ErrorIs(t, err, calendarSvc.ErrForbidden)

	_, err = testpkg.TenantQuery1(t, calendarContext(t, lehrkraftAccount.ID), db, service.CreateSchoolAppointment, schoolTrip("Ohne Klasse", calendarSvc.SchoolParentsClasses))
	require.ErrorIs(t, err, calendarSvc.ErrInvalidRequest)

	invalid := schoolTrip("Ohne Art", calendarSvc.SchoolParentsNone)
	invalid.EventType = "party"
	_, err = testpkg.TenantQuery1(t, calendarContext(t, lehrkraftAccount.ID), db, service.CreateSchoolAppointment, invalid)
	require.ErrorIs(t, err, calendarSvc.ErrInvalidRequest)
}

func TestSchoolCalendarIntegration_LehrkraftChangesOnlyOwnSchultermine(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service, _ := setupSchoolCalendarService(t, db)
	_, ownerAccount := createTestLehrkraft(t, db, "Eigene", "Lehrkraft")
	_, otherAccount := createTestLehrkraft(t, db, "Andere", "Lehrkraft")

	detail, err := testpkg.TenantQuery1(t, calendarContext(t, ownerAccount.ID), db, service.CreateSchoolAppointment, schoolTrip("Wandertag", calendarSvc.SchoolParentsNone))
	require.NoError(t, err)

	view, err := testpkg.TenantQuery2(t, calendarContext(t, otherAccount.ID), db, service.ListSchoolCalendar, portalDate(2026, 3, 2), portalDate(2026, 3, 8))
	require.NoError(t, err)
	seen := findAppointmentEvent(view.Events, detail.Appointment.ID)
	require.NotNil(t, seen, "every Lehrkraft sees a Schultermin")
	assert.False(t, seen.CanEdit)

	changed := schoolTrip("Wandertag verschoben", calendarSvc.SchoolParentsNone)
	_, err = testpkg.TenantQuery2(t, calendarContext(t, otherAccount.ID), db, service.UpdateSchoolAppointment, detail.Appointment.ID, changed)
	require.ErrorIs(t, err, calendarSvc.ErrForbidden)
	err = testpkg.TenantCommand1(t, calendarContext(t, otherAccount.ID), db, service.DeleteSchoolAppointment, detail.Appointment.ID)
	require.ErrorIs(t, err, calendarSvc.ErrForbidden)

	changed.EventType = appointments.SchoolEventTypeLessonsEnd
	updated, err := testpkg.TenantQuery2(t, calendarContext(t, ownerAccount.ID), db, service.UpdateSchoolAppointment, detail.Appointment.ID, changed)
	require.NoError(t, err)
	assert.Equal(t, "Wandertag verschoben", updated.Appointment.Title)
	require.NotNil(t, updated.Appointment.SchoolEventType)
	assert.Equal(t, appointments.SchoolEventTypeLessonsEnd, *updated.Appointment.SchoolEventType)

	require.NoError(t, testpkg.TenantCommand1(t, calendarContext(t, ownerAccount.ID), db, service.DeleteSchoolAppointment, detail.Appointment.ID))
	view, err = testpkg.TenantQuery2(t, calendarContext(t, otherAccount.ID), db, service.ListSchoolCalendar, portalDate(2026, 3, 2), portalDate(2026, 3, 8))
	require.NoError(t, err)
	assert.Nil(t, findAppointmentEvent(view.Events, detail.Appointment.ID))
}

func TestSchoolCalendarIntegration_OGSAppointmentStaysOutOfReachOfMotoSchule(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service, _ := setupSchoolCalendarService(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "OGS", "Leitung")
	// A dual-role account: organizes in the OGS portal and holds the
	// Lehrkraft role too.
	testpkg.AssignLehrkraftSystemRole(t, db, organizerAccount.ID, testpkg.Tenant(t))

	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Teamsitzung",
		StartDate:    portalDate(2026, 3, 5),
		StartTime:    wallClock(14, 0),
		EndTime:      wallClock(15, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		Targets:      []calendarSvc.AppointmentTarget{{Type: appointments.TargetTypeAllTeachers}},
	})
	require.NoError(t, err)

	_, err = testpkg.TenantQuery2(t, calendarContext(t, organizerAccount.ID), db, service.UpdateSchoolAppointment, detail.Appointment.ID, schoolTrip("Umgedeutet", calendarSvc.SchoolParentsNone))
	require.ErrorIs(t, err, calendarSvc.ErrForbidden)
	view, err := testpkg.TenantQuery2(t, calendarContext(t, organizerAccount.ID), db, service.ListSchoolCalendar, portalDate(2026, 3, 2), portalDate(2026, 3, 8))
	require.NoError(t, err)
	event := findAppointmentEvent(view.Events, detail.Appointment.ID)
	require.NotNil(t, event)
	assert.False(t, event.CanEdit, "moto schule edits Schultermine only")
}

func TestSchoolCalendarIntegration_OGSChoosesLehrkraefte(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service, _ := setupSchoolCalendarService(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "OGS", "Planung")
	lehrkraft, lehrkraftAccount := createTestLehrkraft(t, db, "Eingeladene", "Lehrkraft")
	_, colleagueAccount := testpkg.CreateTestCalendarStaff(t, db, "OGS", "Kollegin")

	withTeachers, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Elternabend",
		StartDate:    portalDate(2026, 3, 5),
		StartTime:    wallClock(18, 0),
		EndTime:      wallClock(19, 30),
		DeliveryMode: appointments.DeliveryModeRSVPRequired,
		Targets:      []calendarSvc.AppointmentTarget{{Type: appointments.TargetTypeAllTeachers}},
	})
	require.NoError(t, err)
	recipientID := findRecipientByStaff(t, withTeachers, lehrkraft.ID)
	require.Len(t, withTeachers.Recipients, 1, "all_teachers reaches the Lehrkräfte only, not the OGS staff")

	staffOnly, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Dienstbesprechung",
		StartDate:    portalDate(2026, 3, 6),
		StartTime:    wallClock(13, 0),
		EndTime:      wallClock(14, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		Targets:      []calendarSvc.AppointmentTarget{{Type: appointments.TargetTypeAllStaff}},
	})
	require.NoError(t, err)

	view, err := testpkg.TenantQuery2(t, calendarContext(t, lehrkraftAccount.ID), db, service.ListSchoolCalendar, portalDate(2026, 3, 2), portalDate(2026, 3, 8))
	require.NoError(t, err)
	invited := findAppointmentEvent(view.Events, withTeachers.Appointment.ID)
	require.NotNil(t, invited, "the Lehrkraft sees what the OGS shows to Lehrkräfte")
	assert.True(t, invited.CanRespond)
	assert.False(t, invited.CanEdit)
	assert.Nil(t, findAppointmentEvent(view.Events, staffOnly.Appointment.ID), "all_staff does not reach the Lehrkräfte")

	require.NoError(t, testpkg.TenantCommand2(t, calendarContext(t, lehrkraftAccount.ID), db, service.RespondToSchoolInvitation, recipientID, appointments.ResponseStatusAccepted))

	colleagueEvents, err := testpkg.TenantQuery2(t, calendarContext(t, colleagueAccount.ID), db, service.ListMyStaffEvents, portalDate(2026, 3, 2), portalDate(2026, 3, 8))
	require.NoError(t, err)
	assert.Nil(t, findAppointmentEvent(colleagueEvents, withTeachers.Appointment.ID))
}

func TestSchoolCalendarIntegration_ShowsClosingDaysAndHolidays(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service, days := setupSchoolCalendarService(t, db)
	_, lehrkraftAccount := createTestLehrkraft(t, db, "Ferien", "Lehrkraft")
	ctx := calendarContext(t, lehrkraftAccount.ID)

	_, err := testpkg.TenantQuery1(t, ctx, db, days.CreateClosingDay, schoolcalendar.CreateClosingDay{ClosingDayFields: schoolcalendar.ClosingDayFields{
		StartDate: "2026-03-04", EndDate: "2026-03-04", Reason: "Pädagogischer Tag",
	}})
	require.NoError(t, err)
	_, err = testpkg.TenantQuery1(t, ctx, db, days.CreateCalendarPeriod, schoolcalendar.CreateCalendarPeriod{CalendarPeriodFields: schoolcalendar.CalendarPeriodFields{
		Name: "Osterferien", PeriodType: schoolcalendar.PeriodTypeHoliday, StartDate: "2026-03-30", EndDate: "2026-04-10", WeekCycleLength: 1, IsActive: true,
	}})
	require.NoError(t, err)

	view, err := testpkg.TenantQuery2(t, ctx, db, service.ListSchoolCalendar, portalDate(2026, 3, 1), portalDate(2026, 4, 5))
	require.NoError(t, err)
	require.Len(t, view.ClosingDays, 1)
	assert.Equal(t, "Pädagogischer Tag", view.ClosingDays[0].Name)
	require.Len(t, view.Holidays, 1)
	assert.Equal(t, "Osterferien", view.Holidays[0].Name)
	assert.Equal(t, "2026-04-10", view.Holidays[0].EndDate)

	outside, err := testpkg.TenantQuery2(t, ctx, db, service.ListSchoolCalendar, portalDate(2026, 5, 1), portalDate(2026, 5, 7))
	require.NoError(t, err)
	assert.Empty(t, outside.ClosingDays)
	assert.Empty(t, outside.Holidays)
}

func TestSchoolCalendarIntegration_LehrkraftSubscribesToTheFeed(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos, reposPeople, reposCalendar, _, _, _, _ := calendarTestRepositories(t, db)
	cfg := calendarTestConfig(t, db)
	cfg = owners.BindCalendarFeedFacts(cfg, nil, repos.ParentCalendarFeed, repos.StaffCalendarFeedToken, reposPeople.Person)
	cfg.StaffFeedTombstoneRepo = reposCalendar.CalendarStaffFeedTombstone
	cfg.SchoolDays = reposCalendar.SchoolCalendar()
	cfg.FrontendURL = "https://moto.test"
	service := owners.NewCalendarPortal(cfg)
	_, lehrkraftAccount := createTestLehrkraft(t, db, "Abo", "Lehrkraft")

	_, err := testpkg.TenantQuery1(t, calendarContext(t, lehrkraftAccount.ID), db, service.CreateSchoolAppointment, calendarSvc.SchoolAppointmentRequest{
		Title:     "Sportfest",
		StartDate: portalDate(2026, 1, 8),
		StartTime: wallClock(8, 0),
		EndTime:   wallClock(12, 0),
		EventType: appointments.SchoolEventTypeOther,
	})
	require.NoError(t, err)

	httpsURL, _, err := testpkg.TenantQueryPair0(t, calendarContext(t, lehrkraftAccount.ID), db, service.SchoolCalendarFeedURL)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(httpsURL, "https://moto.test/api/calendar-feed/"))

	_, content, err := service.StaffCalendarFeedByToken(testpkg.Ctx(t), strings.TrimPrefix(httpsURL, "https://moto.test/api/calendar-feed/"))
	require.NoError(t, err, "a Lehrkraft without calendar:own still owns the feed")
	assert.Contains(t, content, "SUMMARY:Sportfest")
}

func TestSchoolCalendarIntegration_FeedRequiresCurrentCalendarPermission(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos, reposPeople, reposCalendar, _, _, _, _ := calendarTestRepositories(t, db)
	cfg := calendarTestConfig(t, db)
	cfg = owners.BindCalendarFeedFacts(cfg, nil, repos.ParentCalendarFeed, repos.StaffCalendarFeedToken, reposPeople.Person)
	cfg.StaffFeedTombstoneRepo = reposCalendar.CalendarStaffFeedTombstone
	cfg.CalDAVPolicy = fixedCalDAVSettings{enabled: true}
	cfg.FrontendURL = "https://moto.test"
	service := owners.NewCalendarPortal(cfg)
	_, account := testpkg.CreateTestStaffWithAccount(t, db, "Kalender", "Rechteentzug")
	role := testpkg.CreateTestRole(t, db, "school-calendar")
	_, err := db.ExecContext(testpkg.Ctx(t), "UPDATE auth.roles SET school_portal = TRUE WHERE id = ?", role.ID)
	require.NoError(t, err)
	_, err = db.ExecContext(testpkg.Ctx(t), "INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)", account.ID, role.ID, testpkg.Tenant(t))
	require.NoError(t, err)
	_, err = db.ExecContext(testpkg.Ctx(t), `INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT ?, id FROM auth.permissions WHERE name = 'calendar:school'`, role.ID)
	require.NoError(t, err)

	url, _, err := testpkg.TenantQueryPair0(t, calendarContext(t, account.ID), db, service.SchoolCalendarFeedURL)
	require.NoError(t, err)
	token := strings.TrimPrefix(url, "https://moto.test/api/calendar-feed/")
	require.NotEmpty(t, token)
	_, content, err := service.StaffCalendarFeedByToken(testpkg.Ctx(t), token)
	require.NoError(t, err)
	assert.Contains(t, content, "BEGIN:VCALENDAR")
	_, err = service.AuthenticateStaffCalDAV(testpkg.Ctx(t), account.Email, token)
	require.NoError(t, err)

	// Keep the school-portal role and the issued token, revoking only its calendar grant.
	_, err = db.ExecContext(testpkg.Ctx(t), "DELETE FROM auth.role_permissions WHERE role_id = ?", role.ID)
	require.NoError(t, err)
	_, _, err = service.StaffCalendarFeedByToken(testpkg.Ctx(t), token)
	assert.ErrorIs(t, err, calendarSvc.ErrNotFound)
	_, err = service.AuthenticateStaffCalDAV(testpkg.Ctx(t), account.Email, token)
	assert.ErrorIs(t, err, calendarSvc.ErrNotFound)

	// The same token remains usable when the account still has OGS calendar access.
	testpkg.GrantTestPermission(t, db, testpkg.Tenant(t), account.ID, "calendar:own")
	_, _, err = service.StaffCalendarFeedByToken(testpkg.Ctx(t), token)
	require.NoError(t, err)
	_, err = service.AuthenticateStaffCalDAV(testpkg.Ctx(t), account.Email, token)
	require.NoError(t, err)
}

func TestSchoolCalendarIntegration_TeacherTargetsRequireCalendarAccess(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service, _ := setupSchoolCalendarService(t, db)
	_, organizer := testpkg.CreateTestCalendarStaff(t, db, "OGS", "Einladungen")
	expected := make(map[int64]bool)
	for _, permission := range []string{"calendar:school", "calendar:own", ""} {
		staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Schulportal", "Kalenderrecht")
		role := testpkg.CreateTestRole(t, db, "teacher-target")
		_, err := db.ExecContext(testpkg.Ctx(t), "UPDATE auth.roles SET school_portal = TRUE WHERE id = ?", role.ID)
		require.NoError(t, err)
		_, err = db.ExecContext(testpkg.Ctx(t), "INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)", account.ID, role.ID, testpkg.Tenant(t))
		require.NoError(t, err)
		// Start with school access in every case, then revoke it where appropriate.
		_, err = db.ExecContext(testpkg.Ctx(t), `INSERT INTO auth.role_permissions (role_id, permission_id)
			SELECT ?, id FROM auth.permissions WHERE name = 'calendar:school'`, role.ID)
		require.NoError(t, err)
		if permission != "calendar:school" {
			_, err = db.ExecContext(testpkg.Ctx(t), "DELETE FROM auth.role_permissions WHERE role_id = ?", role.ID)
			require.NoError(t, err)
		}
		if permission == "calendar:own" {
			testpkg.GrantTestPermission(t, db, testpkg.Tenant(t), account.ID, permission)
		}
		if permission != "" {
			expected[staff.ID] = true
		}
	}

	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizer.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title: "Einladung an erreichbare Lehrkräfte", StartDate: portalDate(2026, 3, 5),
		StartTime: wallClock(18, 0), EndTime: wallClock(19, 0),
		DeliveryMode: appointments.DeliveryModeRSVPRequired,
		Targets:      []calendarSvc.AppointmentTarget{{Type: appointments.TargetTypeAllTeachers}},
	})
	require.NoError(t, err)
	require.Len(t, detail.Recipients, len(expected), "teachers without either calendar permission receive no pending invitation")
	for staffID := range expected {
		assert.NotZero(t, findRecipientByStaff(t, detail, staffID))
	}
}
