// Router-level tests for the hint „Gast aus …“ on a holiday guest of the
// hosting school (#5630, ADR 0120 Nachtrag): the copy carries the name of
// its home school, the reconciliation renews it, GET /api/holiday-guests lists
// it for the guests of today behind students:read, and no other school sees
// them.
package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// badges lists the hints a school's staff member sees, by child.
func (f holidayGuestFixture) badges(t *testing.T, token string) map[int64]string {
	t.Helper()
	rec := f.do(t, token, http.MethodGet, "/api/holiday-guests", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	result := map[int64]string{}
	for _, badge := range decodeSchoolPortalData[[]struct {
		StudentID  int64  `json:"student_id"`
		SchoolName string `json:"school_name"`
	}](t, rec.Body.Bytes()) {
		result[badge.StudentID] = badge.SchoolName
	}
	return result
}

func TestHolidayGuestsNameTheirHomeSchoolInTheSchoolLists(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Feriengäste Heimatschule")
	week := f.offer.Weeks[0].ID // 06.07.2099 to 10.07.2099, hosted by the OGS Nord
	sued := f.offer.Schools[1]
	lead := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead)
	team := f.staffOf(t, f.nord, permissions.StudentsRead)

	var tomRegistration int64
	require.NoError(t, f.db.NewRaw(`SELECT id FROM enrollment.holiday_registrations WHERE id <> ? AND organization_id =
		(SELECT organization_id FROM enrollment.holiday_registrations WHERE id = ?)`, f.siblings, f.siblings).Scan(t.Context(), &tomRegistration))
	f.confirm(t, f.siblings, "confirm")
	f.confirm(t, tomRegistration, "confirm")
	require.Equal(t, http.StatusOK, f.assign(t, lead, week, f.group).Code)

	f.today.Store(calendar.NewDate(2099, 7, 3))
	assert.Empty(t, f.badges(t, team), "before the week no guest has a hint: it is in no list yet")

	f.today.Store(calendar.NewDate(2099, 7, 6))
	f.reconcile(t)
	children := f.roster(t, calendar.NewDate(2099, 7, 6))
	ben, tom := children["Ben Muster"], children["Tom Beispiel"]
	require.NotZero(t, ben)
	require.NotZero(t, tom)
	assert.Equal(t, map[int64]string{ben: "Grundschule Melle", tom: sued.Name}, f.badges(t, team),
		"a child of another school of the Träger names it, a child without one the school its parents named")
	var stored string
	require.NoError(t, f.db.NewRaw(`SELECT home_school_name FROM enrollment.holiday_guests WHERE tenant_id = ? AND student_id = ?`,
		f.nord, tom).Scan(t.Context(), &stored))
	assert.Equal(t, sued.Name, stored, "the copy carries the name of the home school")
	sheet := func(student int64) string {
		t.Helper()
		rec := f.sheet(t, lead, student)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return decodeSchoolPortalData[struct {
			SchoolName string `json:"school_name"`
		}](t, rec.Body.Bytes()).SchoolName
	}
	assert.Equal(t, sued.Name, sheet(tom), "the child file names the home school too")
	assert.Equal(t, "Grundschule Melle", sheet(ben))

	// The Träger renames the OGS Süd, and the parents of Ben take back the
	// school they named: the next reconciliation renews both copies.
	_, err := f.db.ExecContext(t.Context(), `UPDATE platform.schools SET name = 'OGS Süd am Park' WHERE id = ?`, sued.ID)
	require.NoError(t, err)
	_, err = f.db.ExecContext(t.Context(), `UPDATE enrollment.holiday_registration_children SET guest_school_name = ''
		WHERE holiday_registration_id = ? AND first_name = 'Ben'`, f.siblings)
	require.NoError(t, err)
	f.reconcile(t)
	assert.Equal(t, map[int64]string{ben: "", tom: "OGS Süd am Park"}, f.badges(t, team),
		"a change of the school in the registration reaches the copy at the next reconciliation; without a school the hint says Feriengast")

	// A school of another Träger: the database refuses it as a home school,
	// so its name never reaches the hosting school.
	foreign := testpkg.CreateTestOrganization(t, f.db, &testpkg.Organization{
		Name: "Fremder Träger", Slug: fmt.Sprintf("fremder-traeger-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	foreignSchool := testpkg.CreateTestSchoolInOrganization(t, f.db, foreign, "OGS eines fremden Trägers")

	// Other schools see no guest of the OGS Nord, neither of the same Träger
	// nor of another one.
	assert.Empty(t, f.badges(t, f.staffOf(t, sued.ID, permissions.StudentsRead)), "the OGS Süd sees no guest of the OGS Nord")
	assert.Empty(t, f.badges(t, f.staffOf(t, foreignSchool, permissions.StudentsRead)), "a school of another Träger sees no guest")

	// Without the permission of the child list the hints stay closed.
	rec := f.do(t, f.staffOf(t, f.nord), http.MethodGet, "/api/holiday-guests", nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	f.today.Store(calendar.NewDate(2099, 7, 11))
	assert.Empty(t, f.badges(t, team), "from the day after the week no guest has a hint")
}

func TestHolidayGuestBadgesQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Feriengäste Hinweis Budget")
	lead := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead)
	team := f.staffOf(t, f.nord, permissions.StudentsRead)
	read := func() testpkg.QueryMeasurement {
		t.Helper()
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, "/api/holiday-guests", nil)
		req.Header.Set("Authorization", "Bearer "+team)
		rec := httptest.NewRecorder()
		testpkg.MeasuredHTTPRequest(counter, f.router, rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Selects(""))
	}
	f.today.Store(calendar.NewDate(2099, 7, 6))
	none := read()
	var tomRegistration int64
	require.NoError(t, f.db.NewRaw(`SELECT id FROM enrollment.holiday_registrations WHERE id <> ? AND organization_id =
		(SELECT organization_id FROM enrollment.holiday_registrations WHERE id = ?)`, f.siblings, f.siblings).Scan(t.Context(), &tomRegistration))
	f.confirm(t, f.siblings, "confirm")
	f.confirm(t, tomRegistration, "confirm")
	require.Equal(t, http.StatusOK, f.assign(t, lead, f.offer.Weeks[0].ID, f.group).Code)
	require.Len(t, f.badges(t, team), 2)
	assert.Len(t, read().Statements, len(none.Statements), "the hints read the same statements for any number of guests")
	querybudgets.AssertQueryBudget(t, "api.holiday_guests.list", none)
}
