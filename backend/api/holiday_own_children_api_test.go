package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// ownChildDays lists the planned days of a child as
// "date arrival-pickup reason", the dated exceptions the check-in reads.
func (f holidayGuestFixture) ownChildDays(t *testing.T, student int64) []string {
	t.Helper()
	var days []string
	require.NoError(t, f.db.NewRaw(`SELECT a.exception_date::text || ' ' || to_char(a.expected_arrival, 'HH24:MI') || '-' || to_char(p.pickup_time, 'HH24:MI') || ' ' || p.reason
		FROM schedule.student_arrival_exceptions AS a
		JOIN schedule.student_pickup_exceptions AS p ON p.student_id = a.student_id AND p.exception_date = a.exception_date
		WHERE a.student_id = ? ORDER BY a.exception_date`, student).Scan(t.Context(), &days))
	return days
}

func (f holidayGuestFixture) registration(t *testing.T, token string, student int64) []enrollment.HolidayStudentRegistration {
	t.Helper()
	rec := f.do(t, token, http.MethodGet, fmt.Sprintf("/api/holiday-weeks/students/%d/registration", student), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[[]enrollment.HolidayStudentRegistration](t, rec.Body.Bytes())
}

// liaOfNord creates the student of the OGS Nord the siblings' registration
// names: Lia Muster, born on 2018-03-01.
func (f holidayGuestFixture) liaOfNord(t *testing.T, firstName string) int64 {
	t.Helper()
	lia := testpkg.CreateTestStudentForTenant(t, f.db, f.nord, firstName, "Muster", "2b")
	_, err := f.db.ExecContext(context.Background(), `UPDATE users.persons SET birthday = '2018-03-01' WHERE id = ?`, lia.PersonID)
	require.NoError(t, err)
	return lia.ID
}

func TestHostingSchoolPlansTheBookedDaysOfItsOwnChildAndShowsItsRegistration(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Eigene Kinder")
	week := f.offer.Weeks[0].ID
	lead := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead)
	lia := f.liaOfNord(t, " lia ")
	f.confirm(t, f.siblings, "confirm")

	// Before the school picks the holiday group nobody is named as the author
	// of a plan: the registration is readable, the days wait.
	f.reconcile(t)
	assert.Empty(t, f.ownChildDays(t, lia), "without a holiday group no day is planned yet")
	entries := f.registration(t, lead, lia)
	require.Len(t, entries, 1)
	assert.False(t, entries[0].Planned)

	rec := f.assign(t, lead, week, f.group)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	transfer := decodeSchoolPortalData[enrollment.HolidayGuestTransfer](t, rec.Body.Bytes())
	assert.Equal(t, 1, transfer.Guests, "Ben is the holiday guest")
	assert.Equal(t, 1, transfer.OwnChildren, "Lia is the school's own child")
	planned := []string{
		"2099-07-06 08:00-16:00 Ferienbetreuung laut Anmeldung (08:00 bis 16:00 Uhr)",
		"2099-07-07 08:00-16:00 Ferienbetreuung laut Anmeldung (08:00 bis 16:00 Uhr)",
		"2099-07-08 08:00-16:00 Ferienbetreuung laut Anmeldung (08:00 bis 16:00 Uhr)",
		"2099-07-09 08:00-16:00 Ferienbetreuung laut Anmeldung (08:00 bis 16:00 Uhr)",
		"2099-07-10 08:00-16:00 Ferienbetreuung laut Anmeldung (08:00 bis 16:00 Uhr)",
	}
	assert.Equal(t, planned, f.ownChildDays(t, lia), "each booked day expects the child with the times of its time model")
	class, status, _, _, _ := f.child(t, lia)
	assert.Equal(t, "2b", class, "the child stays what it is at its school")
	assert.Equal(t, "active", status)
	assert.NotContains(t, names(f.roster(t, calendar.NewDate(2099, 7, 8))), "lia Muster", "the school's own child does not move into the holiday group")

	f.reconcile(t)
	assert.Equal(t, planned, f.ownChildDays(t, lia), "a second run plans nothing twice")

	entries = f.registration(t, lead, lia)
	require.Len(t, entries, 1)
	entry := entries[0]
	assert.Equal(t, week, entry.WeekID)
	assert.Equal(t, "Sommerferien 2099", entry.PeriodName)
	assert.Equal(t, calendar.NewDate(2099, 7, 6), entry.StartsOn)
	assert.Equal(t, "bis 16 Uhr", entry.TimeModelName)
	assert.Equal(t, "08:00", entry.TimeModelStarts)
	assert.Equal(t, "16:00", entry.TimeModelEnds)
	assert.True(t, entry.Planned)
	require.Len(t, entry.Emergency, 2)
	assert.Equal(t, enrollment.HolidayGuestContact{Name: "Eva Muster", Phone: "0151 2345678", Relationship: "Eltern"}, entry.Emergency[0])
	assert.Equal(t, "Oma Muster", entry.Emergency[1].Name)
	require.Len(t, entry.Pickup, 1)
	assert.Equal(t, "Opa Muster", entry.Pickup[0].Name)
	assert.Equal(t, 2, healthAccessCount(t, f, f.nord, "holiday_student_registration"), "each read with data stands in the access log")

	// The team moves one pickup; the hourly run keeps the team's time,
	// because the booking did not change.
	_, err := f.db.ExecContext(t.Context(), `UPDATE schedule.student_pickup_exceptions SET pickup_time = '14:30'
		WHERE student_id = ? AND exception_date = '2099-07-07'`, lia)
	require.NoError(t, err)
	f.reconcile(t)
	assert.Equal(t, "2099-07-07 08:00-14:30 Ferienbetreuung laut Anmeldung (08:00 bis 16:00 Uhr)", f.ownChildDays(t, lia)[1])

	// Another child and an account without the right to read contacts.
	other := testpkg.CreateTestStudentForTenant(t, f.db, f.nord, "Mila", "Sonne", "2a")
	assert.Empty(t, f.registration(t, lead, other.ID), "a child without a registration shows nothing")
	assert.Equal(t, 2, healthAccessCount(t, f, f.nord, "holiday_student_registration"), "an empty answer is no read")
	reader := f.staffOf(t, f.nord, permissions.ConfigRead)
	rec = f.do(t, reader, http.MethodGet, fmt.Sprintf("/api/holiday-weeks/students/%d/registration", lia), nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, "contacts need the right that opens a child's contacts")
	foreign := f.staffOf(t, f.office.schools[1], permissions.ConfigRead, permissions.UsersRead)
	rec = f.do(t, foreign, http.MethodGet, fmt.Sprintf("/api/holiday-weeks/students/%d/registration", lia), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"data":[]`, "another school reads nothing of this child")

	// A cancellation clears the plan at the next reconciliation.
	f.confirm(t, f.siblings, "cancel")
	f.reconcile(t)
	assert.Empty(t, f.ownChildDays(t, lia), "the cancelled booking leaves no planned day")
	assert.Empty(t, f.registration(t, lead, lia), "and no registration to read")
}

// A child booked in two weeks the school hosts keeps its contacts in each
// week: the contacts belong to the booking of the week, not to the child.
func TestOwnChildBookedInTwoHostedWeeksShowsItsContactsInBoth(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Zwei Wochen")
	lead := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead)
	lia := f.liaOfNord(t, "Lia")
	f.confirm(t, f.siblings, "confirm")
	var liaChild int64
	require.NoError(t, f.db.NewRaw(`SELECT id FROM enrollment.holiday_registration_children WHERE holiday_registration_id = ? AND first_name = 'Lia'`,
		f.siblings).Scan(t.Context(), &liaChild))
	// The OGS Nord hosts the second week too, and Lia is booked in both.
	_, err := f.db.ExecContext(t.Context(), `UPDATE enrollment.holiday_period_weeks SET host_school_id = ? WHERE id = ?`, f.nord, f.offer.Weeks[1].ID)
	require.NoError(t, err)
	bookings := []map[string]any{}
	for _, week := range f.offer.Weeks[:2] {
		bookings = append(bookings, map[string]any{"week_id": week.ID, "time_model_id": week.TimeModels[0].ID})
	}
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, holidayRebookPath(f.periodID(t), f.siblings, liaChild), map[string]any{"bookings": bookings})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	entries := f.registration(t, lead, lia)
	require.Len(t, entries, 2)
	for _, entry := range entries {
		require.Len(t, entry.Emergency, 2, "week %d names the parent and the emergency contact once", entry.WeekID)
		assert.Equal(t, "Eva Muster", entry.Emergency[0].Name)
		require.Len(t, entry.Pickup, 1, "week %d names who may collect the child", entry.WeekID)
		assert.Equal(t, "Opa Muster", entry.Pickup[0].Name)
	}
}

func TestOwnChildPlanLeavesThePlansOfTeamAndParentsAlone(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Eigene Pläne")
	week := f.offer.Weeks[0].ID
	lead := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead)
	lia := f.liaOfNord(t, "Lia")
	f.confirm(t, f.siblings, "confirm")
	require.Equal(t, http.StatusOK, f.assign(t, lead, week, f.group).Code)
	require.Len(t, f.ownChildDays(t, lia), 5)

	// The team replaces Wednesday's pickup by an entry of its own.
	_, err := f.db.ExecContext(t.Context(), `UPDATE schedule.student_pickup_exceptions SET pickup_time = '12:00', reason = 'Zahnarzt'
		WHERE student_id = ? AND exception_date = '2099-07-08'`, lia)
	require.NoError(t, err)
	f.confirm(t, f.siblings, "cancel")
	f.reconcile(t)
	assert.Equal(t, []string{"2099-07-08 Zahnarzt"}, f.pickupReasons(t, lia), "only the days Enrollment planned go; the team's entry stays")
}

func (f holidayGuestFixture) pickupReasons(t *testing.T, student int64) []string {
	t.Helper()
	var rows []string
	require.NoError(t, f.db.NewRaw(`SELECT exception_date::text || ' ' || reason FROM schedule.student_pickup_exceptions
		WHERE student_id = ? ORDER BY exception_date`, student).Scan(t.Context(), &rows))
	return rows
}

func TestParticipantListNamesOwnChildrenThatFitNoStudentOrSeveral(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Offene Kinder")
	week := f.offer.Weeks[0].ID
	lead := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead)
	f.confirm(t, f.siblings, "confirm")

	assignments := func() map[string]enrollment.HolidayOwnChildAssignment {
		rec := f.do(t, lead, http.MethodGet, fmt.Sprintf("/api/holiday-weeks/%d/participants", week), nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		list := decodeSchoolPortalData[struct {
			Participants []struct {
				ChildID   int64  `json:"child_id"`
				FirstName string `json:"first_name"`
			} `json:"participants"`
		}](t, rec.Body.Bytes())
		rec = f.do(t, lead, http.MethodGet, fmt.Sprintf("/api/holiday-weeks/%d/own-children", week), nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		byChild := map[int64]enrollment.HolidayOwnChildAssignment{}
		for _, assignment := range decodeSchoolPortalData[[]enrollment.HolidayOwnChildAssignment](t, rec.Body.Bytes()) {
			byChild[assignment.ChildID] = assignment
		}
		byName := map[string]enrollment.HolidayOwnChildAssignment{}
		for _, participant := range list.Participants {
			byName[participant.FirstName] = byChild[participant.ChildID]
		}
		return byName
	}

	open := assignments()
	assert.Equal(t, enrollment.HolidayOwnChildNotFound, open["Lia"].Assignment, "no student of the school carries Lia's names and date of birth")
	assert.Nil(t, open["Lia"].StudentID)
	assert.Empty(t, open["Ben"].Assignment, "a holiday guest needs no student of the school")

	lia := f.liaOfNord(t, "Lia")
	matched := assignments()
	assert.Equal(t, enrollment.HolidayOwnChildMatched, matched["Lia"].Assignment)
	require.NotNil(t, matched["Lia"].StudentID)
	assert.Equal(t, lia, *matched["Lia"].StudentID)

	twin := f.liaOfNord(t, "LIA")
	assert.Equal(t, enrollment.HolidayOwnChildAmbiguous, assignments()["Lia"].Assignment, "two students fit: the registration is assigned to neither")
	require.Equal(t, http.StatusOK, f.assign(t, lead, week, f.group).Code)
	assert.Empty(t, f.ownChildDays(t, lia), "an ambiguous registration plans no day")
	assert.Empty(t, f.ownChildDays(t, twin))
	assert.Empty(t, f.registration(t, lead, lia), "and shows its contacts at neither child")

	rec := f.do(t, f.staffOf(t, f.office.schools[1], permissions.ConfigRead), http.MethodGet, fmt.Sprintf("/api/holiday-weeks/%d/own-children", week), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"data":[]`, "another school reads nothing of this week")
}

// TestHolidayOwnChildReadsStayWithinTheirQueryBudget pins the statements of
// the three reads this feature adds; each is flat in the number of children.
func TestHolidayOwnChildReadsStayWithinTheirQueryBudget(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Eigene Kinder Budget")
	week := f.offer.Weeks[0].ID
	lead := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead, permissions.StudentsRead)
	lia := f.liaOfNord(t, "Lia")
	f.confirm(t, f.siblings, "confirm")
	require.Equal(t, http.StatusOK, f.assign(t, lead, week, f.group).Code)
	read := func(path string) testpkg.QueryMeasurement {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+lead)
		rec := httptest.NewRecorder()
		testpkg.MeasuredHTTPRequest(counter, f.router, rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Selects(""))
	}
	querybudgets.AssertQueryBudget(t, "api.holiday_weeks.student_registration", read(fmt.Sprintf("/api/holiday-weeks/students/%d/registration", lia)))
	querybudgets.AssertQueryBudget(t, "api.holiday_weeks.own_children", read(fmt.Sprintf("/api/holiday-weeks/%d/own-children", week)))
	querybudgets.AssertQueryBudget(t, "api.holiday_guests.group_hints", read("/api/holiday-guests/groups"))
}

func TestHolidayGroupSaysBeforeItsWeekHowManyGuestsJoinIt(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Gruppenhinweis")
	week := f.offer.Weeks[0].ID
	lead := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead, permissions.StudentsRead)
	hints := func(token string) []enrollment.HolidayGuestGroupHint {
		rec := f.do(t, token, http.MethodGet, "/api/holiday-guests/groups", nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return decodeSchoolPortalData[[]enrollment.HolidayGuestGroupHint](t, rec.Body.Bytes())
	}

	assert.Empty(t, hints(lead), "a group nobody picked for a week carries no hint")
	f.confirm(t, f.siblings, "confirm")
	require.Equal(t, http.StatusOK, f.assign(t, lead, week, f.group).Code)
	assert.Equal(t, []enrollment.HolidayGuestGroupHint{{
		GroupID: f.group, WeekID: week, StartsOn: calendar.NewDate(2099, 7, 6), EndsOn: calendar.NewDate(2099, 7, 10), Guests: 1,
	}}, hints(lead), "before the week the group says how many guests join it and when")

	sued := f.staffOf(t, f.office.schools[1], permissions.StudentsRead)
	assert.Empty(t, hints(sued), "another school sees no hint of this one")
	rec := f.do(t, f.staffOf(t, f.nord, permissions.ConfigRead), http.MethodGet, "/api/holiday-guests/groups", nil)
	assert.Equal(t, http.StatusForbidden, rec.Code)

	f.today.Store(calendar.NewDate(2099, 7, 6))
	assert.Empty(t, hints(lead), "from the first day the guests stand in the group and the hint goes")
}
