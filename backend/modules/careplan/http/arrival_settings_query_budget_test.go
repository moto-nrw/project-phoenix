package careplanhttp_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/settings"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

// The settings list must remain one bulk read when a school maintains more
// lesson end times. Fixture writes are outside each measured HTTP request.
func TestGetArrivalSettingsQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc := setupStudentsRoute(t)
	actor := testpkg.CreateTestAccount(t, tc.db, "arrival-budget@example.test")
	claims := testutil.AdminTestClaims(int(actor.ID))
	counter := testpkg.CaptureQueriesForContext(t, tc.db)

	type period struct {
		Period  int    `json:"period"`
		EndTime string `json:"end_time"`
	}
	var expected []period
	run := func(source, arrival, pickup string) int {
		t.Helper()
		counter.Reset()
		req := testutil.NewRequest(http.MethodGet, "/arrival-settings", nil)
		req = req.WithContext(counter.Context(req.Context()))
		rr := authExec(t, tc, req, claims, []string{"users:read"})
		require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())
		var result struct {
			Data struct {
				CareDaysSource     string   `json:"care_days_source"`
				SchoolPeriods      []period `json:"school_periods"`
				DefaultArrivalTime string   `json:"default_arrival_time"`
				DefaultPickupTime  string   `json:"default_pickup_time"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
		require.NotNil(t, result.Data.SchoolPeriods, "empty lists must serialize as []")
		require.Equal(t, expected, result.Data.SchoolPeriods)
		require.Equal(t, source, result.Data.CareDaysSource)
		require.Equal(t, arrival, result.Data.DefaultArrivalTime)
		require.Equal(t, pickup, result.Data.DefaultPickupTime)
		require.Len(t, counter.Selects("config.setting_values"), 1)
		require.Empty(t, counter.Selects("config.traeger_setting_values"), "these keys are school-only")
		require.Zero(t, counter.WriteRows(), "the settings reader must not write")
		querybudgets.AssertQueryBudget(t, "api.students.arrival_settings", counter.Measurement(counter.Queries()))
		t.Logf("%d maintained periods: %d statements, one settings read, no writes", len(expected), counter.Total())
		return counter.Total()
	}

	expected = make([]period, 0, 8)
	empty := run("weekly_plan", "", "")
	values := []string{"12:01", "12:02", "12:03", "12:04", "12:05", "12:06", "12:07", "12:08"}
	addPeriods := func(start, end int) {
		t.Helper()
		for i := start; i < end; i++ {
			require.NoError(t, tc.settings().SetValue(testpkg.Ctx(t), settings.SchoolPeriodEndKey(i+1), values[i], nil, nil))
			expected = append(expected, period{Period: i + 1, EndTime: values[i]})
		}
	}
	addPeriods(0, 3)
	small := run("weekly_plan", "", "")
	addPeriods(3, 8)
	for key, value := range map[string]any{
		settings.KeyEnrollmentBookingsAuthoritative: true,
		settings.KeyCareDefaultArrivalTime:          "11:30",
		settings.KeyCareDefaultPickupTime:           "17:00",
	} {
		require.NoError(t, tc.settings().SetValue(testpkg.Ctx(t), key, value, nil, nil))
	}
	large := run("bookings", "11:30", "17:00")
	require.Equal(t, empty, small, "statements must not grow from zero to three periods")
	require.Equal(t, small, large, "statements must not grow from three to eight periods")
}
