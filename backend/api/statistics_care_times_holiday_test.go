package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
)

// Soll und Ist with a confirmed holiday booking (#5994, ADR 0156 addendum):
// on the days of a holiday guest's booking its time model is the Soll and
// replaces the contract's; one child-day, one comparison.

// insertHolidayGuest records studentID as a holiday guest of school from
// startsOn to endsOn with the time model from to until, as the
// reconciliation of a confirmed booking does (ADR 0120).
func insertHolidayGuest(t *testing.T, db *bun.DB, school, studentID int64, startsOn, endsOn calendar.Date, endedOn *calendar.Date, from, until string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `INSERT INTO enrollment.holiday_guests
		(tenant_id, organization_id, holiday_period_week_id, holiday_registration_child_id, student_id,
			starts_on, ends_on, ended_on, time_model_name, time_model_starts_at, time_model_ends_at)
		VALUES (?, 0, ?, ?, ?, ?, ?, ?, 'Ganztag', ?, ?)`,
		school, studentID, studentID, studentID, startsOn, endsOn, endedOn, from, until)
	require.NoError(t, err)
}

// The regression of #5994: a contract until 13:00, a confirmed holiday
// booking until 16:00 and a check-out at 15:45 are no overrun.
func TestStudentCareTimes_HolidayBookingReplacesTheContract(t *testing.T) {
	t.Parallel()
	tc := setupStatisticsRoute(t)
	school := testpkg.Tenant(t)
	_, account := testpkg.CreateTestStaffWithAccountForTenant(t, tc.db, school, "Ferien", "Leitung")
	child := testpkg.CreateTestStudent(t, tc.db, "Fina", "Ferien", "2a").ID
	device := testpkg.CreateTestDevice(t, tc.db, "holiday-care-time-device")
	monday := calendar.NewDate(2026, 6, 8)
	insertAttendanceSpan(t, tc.db, school, child, device.ID, monday, 8*time.Hour, 15*time.Hour+45*time.Minute)
	insertAttendanceSpan(t, tc.db, school, child, device.ID, monday.AddDays(7), 8*time.Hour, 15*time.Hour+45*time.Minute)
	organization := testpkg.CreateTestOrganization(t, tc.db, &testpkg.Organization{
		Name: "Träger Ferien", Slug: fmt.Sprintf("traeger-ferien-%d", testpkg.UniqueTestTenantID(t)), Active: true}).ID
	testpkg.OwnTraegerCareContracts(t, tc.db, organization)
	testpkg.CreateTestCareContract(t, tc.db, organization, school, child, "versendet", calendar.NewDate(2026, 6, 1),
		testpkg.CareContractService{Weekday: "mon", Module: "OGS", EndsAt: "13:00"})
	insertHolidayGuest(t, tc.db, school, child, monday, monday, nil, "08:00", "16:00")

	days := contractCareTimes(t, tc, account.ID, child)
	require.Len(t, days, 2, "one day per attended Monday, none twice")
	holiday := days[0]
	assert.Equal(t, "2026-06-08", holiday.Date)
	assert.Equal(t, "holiday_booking", holiday.PlannedSource)
	require.NotNil(t, holiday.PlannedEnd)
	assert.Equal(t, "16:00", *holiday.PlannedEnd, "the contract's 13:00 does not override the holiday booking")
	assert.Equal(t, "within_plan", holiday.Kind)
	assert.False(t, holiday.Overrun)

	after := days[1]
	assert.Equal(t, "2026-06-15", after.Date)
	assert.Equal(t, "contract", after.PlannedSource, "after the holiday week the contract decides again")
	assert.Equal(t, "overrun", after.Kind)
}

// A cancelled booking leaves no holiday Soll: a guest the reconciliation
// ended before its week is compared like any child.
func TestStudentCareTimes_CancelledHolidayBookingLeavesTheSoll(t *testing.T) {
	t.Parallel()
	tc := setupStatisticsRoute(t)
	school := testpkg.Tenant(t)
	staff, account := testpkg.CreateTestStaffWithAccountForTenant(t, tc.db, school, "Storno", "Leitung")
	child := careTimeChild(t, tc, staff.ID)
	monday := calendar.NewDate(2026, 6, 8)
	insertHolidayGuest(t, tc.db, school, child, monday, monday.AddDays(4), &monday, "08:00", "10:00")

	days := contractCareTimes(t, tc, account.ID, child)
	require.Len(t, days, 3)
	assert.Equal(t, "care_plan", days[0].PlannedSource, "cancelled before its first day: the care plan stays")
	assert.Equal(t, "within_plan", days[0].Kind)
}

// The Träger reads the same day basis: the holiday booking removes the
// overrun the contract would show, the right and the log stay (#5994).
func TestCareTimeOverrunsReadTheHolidayBooking(t *testing.T) {
	t.Parallel()
	f := newOverrunFixture(t)
	// careTimeChild stays until 14:00 on 8 June; the contract ends at 13:00,
	// the holiday booking of that day at 14:00.
	insertHolidayGuest(t, f.db, f.school, f.child, calendar.NewDate(2026, 6, 8), calendar.NewDate(2026, 6, 8), nil, "08:00", "14:00")

	code, body, raw := f.get(t, f.billing, overrunWindow)
	require.Equal(t, http.StatusOK, code, raw)
	assert.Equal(t, overrunFiguresBody{Children: 1, OverrunDays: 1, UnbookedDays: 1, ExtraMinutes: 360 + 120}, body.Total,
		"8 June is within the holiday booking; 9 and 15 June stay")
	for _, school := range body.Schools {
		if school.SchoolID == f.school {
			assert.Equal(t, 3, school.ComparedDays, "three child-days, each compared once")
		}
	}

	code, _, raw = f.get(t, f.billing, "/traeger/care-time-overruns/children?from=2026-06-08&to=2026-06-16")
	assert.Equal(t, http.StatusForbidden, code, "names still need \"Verträge verwalten\": %s", raw)
	code, named, raw := f.get(t, f.admin, "/traeger/care-time-overruns/children?from=2026-06-08&to=2026-06-16")
	require.Equal(t, http.StatusOK, code, raw)
	require.Len(t, named.Children, 1)
	assert.Equal(t, 480, named.Children[0].ExtraMinutes)
	var logged int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM audit.data_access_log
		WHERE actor_account_id = ? AND resource_type = 'care_time_overruns'`, f.adminID).Scan(context.Background(), &logged))
	assert.Equal(t, 1, logged, "the named read is logged")
}

// ownHolidayBooking registers child of school for the week [startsOn,
// endsOn] the school hosts, with the time model from to until, and confirms
// it or leaves it received: a registration names the school as the child's
// home school, with the child's names and date of birth (#5994).
type ownBooking struct{ period, week, registration, child int64 }

func ownHolidayBooking(t *testing.T, db *bun.DB, organization, school int64, first, last string, born, startsOn, endsOn calendar.Date, from, until, status string) ownBooking {
	t.Helper()
	ctx := context.Background()
	var period, week, model, registration, child int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods (organization_id, name, registration_opens_on,
			registration_closes_on, status, published_at) VALUES (?, 'Sommerferien', '2026-01-01', '2026-06-01', 'published', NOW())
		RETURNING id`, organization).Scan(ctx, &period))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_period_weeks (organization_id, holiday_period_id, starts_on, ends_on, host_school_id)
		VALUES (?, ?, ?, ?, ?) RETURNING id`, organization, period, startsOn, endsOn, school).Scan(ctx, &week))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_week_time_models (organization_id, holiday_period_week_id, name, starts_at, ends_at, price_cents)
		VALUES (?, ?, 'Ganztag', ?, ?, 4500) RETURNING id`, organization, week, from, until).Scan(ctx, &model))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations (organization_id, holiday_period_id, guardian_first_name,
			guardian_last_name, guardian_email, guardian_phone, privacy_consent_at, status)
		VALUES (?, ?, 'Eva', 'Ferien', 'eva@example.org', '0123', NOW(), ?) RETURNING id`, organization, period, status).Scan(ctx, &registration))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registration_children (organization_id, holiday_period_id,
			holiday_registration_id, position, first_name, last_name, birth_date, home_school_id)
		VALUES (?, ?, ?, 1, ?, ?, ?, ?) RETURNING id`, organization, period, registration, first, last, born, school).Scan(ctx, &child))
	_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_bookings (organization_id, holiday_period_id,
			holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id) VALUES (?, ?, ?, ?, ?)`,
		organization, period, child, week, model)
	require.NoError(t, err)
	return ownBooking{period: period, week: week, registration: registration, child: child}
}

// rebookOwnChild does what the Träger-Büro's rebooking does (#4663): it
// replaces the child's bookings by the time model from to until in the same
// week and writes the change into the registration's trail at at.
func rebookOwnChild(t *testing.T, db *bun.DB, organization int64, booking ownBooking, from, until, at string) {
	t.Helper()
	ctx := context.Background()
	var before, after int64
	require.NoError(t, db.NewRaw(`SELECT holiday_week_time_model_id FROM enrollment.holiday_registration_bookings
		WHERE holiday_registration_child_id = ?`, booking.child).Scan(ctx, &before))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_week_time_models (organization_id, holiday_period_week_id, name, starts_at, ends_at, price_cents)
		VALUES (?, ?, 'Vormittag', ?, ?, 3000) RETURNING id`, organization, booking.week, from, until).Scan(ctx, &after))
	_, err := db.ExecContext(ctx, `DELETE FROM enrollment.holiday_registration_bookings WHERE organization_id = ? AND holiday_registration_child_id = ?`,
		organization, booking.child)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_bookings (organization_id, holiday_period_id,
			holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id) VALUES (?, ?, ?, ?, ?)`,
		organization, booking.period, booking.child, booking.week, after)
	require.NoError(t, err)
	details := fmt.Sprintf(`{"child_id": %d, "before": [{"week_id": %d, "time_model_id": %d}], "after": [{"week_id": %d, "time_model_id": %d}]}`,
		booking.child, booking.week, before, booking.week, after)
	_, err = db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_audit
			(organization_id, holiday_period_id, holiday_registration_id, action, details, occurred_at)
		VALUES (?, ?, ?, 'rebook', ?::jsonb, ?::timestamptz)`, organization, booking.period, booking.registration, details, at)
	require.NoError(t, err)
}

// joinTraeger moves the per-test school into organization for the test, as
// the Träger overrun fixture does.
func joinTraeger(t *testing.T, db *bun.DB, organization, school int64) {
	t.Helper()
	var previous *int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, school).Scan(context.Background(), &previous))
	_, err := db.ExecContext(context.Background(), `UPDATE platform.schools SET organization_id = ? WHERE id = ?`, organization, school)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), `UPDATE platform.schools SET organization_id = ? WHERE id = ?`, previous, school)
		require.NoError(t, err)
	})
}

// A child of the school itself is no holiday guest: its confirmed holiday
// booking at its own school replaces the contract's Soll as well (#5994).
// A received registration books nothing; the contract stays.
func TestStudentCareTimes_OwnChildsHolidayBookingReplacesTheContract(t *testing.T) {
	t.Parallel()
	tc := setupStatisticsRoute(t)
	school := testpkg.Tenant(t)
	_, account := testpkg.CreateTestStaffWithAccountForTenant(t, tc.db, school, "Eigen", "Leitung")
	child := testpkg.CreateTestStudent(t, tc.db, "Mila", "Eigenkind", "2a")
	born := calendar.NewDate(2018, 3, 4)
	_, err := tc.db.ExecContext(context.Background(), `UPDATE users.persons SET birthday = ? WHERE id = ?`, born, child.PersonID)
	require.NoError(t, err)
	device := testpkg.CreateTestDevice(t, tc.db, "own-holiday-device")
	monday := calendar.NewDate(2026, 6, 8)
	insertAttendanceSpan(t, tc.db, school, child.ID, device.ID, monday, 8*time.Hour, 15*time.Hour+45*time.Minute)
	insertAttendanceSpan(t, tc.db, school, child.ID, device.ID, monday.AddDays(7), 8*time.Hour, 15*time.Hour+45*time.Minute)
	organization := testpkg.CreateTestOrganization(t, tc.db, &testpkg.Organization{
		Name: "Träger Eigenkind", Slug: fmt.Sprintf("traeger-eigenkind-%d", testpkg.UniqueTestTenantID(t)), Active: true}).ID
	testpkg.OwnTraegerCareContracts(t, tc.db, organization)
	testpkg.OwnTraegerHolidayPeriods(t, tc.db, organization)
	joinTraeger(t, tc.db, organization, school)
	testpkg.CreateTestCareContract(t, tc.db, organization, school, child.ID, "versendet", calendar.NewDate(2026, 6, 1),
		testpkg.CareContractService{Weekday: "mon", Module: "OGS", EndsAt: "13:00"})
	// The parents spell the name in their own way; case and spaces do not count.
	ownHolidayBooking(t, tc.db, organization, school, " mila ", "EIGENKIND", born, monday, monday.AddDays(4), "08:00", "16:00", "confirmed")
	ownHolidayBooking(t, tc.db, organization, school, "Mila", "Eigenkind", born, monday.AddDays(7), monday.AddDays(7), "08:00", "16:00", "received")

	days := contractCareTimes(t, tc, account.ID, child.ID)
	byDate := map[string]contractCareTimeDayPayload{}
	for _, day := range days {
		byDate[day.Date] = day
	}
	holiday := byDate["2026-06-08"]
	assert.Equal(t, "holiday_booking", holiday.PlannedSource, "the confirmed holiday booking decides the day")
	require.NotNil(t, holiday.PlannedEnd)
	assert.Equal(t, "16:00", *holiday.PlannedEnd, "the contract's 13:00 does not override the holiday booking")
	assert.Equal(t, "within_plan", holiday.Kind)

	received := byDate["2026-06-15"]
	assert.Equal(t, "contract", received.PlannedSource, "a registration that is not confirmed books nothing")
	assert.Equal(t, "overrun", received.Kind)
}

// A rebooking changes the booking from its day on, never a day that has
// passed (#5994): the Soll of a holiday day before the rebooking stays the
// booking the child had then, also once its time model is archived.
func TestStudentCareTimes_RebookingKeepsTheSollOfPastHolidayDays(t *testing.T) {
	t.Parallel()
	tc := setupStatisticsRoute(t)
	school := testpkg.Tenant(t)
	_, account := testpkg.CreateTestStaffWithAccountForTenant(t, tc.db, school, "Umbuchung", "Leitung")
	child := testpkg.CreateTestStudent(t, tc.db, "Rudi", "Umgebucht", "2a")
	born := calendar.NewDate(2018, 5, 6)
	_, err := tc.db.ExecContext(context.Background(), `UPDATE users.persons SET birthday = ? WHERE id = ?`, born, child.PersonID)
	require.NoError(t, err)
	device := testpkg.CreateTestDevice(t, tc.db, "rebooking-device")
	monday := calendar.NewDate(2026, 6, 8)
	insertAttendanceSpan(t, tc.db, school, child.ID, device.ID, monday, 8*time.Hour, 15*time.Hour+45*time.Minute)
	insertAttendanceSpan(t, tc.db, school, child.ID, device.ID, monday.AddDays(3), 8*time.Hour, 12*time.Hour+55*time.Minute)
	organization := testpkg.CreateTestOrganization(t, tc.db, &testpkg.Organization{
		Name: "Träger Umbuchung", Slug: fmt.Sprintf("traeger-umbuchung-%d", testpkg.UniqueTestTenantID(t)), Active: true}).ID
	testpkg.OwnTraegerHolidayPeriods(t, tc.db, organization)
	joinTraeger(t, tc.db, organization, school)
	booking := ownHolidayBooking(t, tc.db, organization, school, "Rudi", "Umgebucht", born, monday, monday.AddDays(4), "08:00", "16:00", "confirmed")
	// On Wednesday the Träger-Büro rebooks the week to the morning until 13:00.
	rebookOwnChild(t, tc.db, organization, booking, "08:00", "13:00", "2026-06-10T09:00:00+02:00")
	// Then it archives both time models of the week: they are no longer
	// offered, but the days booked on them keep reading them.
	_, err = tc.db.ExecContext(context.Background(), `UPDATE enrollment.holiday_week_time_models SET archived_at = NOW()
		WHERE organization_id = ? AND holiday_period_week_id = ?`, organization, booking.week)
	require.NoError(t, err)

	days := contractCareTimes(t, tc, account.ID, child.ID)
	byDate := map[string]contractCareTimeDayPayload{}
	for _, day := range days {
		byDate[day.Date] = day
	}
	past := byDate["2026-06-08"]
	require.NotNil(t, past.PlannedEnd, "Monday before the rebooking stays booked")
	assert.Equal(t, "16:00", *past.PlannedEnd, "the rebooking does not change a day that had passed")
	assert.Equal(t, "within_plan", past.Kind, "a check-out at 15:45 stays within the booking of that day")
	after := byDate["2026-06-11"]
	require.NotNil(t, after.PlannedEnd)
	assert.Equal(t, "13:00", *after.PlannedEnd, "from the rebooking on, the new time model is the Soll")
	assert.Equal(t, "holiday_booking", after.PlannedSource)
}

// The own children of a holiday booking are found in one read of the
// People Directory for all bookings (#5994): two more booked children cost
// no further statement.
func TestCareTimeOverrunsQueryBudgetWithOwnHolidayBookings(t *testing.T) {
	t.Parallel()
	tc := setupStatisticsRoute(t)
	school := testpkg.Tenant(t)
	_, account := testpkg.CreateTestStaffWithAccountForTenant(t, tc.db, school, "Budget", "Ferien")
	claims := claimsFor(t, account.ID)
	organization := testpkg.CreateTestOrganization(t, tc.db, &testpkg.Organization{
		Name: "Träger Ferienbudget", Slug: fmt.Sprintf("traeger-ferienbudget-%d", testpkg.UniqueTestTenantID(t)), Active: true}).ID
	testpkg.OwnTraegerHolidayPeriods(t, tc.db, organization)
	joinTraeger(t, tc.db, organization, school)
	device := testpkg.CreateTestDevice(t, tc.db, "holiday-budget-device")
	monday := calendar.NewDate(2026, 6, 8)
	bookChild := func(first string, born calendar.Date) {
		t.Helper()
		child := testpkg.CreateTestStudent(t, tc.db, first, "Ferienbudget", "2a")
		_, err := tc.db.ExecContext(context.Background(), `UPDATE users.persons SET birthday = ? WHERE id = ?`, born, child.PersonID)
		require.NoError(t, err)
		insertAttendanceSpan(t, tc.db, school, child.ID, device.ID, monday, 8*time.Hour, 16*time.Hour+30*time.Minute)
		ownHolidayBooking(t, tc.db, organization, school, first, "Ferienbudget", born, monday, monday.AddDays(4), "08:00", "16:00", "confirmed")
	}
	bookChild("Anna", calendar.NewDate(2018, 1, 2))

	counter := testpkg.CaptureQueriesForContext(t, tc.db)
	run := func() testpkg.QueryMeasurement {
		t.Helper()
		counter.Reset()
		req := httptest.NewRequest(http.MethodGet, "/care-time-overruns?from=2026-06-08&to=2026-06-12", nil)
		rec := authExec(t, tc, req.WithContext(counter.Context(req.Context())), claims, reportPermissions)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Queries())
	}
	run() // the first list records its access; measure the steady state
	one := run()

	bookChild("Ben", calendar.NewDate(2018, 2, 3))
	bookChild("Cleo", calendar.NewDate(2019, 4, 5))
	three := run()

	assert.Len(t, three.Statements, len(one.Statements), "the own children of all holiday bookings are read in one batch")
	querybudgets.AssertQueryBudget(t, "api.statistics.care_time_overruns_own_holiday_bookings", three)
}

// ownChildForHoliday creates a child of school with a date of birth, a sent
// contract until 13:00 on Mondays and an attended Monday.
func ownChildForHoliday(t *testing.T, tc *statisticsHTTPTestContext, organization, school int64, first string, born calendar.Date, until time.Duration) int64 {
	t.Helper()
	child := testpkg.CreateTestStudent(t, tc.db, first, "Muster", "2a")
	_, err := tc.db.ExecContext(context.Background(), `UPDATE users.persons SET birthday = ? WHERE id = ?`, born, child.PersonID)
	require.NoError(t, err)
	device := testpkg.CreateTestDevice(t, tc.db, "holiday-own-"+first)
	insertAttendanceSpan(t, tc.db, school, child.ID, device.ID, calendar.NewDate(2026, 6, 8), 8*time.Hour, until)
	testpkg.CreateTestCareContract(t, tc.db, organization, school, child.ID, "versendet", calendar.NewDate(2026, 6, 1),
		testpkg.CareContractService{Weekday: "mon", Module: "OGS", EndsAt: "13:00"})
	return child.ID
}

// The membership on the holiday day decides, not today's (#5994): a child
// who left the school after the holidays keeps the Soll of its holiday
// booking for those days.
func TestStudentCareTimes_ChildThatLeftAfterTheHolidaysKeepsItsHolidaySoll(t *testing.T) {
	t.Parallel()
	tc := setupStatisticsRoute(t)
	school := testpkg.Tenant(t)
	_, account := testpkg.CreateTestStaffWithAccountForTenant(t, tc.db, school, "Austritt", "Leitung")
	organization := testpkg.CreateTestOrganization(t, tc.db, &testpkg.Organization{
		Name: "Träger Austritt", Slug: fmt.Sprintf("traeger-austritt-%d", testpkg.UniqueTestTenantID(t)), Active: true}).ID
	testpkg.OwnTraegerCareContracts(t, tc.db, organization)
	testpkg.OwnTraegerHolidayPeriods(t, tc.db, organization)
	joinTraeger(t, tc.db, organization, school)
	born := calendar.NewDate(2018, 7, 8)
	child := ownChildForHoliday(t, tc, organization, school, "Paula", born, 15*time.Hour+45*time.Minute)
	monday := calendar.NewDate(2026, 6, 8)
	ownHolidayBooking(t, tc.db, organization, school, "Paula", "Muster", born, monday, monday.AddDays(4), "08:00", "16:00", "confirmed")
	// Paula left the school at the end of June, before today.
	_, err := tc.db.ExecContext(context.Background(), `UPDATE users.student_school_memberships SET enrolled_until = '2026-06-30'
		WHERE student_profile_id = ?`, child)
	require.NoError(t, err)

	days := contractCareTimes(t, tc, account.ID, child)
	require.NotEmpty(t, days)
	assert.Equal(t, "2026-06-08", days[0].Date)
	assert.Equal(t, "holiday_booking", days[0].PlannedSource, "she belonged to the school on her holiday day")
	require.NotNil(t, days[0].PlannedEnd)
	assert.Equal(t, "16:00", *days[0].PlannedEnd)
	assert.Equal(t, "within_plan", days[0].Kind)
}

// Names compare in Unicode NFC on both sides (#5994): a composed and a
// decomposed "José" with the same date of birth are two children of the
// same name. The booking books neither; both days are not compared, never
// an all-clear against the contract.
func TestStudentCareTimes_HolidayBookingOfTwoNFCEqualChildrenIsNotCompared(t *testing.T) {
	t.Parallel()
	tc := setupStatisticsRoute(t)
	school := testpkg.Tenant(t)
	_, account := testpkg.CreateTestStaffWithAccountForTenant(t, tc.db, school, "Jose", "Leitung")
	organization := testpkg.CreateTestOrganization(t, tc.db, &testpkg.Organization{
		Name: "Träger José", Slug: fmt.Sprintf("traeger-jose-%d", testpkg.UniqueTestTenantID(t)), Active: true}).ID
	testpkg.OwnTraegerCareContracts(t, tc.db, organization)
	testpkg.OwnTraegerHolidayPeriods(t, tc.db, organization)
	joinTraeger(t, tc.db, organization, school)
	born := calendar.NewDate(2018, 9, 10)
	composed := ownChildForHoliday(t, tc, organization, school, "José", born, 12*time.Hour+30*time.Minute)
	decomposed := ownChildForHoliday(t, tc, organization, school, "José", born, 12*time.Hour+30*time.Minute)
	monday := calendar.NewDate(2026, 6, 8)
	ownHolidayBooking(t, tc.db, organization, school, "José", "Muster", born, monday, monday, "08:00", "16:00", "confirmed")

	for _, child := range []int64{composed, decomposed} {
		days := contractCareTimes(t, tc, account.ID, child)
		require.NotEmpty(t, days)
		assert.Equal(t, "2026-06-08", days[0].Date)
		assert.Equal(t, "no_plan", days[0].Kind, "child %d: the booking fits two children, the day is not compared", child)
		assert.Nil(t, days[0].PlannedEnd, "child %d: neither the booking's 16:00 nor the contract's 13:00", child)
	}
}
