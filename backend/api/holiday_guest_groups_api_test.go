// Router-level tests for the Feriengruppe before its week (ADR 0120,
// Nachtrag 2026-11-11): the daily group lists (navigation and live view of
// the group page) keep back a group that waits for its hosted week and has
// no child in care today, and show it as soon as a child is in care or the
// week has begun. A regular group that is also the Feriengruppe always
// shows. GET /api/holiday-guests/groups names every Feriengruppe with its
// week and guests for the group management, behind groups:read.
package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	groupLiveHTTP "github.com/moto-nrw/project-phoenix/modules/grouplive/http"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

type holidayGuestGroupBody struct {
	GroupID  int64  `json:"group_id"`
	WeekID   int64  `json:"week_id"`
	StartsOn string `json:"starts_on"`
	EndsOn   string `json:"ends_on"`
	Guests   int    `json:"guests"`
}

// groupLiveRouter serves the group page's navigation and live view of the
// fixture's school over the fixture's clock, at noon of its day.
func (f holidayGuestFixture) groupLiveRouter(t *testing.T) http.Handler {
	t.Helper()
	clock := func() time.Time { return f.today.Load().(calendar.Date).BerlinMidnight().Add(12 * time.Hour) }
	feedback, err := NewFeedbackTestModule(testpkg.NewDatabase(t, f.db), testpkg.TenantRuntime(t, f.db))
	require.NoError(t, err)
	_, _, _, _, _, live, err := NewStudentTestModule(testpkg.NewDatabase(t, f.db), testpkg.TenantRuntime(t, f.db), feedback.Feedback, clock)
	require.NoError(t, err)
	unit := testpkg.TenantRuntime(t, f.db)
	students := chi.NewRouter()
	ProtectedTenantRoutes(students, groupLiveHTTP.NewResource(live).Register)
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(tenant.WithUnitOfWork(r.Context(), unit)))
		})
	})
	router.Mount("/api/students", students)
	return testpkg.SessionVerifier(router)
}

// dailyGroups lists the groups of the navigation and of the live view's
// group tabs, which must agree.
func dailyGroups(t *testing.T, router http.Handler, token string) []int64 {
	t.Helper()
	read := func(path string) []int64 {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		type group struct {
			ID int64 `json:"id,string"`
		}
		var groups []group
		if path == "/api/students/ogs-group-navigation" {
			groups = decodeSchoolPortalData[[]group](t, rec.Body.Bytes())
		} else {
			groups = decodeSchoolPortalData[struct {
				Groups []group `json:"groups"`
			}](t, rec.Body.Bytes()).Groups
		}
		ids := make([]int64, 0, len(groups))
		for _, g := range groups {
			ids = append(ids, g.ID)
		}
		return ids
	}
	navigation := read("/api/students/ogs-group-navigation")
	assert.ElementsMatch(t, navigation, read("/api/students/ogs-group-live"), "navigation and group tabs list the same groups")
	return navigation
}

func (f holidayGuestFixture) guestGroups(t *testing.T, token string) []holidayGuestGroupBody {
	t.Helper()
	rec := f.do(t, token, http.MethodGet, "/api/holiday-guests/groups", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[[]holidayGuestGroupBody](t, rec.Body.Bytes())
}

func TestFeriengruppeWaitsOutsideTheDailyListsUntilItsWeek(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Feriengruppe vor der Woche")
	week := f.offer.Weeks[0] // 06.07.2099 to 10.07.2099, hosted by the OGS Nord
	lead := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead,
		permissions.GroupsRead, permissions.StudentsRead, "admin:*")
	live := f.groupLiveRouter(t)

	regular := testpkg.CreateTestEducationGroupForTenant(t, f.db, f.nord, "Bären").ID
	child := testpkg.CreateTestStudentForTenant(t, f.db, f.nord, "Mia", "Bär", "2a")
	testpkg.AssignStudentToGroup(t, f.db, child.ID, regular)

	f.confirm(t, f.siblings, "confirm")
	f.today.Store(calendar.NewDate(2099, 7, 1))
	require.Equal(t, http.StatusOK, f.assign(t, lead, week.ID, f.group).Code)

	assert.Equal(t, []holidayGuestGroupBody{{GroupID: f.group, WeekID: week.ID, StartsOn: "2099-07-06", EndsOn: "2099-07-10", Guests: 1}},
		f.guestGroups(t, lead), "the group management names the week and the guests waiting for it")
	assert.Equal(t, []int64{regular}, dailyGroups(t, live, lead),
		"before its week the empty Feriengruppe stays out of the daily lists")

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/students/ogs-group-live?group_id=%d", f.group), nil)
	req.Header.Set("Authorization", "Bearer "+lead)
	rec := httptest.NewRecorder()
	live.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code, "a link to the Feriengruppe still opens it: %s", rec.Body.String())

	f.today.Store(calendar.NewDate(2099, 7, 6))
	f.reconcile(t)
	assert.ElementsMatch(t, []int64{regular, f.group}, dailyGroups(t, live, lead),
		"from the first day of the week the Feriengruppe is a daily group")

	f.today.Store(calendar.NewDate(2099, 7, 11))
	assert.Empty(t, f.guestGroups(t, lead), "after its last week the group is no Feriengruppe")
	assert.ElementsMatch(t, []int64{regular, f.group}, dailyGroups(t, live, lead),
		"after its last week the group is an ordinary group again")
}

func TestFeriengruppeWithAChildInCareTodayShowsBeforeItsWeek(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Feriengruppe mit Kind")
	lead := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead,
		permissions.GroupsRead, permissions.StudentsRead, "admin:*")
	live := f.groupLiveRouter(t)

	f.today.Store(calendar.NewDate(2099, 7, 1))
	require.Equal(t, http.StatusOK, f.assign(t, lead, f.offer.Weeks[0].ID, f.group).Code)
	assert.Empty(t, dailyGroups(t, live, lead), "the waiting Feriengruppe is the only group and stays back")

	child := testpkg.CreateTestStudentForTenant(t, f.db, f.nord, "Lea", "Früh", "3b")
	testpkg.AssignStudentToGroup(t, f.db, child.ID, f.group)
	assert.Equal(t, []int64{f.group}, dailyGroups(t, live, lead),
		"a regular group that is also the Feriengruppe always shows: its child is in care today")
}

func TestHolidayGuestGroupsStayInTheirSchool(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Feriengruppe Zugriff")
	lead := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead)
	f.today.Store(calendar.NewDate(2099, 7, 1))
	require.Equal(t, http.StatusOK, f.assign(t, lead, f.offer.Weeks[0].ID, f.group).Code)

	rec := f.do(t, f.staffOf(t, f.nord, permissions.StudentsRead), http.MethodGet, "/api/holiday-guests/groups", nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, "the Feriengruppen open with the permission of the group lists: %s", rec.Body.String())
	assert.Len(t, f.guestGroups(t, f.staffOf(t, f.nord, permissions.GroupsRead)), 1)
	assert.Empty(t, f.guestGroups(t, f.staffOf(t, f.offer.Schools[1].ID, permissions.GroupsRead)),
		"another school of the Träger sees no Feriengruppe of the OGS Nord")
}

func TestHolidayGuestGroupsQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Feriengruppe Budget")
	lead := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead)
	team := f.staffOf(t, f.nord, permissions.GroupsRead)
	read := func() testpkg.QueryMeasurement {
		t.Helper()
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, "/api/holiday-guests/groups", nil)
		req.Header.Set("Authorization", "Bearer "+team)
		rec := httptest.NewRecorder()
		testpkg.MeasuredHTTPRequest(counter, f.router, rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Selects(""))
	}
	f.today.Store(calendar.NewDate(2099, 7, 1))
	none := read()
	f.confirm(t, f.siblings, "confirm")
	require.Equal(t, http.StatusOK, f.assign(t, lead, f.offer.Weeks[0].ID, f.group).Code)
	require.Len(t, f.guestGroups(t, team), 1)
	assert.Len(t, read().Statements, len(none.Statements), "the Feriengruppen read the same statements with or without weeks and guests")
	querybudgets.AssertQueryBudget(t, "api.holiday_guests.groups", none)
}
