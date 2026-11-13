package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
)

const homeSchoolChildrenPath = "/api/holiday-weeks/home-school-children"

// tomOfSued is the registration of Tom Beispiel, a child of the OGS Süd
// booked in the first week, which the OGS Nord hosts.
func (f holidayGuestFixture) tomOfSued(t *testing.T) int64 {
	t.Helper()
	var registration int64
	require.NoError(t, f.db.NewRaw(`SELECT holiday_registration_id FROM enrollment.holiday_registration_children
		WHERE holiday_period_id = ? AND first_name = 'Tom'`, f.periodID(t)).Scan(t.Context(), &registration))
	return registration
}

func (f holidayGuestFixture) homeSchoolBookings(t *testing.T, token string) []enrollment.HolidayHomeSchoolBooking {
	t.Helper()
	rec := f.do(t, token, http.MethodGet, homeSchoolChildrenPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[[]enrollment.HolidayHomeSchoolBooking](t, rec.Body.Bytes())
}

// The home school sees the confirmed holiday bookings of its own children in
// weeks another OGS of its Träger hosts (#6679), with the hosting school,
// the week and the booked care, until the last day of the week; nobody else
// sees them.
func TestHomeSchoolSeesTheConfirmedHolidayBookingsOfItsChildrenAtOtherSchools(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Heimatschule")
	sued := f.office.schools[1]
	lead := f.staffOf(t, sued, permissions.ConfigRead)
	tom := f.tomOfSued(t)

	assert.Empty(t, f.homeSchoolBookings(t, lead), "a received registration shows nothing")
	assert.Zero(t, healthAccessCount(t, f, sued, "holiday_home_school_bookings"), "an empty answer is no read")

	f.confirm(t, tom, "confirm")
	entries := f.homeSchoolBookings(t, lead)
	require.Len(t, entries, 1)
	entry := entries[0]
	var nordName string
	require.NoError(t, f.db.NewRaw(`SELECT name FROM platform.schools WHERE id = ?`, f.nord).Scan(t.Context(), &nordName))
	assert.Equal(t, "Tom", entry.FirstName)
	assert.Equal(t, "Beispiel", entry.LastName)
	assert.Equal(t, calendar.NewDate(2017, 5, 20), entry.BirthDate)
	assert.Equal(t, f.offer.Weeks[0].ID, entry.WeekID)
	assert.Equal(t, "Sommerferien 2099", entry.PeriodName)
	assert.Equal(t, calendar.NewDate(2099, 7, 6), entry.StartsOn)
	assert.Equal(t, calendar.NewDate(2099, 7, 10), entry.EndsOn)
	assert.Equal(t, f.nord, entry.HostSchoolID)
	assert.Equal(t, nordName, entry.HostSchoolName, "the hosting school is named")
	assert.Equal(t, "bis 13 Uhr", entry.TimeModelName)
	assert.NotEmpty(t, entry.TimeModelStarts)
	assert.Empty(t, entry.Days, "the whole week is booked")
	assert.Equal(t, enrollment.HolidayOwnChildNotFound, entry.Assignment, "no child of the OGS Süd carries Tom's names and date of birth yet")
	assert.Nil(t, entry.StudentID)
	assert.Equal(t, 1, healthAccessCount(t, f, sued, "holiday_home_school_bookings"), "the read stands in the home school's access log")

	student := testpkg.CreateTestStudentForTenant(t, f.db, sued, "Tom", "Beispiel", "3a")
	_, err := f.db.ExecContext(context.Background(), `UPDATE users.persons SET birthday = '2017-05-20' WHERE id = ?`, student.PersonID)
	require.NoError(t, err)
	entries = f.homeSchoolBookings(t, lead)
	require.Len(t, entries, 1)
	assert.Equal(t, enrollment.HolidayOwnChildMatched, entries[0].Assignment)
	require.NotNil(t, entries[0].StudentID)
	assert.Equal(t, student.ID, *entries[0].StudentID, "the entry links the child's file")

	// The hosting school reads this child in its participant list, not
	// here; its own child Lia is at the week it hosts itself.
	nordLead := f.staffOf(t, f.nord, permissions.ConfigRead)
	f.confirm(t, f.siblings, "confirm")
	assert.Empty(t, f.homeSchoolBookings(t, nordLead), "the OGS Nord has no child at another OGS")
	rec := f.do(t, f.staffOf(t, sued, permissions.UsersRead), http.MethodGet, homeSchoolChildrenPath, nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, "the list needs the right that opens the holiday weeks")

	// After the last day of the week the access ends.
	f.today.Store(calendar.NewDate(2099, 7, 11))
	assert.Empty(t, f.homeSchoolBookings(t, lead), "an ended week shows nothing")
	f.today.Store(calendar.NewDate(2099, 7, 1))

	// A cancellation takes the booking away at once.
	f.confirm(t, tom, "cancel")
	assert.Empty(t, f.homeSchoolBookings(t, lead), "a cancelled booking shows nothing")
}

// TestHomeSchoolBookingsStayWithinTheirQueryBudget pins the statements of
// GET /api/holiday-weeks/home-school-children: flat in the number of
// children and weeks.
func TestHomeSchoolBookingsStayWithinTheirQueryBudget(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Heimatschule Budget")
	sued := f.office.schools[1]
	lead := f.staffOf(t, sued, permissions.ConfigRead)
	f.confirm(t, f.tomOfSued(t), "confirm")
	read := func() testpkg.QueryMeasurement {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, homeSchoolChildrenPath, nil)
		req.Header.Set("Authorization", "Bearer "+lead)
		rec := httptest.NewRecorder()
		testpkg.MeasuredHTTPRequest(counter, f.router, rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), `"first_name":"Tom"`)
		return counter.Measurement(counter.Selects(""))
	}
	one := read()
	// A second child of the OGS Süd in the second week, which the OGS Nord
	// now hosts too: one more child and one more week, no more statements.
	_, err := f.db.ExecContext(t.Context(), `UPDATE enrollment.holiday_period_weeks SET host_school_id = ? WHERE id = ?`, f.nord, f.offer.Weeks[1].ID)
	require.NoError(t, err)
	_, err = f.db.ExecContext(t.Context(), `UPDATE enrollment.holiday_registration_children SET home_school_id = ?, guest_school_name = ''
		WHERE holiday_registration_id = ? AND first_name = 'Ben'`, sued, f.siblings)
	require.NoError(t, err)
	f.confirm(t, f.siblings, "confirm")
	more := read()
	assert.Len(t, more.Statements, len(one.Statements), "the read is flat in the number of children and weeks")
	querybudgets.AssertQueryBudget(t, "api.holiday_weeks.home_school_children", more)
}
