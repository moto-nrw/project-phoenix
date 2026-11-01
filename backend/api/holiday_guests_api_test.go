// Router-level tests for the holiday guests of the school that hosts a
// holiday week (#4666, ADR 0120): the school picks a group, the confirmed
// guests of other schools become children of that group whose care covers
// the week, the staff see only care, emergency contacts and pickup persons,
// the access ends after the week by the service clock and at once with a
// cancellation, every access stands in the access log, and the school
// permissions apply.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type holidayGuestSheetBody struct {
	StudentID       int64  `json:"student_id"`
	StartsOn        string `json:"starts_on"`
	EndsOn          string `json:"ends_on"`
	TimeModelName   string `json:"time_model_name"`
	TimeModelStarts string `json:"time_model_starts_at"`
	TimeModelEnds   string `json:"time_model_ends_at"`
	HomeSchoolID    *int64 `json:"home_school_id"`
	GuestSchoolName string `json:"guest_school_name"`
	Emergency       []struct {
		Name         string `json:"name"`
		Phone        string `json:"phone"`
		Relationship string `json:"relationship"`
	} `json:"emergency_contacts"`
	Pickup []struct {
		Name string `json:"name"`
	} `json:"pickup_contacts"`
	ReadOnly bool `json:"read_only"`
}

// holidayGuestFixture adds the holiday guest routes to the host fixture,
// over the same service clock, and a group "Feriengruppe" of the OGS Nord,
// which hosts week 1. roster names the children of that group the school
// cares for on a day, the roster the group page, the check-in and the kiosk
// read; reconcile runs the hourly reconciliation of the OGS Nord; child reads
// the class and the care interval of a child of the OGS Nord.
type holidayGuestFixture struct {
	holidayHostFixture
	router    http.Handler
	nord      int64
	group     int64
	roster    func(t *testing.T, day calendar.Date) map[string]int64
	reconcile func(t *testing.T)
	child     func(t *testing.T, student int64) (class, status, from, until string, health *string)
}

// careDays lists the planned days of a child as "date arrival-pickup", the
// dated arrival and pickup exceptions the group's day planning reads.
func (f holidayGuestFixture) careDays(t *testing.T, student int64) []string {
	t.Helper()
	var days []string
	require.NoError(t, f.db.NewRaw(`SELECT a.exception_date::text || ' ' || to_char(a.expected_arrival, 'HH24:MI') || '-' || to_char(p.pickup_time, 'HH24:MI')
		FROM schedule.student_arrival_exceptions AS a
		JOIN schedule.student_pickup_exceptions AS p ON p.student_id = a.student_id AND p.exception_date = a.exception_date
		WHERE a.student_id = ? ORDER BY a.exception_date`, student).Scan(t.Context(), &days))
	return days
}

func newHolidayGuestFixture(t *testing.T, name string) holidayGuestFixture {
	t.Helper()
	f := newHolidayHostFixture(t, name)
	persons := owners.MustNewPeopleDirectory(testpkg.NewDatabase(t, f.db))
	accessLog := owners.NewTestAuditPersistence(testpkg.NewDatabase(t, f.db), nil).DataAccessLog
	clock := func() calendar.Date { return f.today.Load().(calendar.Date) }
	carePlan, err := owners.NewUnobservedCarePlan(testpkg.NewDatabase(t, f.db), owners.MustNewUnobservedTimetable(testpkg.NewDatabase(t, f.db)), persons)
	require.NoError(t, err)
	staff, err := NewRFIDTestPeople(testpkg.NewDatabase(t, f.db))
	require.NoError(t, err)
	guests := owners.NewHolidayGuests(persons, carePlan, staff, accessLog, testHealthCipher(), clock)
	organizations, err := owners.NewOrganizationTenancy(testpkg.NewDatabase(t, f.db))
	require.NoError(t, err)
	unit := testpkg.TenantRuntime(t, f.db)
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(tenant.WithUnitOfWork(r.Context(), unit)))
		})
	})
	router.Mount("/api/holiday-weeks", NewHolidayHostGuestHTTP(NewHolidayHostHTTP(organizations, persons, accessLog, testHealthCipher(), clock), guests).Router(ProtectedTenantRoutes))
	router.Mount("/api/holiday-guests", NewHolidayGuestHTTP(guests).Router(ProtectedTenantRoutes))
	nord := f.office.schools[0]
	group := testpkg.CreateTestEducationGroupForTenant(t, f.db, nord, "Feriengruppe").ID
	inNord := func(t *testing.T, fn func(context.Context) error) {
		t.Helper()
		id, err := tenant.NewTenantID(nord)
		require.NoError(t, err)
		require.NoError(t, tenant.WithinTenant(tenant.WithUnitOfWork(t.Context(), unit), id, fn))
	}
	return holidayGuestFixture{
		holidayHostFixture: f, router: testpkg.SessionVerifier(router), nord: nord, group: group,
		roster: func(t *testing.T, day calendar.Date) map[string]int64 {
			t.Helper()
			children := map[string]int64{}
			inNord(t, func(ctx context.Context) error {
				entries, err := persons.ListStudentRosterByGroup(ctx, []int64{group}, day.String())
				for _, entry := range entries {
					children[entry.FirstName+" "+entry.LastName] = entry.Record.ID
				}
				return err
			})
			return children
		},
		reconcile: func(t *testing.T) {
			t.Helper()
			inNord(t, func(ctx context.Context) error { return guests.ReconcileHolidayGuests(ctx, nord) })
		},
		child: func(t *testing.T, student int64) (string, string, string, string, *string) {
			t.Helper()
			var class, status, from, until string
			var health *string
			inNord(t, func(ctx context.Context) error {
				record, err := persons.ReadEnrollmentStudent(ctx, student, "")
				class, status, from, until, health = record.SchoolClass, record.Status, record.EnrolledFrom, record.EnrolledUntil, record.HealthInfo
				return err
			})
			return class, status, from, until, health
		},
	}
}

func (f holidayGuestFixture) do(t *testing.T, token, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&payload).Encode(body))
	}
	req := httptest.NewRequest(method, path, &payload)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	return rec
}

func (f holidayGuestFixture) assign(t *testing.T, token string, week, group int64) *httptest.ResponseRecorder {
	t.Helper()
	return f.do(t, token, http.MethodPut, fmt.Sprintf("/api/holiday-weeks/%d/guest-group", week),
		map[string]string{"group_id": fmt.Sprint(group)})
}

func (f holidayGuestFixture) sheet(t *testing.T, token string, student int64) *httptest.ResponseRecorder {
	t.Helper()
	return f.do(t, token, http.MethodGet, fmt.Sprintf("/api/holiday-guests/%d", student), nil)
}

// guestLog lists the school's holiday guest access log entries as
// "resource_type:guests:automatic" for a transfer and "resource_type:student"
// for a sheet read, oldest first.
func (f holidayGuestFixture) guestLog(t *testing.T, school int64) []string {
	t.Helper()
	var rows []struct {
		ResourceType string `bun:"resource_type"`
		StudentID    *int64 `bun:"student_id"`
		Guests       string `bun:"guests"`
		Automatic    string `bun:"automatic"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT resource_type, student_id, COALESCE(metadata->>'guests', '') AS guests,
			COALESCE(metadata->>'automatic', '') AS automatic
		FROM audit.data_access_log WHERE tenant_id = ? AND resource_type LIKE 'holiday_guest%' ORDER BY id`, school).
		Scan(t.Context(), &rows))
	entries := make([]string, len(rows))
	for i, row := range rows {
		if row.StudentID != nil {
			entries[i] = fmt.Sprintf("%s:%d", row.ResourceType, *row.StudentID)
			continue
		}
		entries[i] = row.ResourceType + ":" + row.Guests + ":" + row.Automatic
	}
	return entries
}

func names(children map[string]int64) []string {
	result := make([]string, 0, len(children))
	for name := range children {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func TestHolidayGuestsJoinTheGroupOfTheHostingSchoolOnlyFromAConfirmedBooking(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Feriengäste")
	week := f.offer.Weeks[0].ID
	lead := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead)
	weekDay := calendar.NewDate(2099, 7, 8)

	rec := f.assign(t, lead, week, f.group)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, enrollment.HolidayGuestTransfer{WeekID: week, GroupID: f.group, Guests: 0},
		decodeSchoolPortalData[enrollment.HolidayGuestTransfer](t, rec.Body.Bytes()), "without a confirmed booking no child comes")
	assert.Empty(t, f.roster(t, weekDay))

	f.confirm(t, f.siblings, "confirm")
	f.reconcile(t)
	assert.Empty(t, f.roster(t, weekDay), "before its week the guest is in no group yet")
	var waiting int64
	require.NoError(t, f.db.NewRaw(`SELECT student_id FROM enrollment.holiday_guests WHERE tenant_id = ?`, f.nord).Scan(t.Context(), &waiting))
	_, status, _, _, _ := f.child(t, waiting)
	assert.Equal(t, "pending", status, "before its week the guest waits like a child before its first care day")

	f.today.Store(calendar.NewDate(2099, 7, 6))
	f.reconcile(t)
	children := f.roster(t, weekDay)
	assert.Equal(t, []string{"Ben Muster"}, names(children),
		"the confirmed holiday guest joins the group; Lia is a child of the OGS Nord already, Tom is not confirmed")
	listed := decodeSchoolPortalData[[]struct {
		GuestGroupID *int64 `json:"guest_group_id"`
		Guests       int    `json:"guests"`
	}](t, f.get(t, lead, "/").Body.Bytes())
	require.Len(t, listed, 1)
	require.NotNil(t, listed[0].GuestGroupID)
	assert.Equal(t, f.group, *listed[0].GuestGroupID)
	assert.Equal(t, 1, listed[0].Guests)

	ben := children["Ben Muster"]
	assert.Equal(t, []string{
		"2099-07-06 08:00-13:00", "2099-07-07 08:00-13:00", "2099-07-08 08:00-13:00", "2099-07-09 08:00-13:00", "2099-07-10 08:00-13:00",
	}, f.careDays(t, ben), "each day of the week expects the guest with the times of its time model")
	assert.Equal(t, waiting, ben, "the first day of the week moves the waiting copy into the group")
	class, status, from, until, health := f.child(t, ben)
	assert.Equal(t, enrollment.HolidayGuestSchoolClass, class)
	assert.Equal(t, "active", status)
	assert.Equal(t, "2099-07-06", from)
	assert.Equal(t, "2099-07-10", until)
	assert.Nil(t, health, "no health information leaves the Träger")

	rec = f.sheet(t, lead, ben)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	sheet := decodeSchoolPortalData[holidayGuestSheetBody](t, rec.Body.Bytes())
	assert.Equal(t, "bis 13 Uhr", sheet.TimeModelName)
	assert.Equal(t, "2099-07-06", sheet.StartsOn)
	assert.Equal(t, "2099-07-10", sheet.EndsOn)
	assert.Nil(t, sheet.HomeSchoolID)
	assert.Equal(t, "Grundschule Melle", sheet.GuestSchoolName)
	require.Len(t, sheet.Emergency, 2)
	assert.Equal(t, "Eva Muster", sheet.Emergency[0].Name, "the registering parent comes first")
	assert.Equal(t, "0151 2345678", sheet.Emergency[0].Phone)
	assert.Equal(t, "Eltern", sheet.Emergency[0].Relationship)
	assert.Equal(t, "Oma Muster", sheet.Emergency[1].Name)
	assert.Equal(t, "05402 1234", sheet.Emergency[1].Phone)
	require.Len(t, sheet.Pickup, 1)
	assert.Equal(t, "Opa Muster", sheet.Pickup[0].Name)
	assert.False(t, sheet.ReadOnly)
	for _, private := range []string{"Erdnüsse", "eva.muster@example.org"} {
		assert.NotContains(t, rec.Body.String(), private, "only care, emergency contacts and pickup persons leave the Träger")
	}

	// Tom's booking is confirmed later: the hourly reconciliation brings him.
	var tom int64
	require.NoError(t, f.db.NewRaw(`SELECT id FROM enrollment.holiday_registrations WHERE id <> ? AND organization_id =
		(SELECT organization_id FROM enrollment.holiday_registrations WHERE id = ?)`, f.siblings, f.siblings).Scan(t.Context(), &tom))
	f.confirm(t, tom, "confirm")
	f.reconcile(t)
	f.reconcile(t)
	assert.Equal(t, []string{"Ben Muster", "Tom Beispiel"}, names(f.roster(t, weekDay)), "a second run copies nobody twice")
	rec = f.sheet(t, lead, f.roster(t, weekDay)["Tom Beispiel"])
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, f.offer.Schools[1].ID, *decodeSchoolPortalData[holidayGuestSheetBody](t, rec.Body.Bytes()).HomeSchoolID)

	assert.Equal(t, []string{
		"holiday_guest_transfer:0:false",
		"holiday_guest_transfer:1:true",
		fmt.Sprintf("holiday_guest_sheet:%d", ben),
		"holiday_guest_transfer:2:true",
		fmt.Sprintf("holiday_guest_sheet:%d", f.roster(t, weekDay)["Tom Beispiel"]),
	}, f.guestLog(t, f.nord), "every transfer and every sheet read stands in the school's access log")

	// The Träger-Büro rebooks Ben away from the week and back: the ended guest
	// returns as the same child instead of failing the reconciliation.
	var benChild int64
	require.NoError(t, f.db.NewRaw(`SELECT id FROM enrollment.holiday_registration_children WHERE holiday_registration_id = ? AND first_name = 'Ben'`,
		f.siblings).Scan(t.Context(), &benChild))
	rebook := func(weeks ...int) {
		t.Helper()
		bookings := []map[string]any{}
		for _, index := range weeks {
			bookings = append(bookings, map[string]any{"week_id": f.offer.Weeks[index].ID, "time_model_id": f.offer.Weeks[index].TimeModels[0].ID})
		}
		path := fmt.Sprintf("%s/registrations/%d/children/%d/bookings", holidayPeriodPath(f.periodID(t)), f.siblings, benChild)
		rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, path, map[string]any{"bookings": bookings})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}
	rebook(1)
	f.reconcile(t)
	assert.Equal(t, []string{"Tom Beispiel"}, names(f.roster(t, weekDay)), "the guest rebooked away leaves the group")
	assert.Empty(t, f.careDays(t, ben))
	rebook(0, 1)
	f.reconcile(t)
	back := f.roster(t, weekDay)
	assert.Equal(t, []string{"Ben Muster", "Tom Beispiel"}, names(back), "the guest booked back returns to the group")
	assert.Equal(t, ben, back["Ben Muster"], "the returning guest is the same child")
	assert.Len(t, f.careDays(t, ben), 5, "the returning guest has its days planned again")
	assert.Equal(t, http.StatusOK, f.sheet(t, lead, ben).Code)

	// A cancellation ends the guest at the next reconciliation.
	f.confirm(t, f.siblings, "cancel")
	f.reconcile(t)
	assert.Equal(t, []string{"Tom Beispiel"}, names(f.roster(t, calendar.NewDate(2099, 7, 6))), "from today the cancelled guest is no longer in the group")
	assert.Empty(t, f.careDays(t, ben), "the cancelled guest has no planned day left")
	rec = f.sheet(t, lead, ben)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "enrollment.holiday_guest_not_found")

}

func TestHolidayGuestRoutesNeedTheSchoolPermissionAndTheHostingSchool(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Feriengäste Rechte")
	f.today.Store(calendar.NewDate(2099, 7, 6)) // the first day of the week
	week := f.offer.Weeks[0].ID
	lead := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead)
	f.confirm(t, f.siblings, "confirm")
	require.Equal(t, http.StatusOK, f.assign(t, lead, week, f.group).Code)
	ben := f.roster(t, calendar.NewDate(2099, 7, 8))["Ben Muster"]
	require.NotZero(t, ben)

	readOnly := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.StudentsRead)
	assert.Equal(t, http.StatusForbidden, f.assign(t, readOnly, week, f.group).Code, "picking the group needs users:create")
	assert.Equal(t, http.StatusForbidden, f.sheet(t, readOnly, ben).Code, "the contacts need users:read")

	sued := f.office.schools[1]
	other := f.staffOf(t, sued, permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead)
	rec := f.assign(t, other, week, testpkg.CreateTestEducationGroupForTenant(t, f.db, sued, "Fremde Gruppe").ID)
	assert.Equal(t, http.StatusNotFound, rec.Code, "a school of the same Träger cannot take a week it does not host")
	assert.Contains(t, rec.Body.String(), "enrollment.hosted_holiday_week_not_found")
	assert.Equal(t, http.StatusNotFound, f.sheet(t, other, ben).Code, "another school sees no guest of the host")

	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Feriengäste Fremd", identityaccess.TraegerRoleAdministration)
	stranger := f.staffOf(t, foreign.schools[0], permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead)
	assert.Equal(t, http.StatusNotFound, f.assign(t, stranger, week, f.group).Code, "a school of another Träger sees no week")
	assert.Equal(t, http.StatusNotFound, f.sheet(t, stranger, ben).Code)

	foreignGroup := testpkg.CreateTestEducationGroupForTenant(t, f.db, sued, "Gruppe der OGS Süd").ID
	rec = f.assign(t, lead, week, foreignGroup)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "the group must be one of the school's")
	assert.Contains(t, rec.Body.String(), "enrollment.holiday_guest_group_invalid")
	assert.Empty(t, f.guestLog(t, sued), "a refused access is not logged")
}

func TestHolidayGuestsLeaveGroupAndCheckInAfterTheWeek(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Feriengäste Ende")
	f.today.Store(calendar.NewDate(2099, 7, 6)) // the first day of the week
	week := f.offer.Weeks[0].ID
	lead := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead)
	f.confirm(t, f.siblings, "confirm")
	require.Equal(t, http.StatusOK, f.assign(t, lead, week, f.group).Code)
	ben := f.roster(t, calendar.NewDate(2099, 7, 10))["Ben Muster"]
	require.NotZero(t, ben, "the last day of the week still shows the guest")

	f.today.Store(calendar.NewDate(2099, 7, 11))
	assert.Empty(t, f.roster(t, calendar.NewDate(2099, 7, 11)), "from the day after the week the guest is no longer cared for")
	rec := f.sheet(t, lead, ben)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.True(t, decodeSchoolPortalData[holidayGuestSheetBody](t, rec.Body.Bytes()).ReadOnly, "seven days only to read")

	f.today.Store(calendar.NewDate(2099, 7, 17))
	f.reconcile(t)
	assert.Equal(t, http.StatusOK, f.sheet(t, lead, ben).Code, "the seventh day after the week still shows the sheet")

	f.today.Store(calendar.NewDate(2099, 7, 18))
	f.reconcile(t)
	rec = f.sheet(t, lead, ben)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "enrollment.holiday_guest_not_found")
	var contacts int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM enrollment.holiday_guest_contacts WHERE tenant_id = ?`, f.nord).Scan(t.Context(), &contacts))
	assert.Zero(t, contacts, "the contacts are deleted after the read-only days")
}

func TestHolidayGuestReconciliationSurvivesAMissingAuthorAndReplansOnTheNextGroupChoice(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Feriengäste Abgleich")
	f.today.Store(calendar.NewDate(2099, 7, 6)) // the first day of the week
	week := f.offer.Weeks[0].ID
	lead := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead)
	weekDay := calendar.NewDate(2099, 7, 8)
	require.Equal(t, http.StatusOK, f.assign(t, lead, week, f.group).Code)

	// The account that picked the group loses its staff record: the hourly
	// run still copies the confirmed guest, only its days stay unplanned.
	stranger := testpkg.CreateTestAccount(t, f.db, "feriengast-ohne-personal")
	_, err := f.db.ExecContext(t.Context(), `UPDATE enrollment.holiday_host_groups SET assigned_by_account_id = ? WHERE tenant_id = ?`,
		stranger.ID, f.nord)
	require.NoError(t, err)
	f.confirm(t, f.siblings, "confirm")
	f.reconcile(t)
	ben := f.roster(t, weekDay)["Ben Muster"]
	require.NotZero(t, ben, "the guest joins the group although its days cannot be planned")
	assert.Empty(t, f.careDays(t, ben))

	// The group is deleted and Ben rebooked to "bis 16 Uhr" meanwhile: the
	// next group choice plans every day with the new times.
	_, err = f.db.ExecContext(t.Context(), `DELETE FROM education.groups WHERE id = ?`, f.group)
	require.NoError(t, err)
	var benChild int64
	require.NoError(t, f.db.NewRaw(`SELECT id FROM enrollment.holiday_registration_children WHERE holiday_registration_id = ? AND first_name = 'Ben'`,
		f.siblings).Scan(t.Context(), &benChild))
	path := fmt.Sprintf("%s/registrations/%d/children/%d/bookings", holidayPeriodPath(f.periodID(t)), f.siblings, benChild)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, path, map[string]any{"bookings": []map[string]any{
		{"week_id": week, "time_model_id": f.offer.Weeks[0].TimeModels[1].ID},
	}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	f.reconcile(t)
	assert.Empty(t, f.careDays(t, ben), "without a group nobody is there to plan the days")

	again := testpkg.CreateTestEducationGroupForTenant(t, f.db, f.nord, "Feriengruppe neu").ID
	require.Equal(t, http.StatusOK, f.assign(t, lead, week, again).Code)
	assert.Equal(t, []string{
		"2099-07-06 08:00-16:00", "2099-07-07 08:00-16:00", "2099-07-08 08:00-16:00", "2099-07-09 08:00-16:00", "2099-07-10 08:00-16:00",
	}, f.careDays(t, ben), "the group choice plans the days with the times booked meanwhile")
}
