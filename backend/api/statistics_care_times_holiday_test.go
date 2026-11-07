package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
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
