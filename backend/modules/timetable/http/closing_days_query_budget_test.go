package timetablehttp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	rootAPI "github.com/moto-nrw/project-phoenix/api"
	apiTest "github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	timetableAPI "github.com/moto-nrw/project-phoenix/modules/timetable/http"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestClosingDaysRouteGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db, _, periods := setupTimetableModule(t)
	closing, ok := periods.(timetableAPI.ClosingDays)
	require.True(t, ok)
	settings, err := owners.NewSettingsTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	resource := timetableAPI.NewResource(timetableAPI.Dependencies{ClosingDays: closing, SettingsService: settings.Settings})
	router := chi.NewRouter()
	router.Mount("/timetable", resource.Router(rootAPI.ProtectedTenantRoutes))
	account := testpkg.CreateTestAccount(t, db, "closing-growth")
	counter := testpkg.CaptureQueriesForContext(t, db)
	created, small := 0, 0
	wantIDs := []int64{}
	for _, n := range []int{3, 8} {
		for created < n {
			date := calendar.NewDate(2027, 1, 1).AddDays(created * 3)
			day := testpkg.CreateTestClosingDay(t, db, date, date, fmt.Sprintf("Growth %d", created))
			wantIDs = append(wantIDs, day.ID)
			created++
		}
		counter.Reset()
		request := httptest.NewRequestWithContext(counter.Context(context.Background()), http.MethodGet, "/timetable/closing-days/", nil)
		response := apiTest.ExecuteWithAuth(t, router, request, apiTest.TenantUserTestClaims(int(account.ID), testpkg.Tenant(t), permissions.SchedulesRead))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		var envelope struct {
			Data []timetableAPI.ClosingDayResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
		require.Len(t, envelope.Data, n)
		gotIDs := []int64{}
		for i, day := range envelope.Data {
			gotIDs = append(gotIDs, day.ID)
			require.Equal(t, calendar.NewDate(2027, 1, 1).AddDays(i*3).String(), day.StartDate)
			require.Equal(t, day.StartDate, day.EndDate)
			require.Equal(t, fmt.Sprintf("Growth %d", i), day.Reason)
		}
		require.ElementsMatch(t, wantIDs, gotIDs)
		require.Zero(t, counter.WriteRows())
		if n == 3 {
			small = counter.Total()
		} else {
			require.Equal(t, small, counter.Total())
		}
		t.Logf("closing days growth N%d statements=%d", n, counter.Total())
		querybudgets.AssertQueryBudget(t, "api.timetable.closing_days.list", counter.Queries())
	}
	foreign := testpkg.NewTenantScope(t, db)
	request := httptest.NewRequest(http.MethodGet, "/timetable/closing-days/", nil)
	response := apiTest.ExecuteWithAuth(t, router, request, apiTest.TenantUserTestClaims(int(account.ID), foreign.TenantID, permissions.SchedulesRead))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var empty struct {
		Data []timetableAPI.ClosingDayResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &empty))
	require.Empty(t, empty.Data, "foreign school must not see closing days")
}
