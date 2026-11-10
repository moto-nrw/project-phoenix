package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	enrollmentHTTP "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Drive the production PUT handler and owner with a fixed Berlin change day.
// Authentication/rights are covered by TestTraegerHolidayPeriodsNeedTheEnrollmentRight;
// this route-sized router binds the already authenticated organization.
func editHolidayTimesOn(t *testing.T, db *bun.DB, org, account int64, booking ownBooking, on calendar.Date, start, end string) {
	t.Helper()
	ctx := context.Background()
	var model int64
	require.NoError(t, db.NewRaw(`SELECT holiday_week_time_model_id FROM enrollment.holiday_registration_bookings
  WHERE organization_id = ? AND holiday_registration_child_id = ?`, org, booking.child).Scan(ctx, &model))
	body := holidayPeriodBody{Name: "Sommerferien", RegistrationOpensOn: "2026-01-01", RegistrationClosesOn: "2026-06-01",
		Weeks: []holidayWeekBody{{ID: booking.week, StartsOn: "2026-06-08", EndsOn: "2026-06-12", HostSchoolID: testpkg.Tenant(t),
			TimeModels: []holidayTimeModelBody{{ID: model, Name: "Ganztag", StartsAt: start, EndsAt: end, PriceCents: 4500}}}}}
	principal, err := permissions.NewPrincipal(permissions.PrincipalInput{AccountID: account, OrganizationID: org, Scope: tenant.ScopeOrg, Permissions: []string{identityaccess.TraegerRightEnrollmentManage}})
	require.NoError(t, err)
	review, places, allocation := enrollmentCompose.NewHolidayReviewAllocation(testHealthCipher(), nil, owners.HolidayTestSchoolCalendar{})
	resource := enrollmentHTTP.NewHolidayPeriodResource(enrollmentCompose.NewHolidayPeriods(func() calendar.Date { return on }, owners.HolidayTestSchoolCalendar{}),
		review, places, allocation, holidaySchoolNames{}, common.RequiresPermission(identityaccess.TraegerRightEnrollmentManage), common.RequiresPermission(identityaccess.TraegerRightHealthRead))
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scope := tenant.ClaimScope{}.BindOrganization(tenant.WithUnitOfWork(r.Context(), testpkg.TenantRuntime(t, db)), org, tenant.ScopeOrg)
			next.ServeHTTP(w, r.WithContext(permissions.WithPrincipal(scope, principal)))
		})
	})
	router.Mount("/traeger/holiday-periods", resource.TraegerRouter(func(r chi.Router, mount func(chi.Router, common.Middleware)) { mount(r, common.TraegerTxMiddleware) }))
	response := traegerJSON(t, router, "", http.MethodPut, holidayPeriodPath(booking.period), body)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
}

func TestStudentCareTimes_TimeEditsKeepPastHolidaySoll(t *testing.T) {
	t.Parallel()
	tc := setupStatisticsRoute(t)
	school := testpkg.Tenant(t)
	_, account := testpkg.CreateTestStaffWithAccountForTenant(t, tc.db, school, "Zeitänderung", "Leitung")
	own := testpkg.CreateTestStudent(t, tc.db, "Tina", "Eigen", "2a")
	guest := testpkg.CreateTestStudent(t, tc.db, "Gina", "Gast", "Feriengast")
	born := calendar.NewDate(2018, 3, 4)
	_, err := tc.db.ExecContext(context.Background(), `UPDATE users.persons SET birthday = ? WHERE id = ?`, born, own.PersonID)
	require.NoError(t, err)
	device := testpkg.CreateTestDevice(t, tc.db, "holiday-time-edits")
	monday := calendar.NewDate(2026, 6, 8)
	for _, child := range []int64{own.ID, guest.ID} {
		for day := 0; day < 5; day++ {
			insertAttendanceSpan(t, tc.db, school, child, device.ID, monday.AddDays(day), 8*time.Hour, 15*time.Hour+45*time.Minute)
		}
	}
	org := testpkg.CreateTestOrganization(t, tc.db, &testpkg.Organization{Name: "Träger Zeitänderung", Slug: fmt.Sprintf("time-edits-%d", testpkg.UniqueTestTenantID(t)), Active: true}).ID
	testpkg.OwnTraegerHolidayPeriods(t, tc.db, org)
	joinTraeger(t, tc.db, org, school)
	booking := ownHolidayBooking(t, tc.db, org, school, "Tina", "Eigen", born, monday, monday.AddDays(4), "08:00", "16:00", "confirmed")
	guestBooking := addGuestToTimeEditBooking(t, tc.db, org, booking, guest.ID, monday)
	assertHolidayTimeEdits(t, tc, account.ID, []int64{own.ID, guest.ID}, []string{"08:00/16:00", "08:00/16:00", "08:00/16:00", "08:00/16:00", "08:00/16:00"})
	// Wednesday 00:30 Berlin, not UTC Tuesday: the injected value is a calendar day.
	editHolidayTimesOn(t, tc.db, org, account.ID, booking, monday.AddDays(2), "09:00", "13:00")
	assertHolidayTimeEdits(t, tc, account.ID, []int64{own.ID, guest.ID}, []string{"08:00/16:00", "08:00/16:00", "09:00/13:00", "09:00/13:00", "09:00/13:00"})
	editHolidayTimesOn(t, tc.db, org, account.ID, booking, monday.AddDays(3), "07:00", "14:00")
	editHolidayTimesOn(t, tc.db, org, account.ID, booking, monday.AddDays(3), "10:00", "12:00")
	assertHolidayTimeEdits(t, tc, account.ID, []int64{own.ID, guest.ID}, []string{"08:00/16:00", "08:00/16:00", "09:00/13:00", "10:00/12:00", "10:00/12:00"})
	rebookOwnChild(t, tc.db, org, booking, "08:00", "11:00", "2026-06-12T00:30:00+02:00")
	rebookOwnChild(t, tc.db, org, guestBooking, "08:00", "11:00", "2026-06-12T00:30:00+02:00")
	_, err = tc.db.ExecContext(context.Background(), `UPDATE enrollment.holiday_week_time_models SET archived_at = NOW()
  WHERE organization_id = ? AND holiday_period_week_id = ?`, org, booking.week)
	require.NoError(t, err)
	assertHolidayTimeEdits(t, tc, account.ID, []int64{own.ID, guest.ID}, []string{"08:00/16:00", "08:00/16:00", "09:00/13:00", "10:00/12:00", "08:00/11:00"})
	// A later host/date move ends A's guest grant but must not erase its
	// already booked days, including the distinct model windows of Wednesday
	// and Thursday. The old copy alone still says 08:00–16:00 throughout.
	nextHost := testpkg.CreateTestSchoolInOrganization(t, tc.db, org, "Neue Ferien-OGS")
	_, err = tc.db.ExecContext(context.Background(), `UPDATE enrollment.holiday_period_weeks
  SET host_school_id = ?, starts_on = ?, ends_on = ? WHERE organization_id = ? AND id = ?`,
		nextHost, monday.AddDays(7), monday.AddDays(11), org, booking.week)
	require.NoError(t, err)
	_, err = tc.db.ExecContext(context.Background(), `UPDATE enrollment.holiday_guests SET ended_on = ?
  WHERE tenant_id = ? AND student_id = ?`, monday.AddDays(4), school, guest.ID)
	require.NoError(t, err)
	days := contractCareTimes(t, tc, account.ID, guest.ID)
	require.Len(t, days, 5)
	for i, want := range []string{"08:00/16:00", "08:00/16:00", "09:00/13:00", "10:00/12:00"} {
		require.NotNil(t, days[i].PlannedStart)
		require.NotNil(t, days[i].PlannedEnd)
		assert.Equal(t, want, *days[i].PlannedStart+"/"+*days[i].PlannedEnd, days[i].Date)
		assert.Equal(t, "holiday_booking", days[i].PlannedSource)
	}
}

func addGuestToTimeEditBooking(t *testing.T, db *bun.DB, org int64, booking ownBooking, student int64, monday calendar.Date) ownBooking {
	t.Helper()
	var child int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registration_children
  (organization_id, holiday_period_id, holiday_registration_id, position, first_name, last_name, birth_date)
  VALUES (?, ?, ?, 2, 'Gina', 'Gast', '2018-04-05') RETURNING id`, org, booking.period, booking.registration).Scan(context.Background(), &child))
	_, err := db.ExecContext(context.Background(), `INSERT INTO enrollment.holiday_registration_bookings
  (organization_id, holiday_period_id, holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id)
  SELECT organization_id, holiday_period_id, ?, holiday_period_week_id, holiday_week_time_model_id
  FROM enrollment.holiday_registration_bookings WHERE holiday_registration_child_id = ?`, child, booking.child)
	require.NoError(t, err)
	_, err = db.ExecContext(context.Background(), `INSERT INTO enrollment.holiday_guests
  (tenant_id, organization_id, holiday_period_week_id, holiday_registration_child_id, student_id, starts_on, ends_on,
   time_model_name, time_model_starts_at, time_model_ends_at)
  VALUES (?, ?, ?, ?, ?, ?, ?, 'Ganztag', '08:00', '16:00')`, testpkg.Tenant(t), org, booking.week, child, student, monday, monday.AddDays(4))
	require.NoError(t, err)
	return ownBooking{period: booking.period, week: booking.week, registration: booking.registration, child: child}
}

func assertHolidayTimeEdits(t *testing.T, tc *statisticsHTTPTestContext, account int64, children []int64, want []string) {
	t.Helper()
	for _, child := range children {
		days := contractCareTimes(t, tc, account, child)
		require.Len(t, days, len(want))
		for i, day := range days {
			require.NotNil(t, day.PlannedStart)
			require.NotNil(t, day.PlannedEnd)
			assert.Equal(t, want[i], *day.PlannedStart+"/"+*day.PlannedEnd, "child %d on %s", child, day.Date)
			assert.Equal(t, "holiday_booking", day.PlannedSource)
			if i < 2 {
				assert.False(t, day.Overrun, "a 15:45 checkout before the edit stays within 16:00")
			}
		}
	}
}

// The Träger report consumes the same historical day basis as the child's
// Soll und Ist: neither own children nor guests gain retrospective overruns.
func TestCareTimeOverruns_TimeEditKeepsThePastHolidayComparison(t *testing.T) {
	t.Parallel()
	f := newOverrunFixture(t)
	var org int64
	require.NoError(t, f.db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, f.school).Scan(context.Background(), &org))
	testpkg.OwnTraegerHolidayPeriods(t, f.db, org)
	born := calendar.NewDate(2018, 3, 4)
	_, err := f.db.ExecContext(context.Background(), `UPDATE users.persons SET birthday = ?
  WHERE id = (SELECT person_id FROM users.student_profiles WHERE id = ?)`, born, f.child)
	require.NoError(t, err)
	monday := calendar.NewDate(2026, 6, 8)
	booking := ownHolidayBooking(t, f.db, org, f.school, "Sina", "Soll", born, monday, monday.AddDays(4), "08:00", "16:00", "confirmed")
	guest := testpkg.CreateTestStudent(t, f.db, "Gina", "Gast", "Feriengast")
	addGuestToTimeEditBooking(t, f.db, org, booking, guest.ID, monday)
	device := testpkg.CreateTestDevice(t, f.db, "traeger-time-edit")
	for day := 0; day < 2; day++ {
		insertAttendanceSpan(t, f.db, f.school, guest.ID, device.ID, monday.AddDays(day), 8*time.Hour, 15*time.Hour+45*time.Minute)
	}
	path := "/traeger/care-time-overruns?from=2026-06-08&to=2026-06-09"
	code, before, raw := f.get(t, f.billing, path)
	require.Equal(t, http.StatusOK, code, raw)
	assert.Zero(t, before.Total.OverrunDays)
	assert.Zero(t, before.Total.UnbookedDays)
	assert.Zero(t, before.Total.ExtraMinutes)
	editHolidayTimesOn(t, f.db, org, f.adminID, booking, monday.AddDays(2), "09:00", "13:00")
	code, after, raw := f.get(t, f.billing, path)
	require.Equal(t, http.StatusOK, code, raw)
	assert.Equal(t, before, after, "the same four past child-days still have no overrun")
	compared := 0
	for _, school := range after.Schools {
		compared += school.ComparedDays
	}
	assert.Equal(t, 4, compared, "the report actually compares both children on both days")
}
