package timetablehttp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	rootAPI "github.com/moto-nrw/project-phoenix/api"
	apiTest "github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	timetableAPI "github.com/moto-nrw/project-phoenix/modules/timetable/http"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

type holidayRouteSetup struct {
	db       *bun.DB
	router   chi.Router
	tenantID int64
}

// setupHolidayRoute mounts GET /timetable/holidays over the real School
// Calendar and the real settings, so the federal state comes from the
// tenant's operations.federal_state like in production (#4775).
func setupHolidayRoute(t *testing.T) holidayRouteSetup {
	t.Helper()
	db, _, schoolCalendar := setupTimetableModule(t)
	holidays, ok := schoolCalendar.(timetableAPI.Holidays)
	require.True(t, ok, "the School Calendar serves the holiday list")
	schoolSettings, err := owners.NewSettingsTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	resource := timetableAPI.NewResource(timetableAPI.Dependencies{
		Holidays:        holidays,
		SettingsService: schoolSettings.Settings,
	})
	router := chi.NewRouter()
	router.Mount("/timetable", resource.Router(rootAPI.ProtectedTenantRoutes))
	return holidayRouteSetup{db: db, router: router, tenantID: tenant.FromContext(testpkg.Ctx(t))}
}

type holidayListBody struct {
	Data struct {
		FederalState     string `json:"federal_state"`
		FederalStateName string `json:"federal_state_name"`
		Holidays         []struct {
			Date string `json:"date"`
			Name string `json:"name"`
		} `json:"holidays"`
	} `json:"data"`
}

func getHolidays(t *testing.T, setup holidayRouteSetup, tenantID int64, query string, granted ...string) *httptest.ResponseRecorder {
	return getHolidaysWithContext(t, setup, context.Background(), tenantID, query, granted...)
}

func getHolidaysWithContext(t *testing.T, setup holidayRouteSetup, ctx context.Context, tenantID int64, query string, granted ...string) *httptest.ResponseRecorder {
	t.Helper()
	account := testpkg.CreateTestAccount(t, setup.db, fmt.Sprintf("holidays-%d", time.Now().UnixNano()))
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/timetable/holidays"+query, nil)
	return apiTest.ExecuteWithAuth(t, setup.router, req, apiTest.TenantUserTestClaims(int(account.ID), tenantID, granted...))
}

func decodeHolidayList(t *testing.T, response *httptest.ResponseRecorder) holidayListBody {
	t.Helper()
	require.Equal(t, http.StatusOK, response.Code, "body=%s", response.Body.String())
	var body holidayListBody
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	return body
}

func holidayNames(body holidayListBody) []string {
	names := make([]string, 0, len(body.Data.Holidays))
	for _, holiday := range body.Data.Holidays {
		names = append(names, holiday.Name)
	}
	return names
}

// Bayern celebrates Heilige Drei Könige, Nordrhein-Westfalen does not; each
// school sees the holidays of its own federal state only.
func TestHolidayRouteFollowsTheFederalStateOfEachSchool(t *testing.T) {
	t.Parallel()
	setup := setupHolidayRoute(t)
	bavaria, _ := testpkg.CreateTestTenant(t, setup.db)
	testpkg.SetTestTenantSetting(t, setup.db, bavaria, "operations.federal_state", `"DE-BY"`)

	nrw := decodeHolidayList(t, getHolidays(t, setup, setup.tenantID, "?from=2027-01-01&to=2027-01-31", permissions.SchedulesRead))
	assert.Equal(t, "DE-NW", nrw.Data.FederalState, "the registry default applies without an override")
	assert.Equal(t, "Nordrhein-Westfalen", nrw.Data.FederalStateName)
	assert.Contains(t, holidayNames(nrw), "Neujahrstag")
	assert.NotContains(t, holidayNames(nrw), "Heilige Drei Könige")

	by := decodeHolidayList(t, getHolidays(t, setup, bavaria, "?from=2027-01-01&to=2027-01-31", permissions.SchedulesRead))
	assert.Equal(t, "DE-BY", by.Data.FederalState)
	assert.Equal(t, "Bayern", by.Data.FederalStateName)
	assert.Contains(t, holidayNames(by), "Heilige Drei Könige")
	require.Len(t, by.Data.Holidays, 2)
	assert.Equal(t, "2027-01-06", by.Data.Holidays[1].Date)
}

// The calendar page reads the holidays with calendar:own, the period page
// with schedules:read; neither needs a time-tracking permission. A role
// with none of them gets the registered forbidden code.
func TestHolidayRouteNeedsACalendarReadPermission(t *testing.T) {
	t.Parallel()
	setup := setupHolidayRoute(t)
	const window = "?from=2026-12-01&to=2026-12-31"

	for _, granted := range []string{permissions.CalendarOwn, permissions.SchedulesRead} {
		body := decodeHolidayList(t, getHolidays(t, setup, setup.tenantID, window, granted))
		assert.Contains(t, holidayNames(body), "Erster Weihnachtsfeiertag", granted)
	}

	for _, granted := range [][]string{{permissions.TimeTrackingOwn}, {}} {
		response := getHolidays(t, setup, setup.tenantID, window, granted...)
		require.Equal(t, http.StatusForbidden, response.Code, "permissions %v: body=%s", granted, response.Body.String())
		var problem struct {
			Code string `json:"code"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &problem))
		assert.Equal(t, "general.permission", problem.Code, "the registered code of a missing permission")
	}
}

func TestHolidayRouteRejectsAMissingOrTooWideWindow(t *testing.T) {
	t.Parallel()
	setup := setupHolidayRoute(t)

	for _, query := range []string{"", "?from=2026-01-01", "?from=2026-02-01&to=2026-01-01", "?from=2026-01-01&to=2027-12-31"} {
		response := getHolidays(t, setup, setup.tenantID, query, permissions.CalendarOwn)
		assert.Equal(t, http.StatusBadRequest, response.Code, "query %q", query)
	}
}

// The holiday calculation is local. One settings snapshot is the only owner
// read, so the complete request must remain flat as the holiday list grows.
func TestHolidayRouteQueryBudget(t *testing.T) {
	t.Parallel()
	setup := setupHolidayRoute(t)
	counter := testpkg.CaptureQueriesForContext(t, setup.db)

	response := getHolidaysWithContext(
		t,
		setup,
		counter.Context(context.Background()),
		setup.tenantID,
		"?from=2026-08-01&to=2027-07-31",
		permissions.CalendarOwn,
	)
	decodeHolidayList(t, response)
	t.Logf("query budget: holiday list → %d queries", counter.Total())
	querybudgets.AssertQueryBudget(t, "api.timetable.holidays.list", counter.Queries())
}
