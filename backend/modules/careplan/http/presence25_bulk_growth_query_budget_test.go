package careplanhttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestPresence25BulkGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	for _, route := range []struct{ name, path, timeKey, time string }{
		{"arrival_status", "/arrival-schedules/status", "", ""},
		{"arrival_times", "/arrival-times/bulk", "expected_arrival", "08:15"},
		{"pickup_times", "/pickup-times/bulk", "pickup_time", "16:15"},
	} {
		t.Run(route.name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			testpkg.SetupIsolatedTestDB(t)
			tc := setupStudentsRoute(t, fixedCalendarClock)
			router := testutil.NewRouter()
			router.Mount("/api/students", tc.router())
			staff := testpkg.CreateTestStaff(t, tc.db, "Presence25", "Planner")
			ids := make([]int64, 0, 8)
			counter := testpkg.CaptureQueriesForContext(t, tc.db)
			for _, n := range []int{3, 8} {
				for len(ids) < n {
					student := testpkg.CreateTestStudent(t, tc.db, "Presence25", fmt.Sprintf("Child%d", len(ids)), "P25")
					ids = append(ids, student.ID)
					testpkg.CreateTestArrivalSchedule(t, tc.db, student.ID, 1, staff.ID, "08:15")
					testpkg.CreateTestPickupSchedule(t, tc.db, student.ID, 1, staff.ID, "16:15")
				}
				counter.Reset()
				req := testutil.NewAuthenticatedRequest(t, http.MethodPost, "/api/students"+route.path, map[string]any{"student_ids": ids, "date": studentsTestToday.String()})
				req = req.WithContext(counter.Context(req.Context()))
				rr := testutil.ExecuteWithAuthPermissions(t, router, req, testutil.AdminTestClaims(1), []string{"admin:*"})
				require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
				var envelope struct {
					Data json.RawMessage `json:"data"`
				}
				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))
				if route.name == "arrival_status" {
					var result struct {
						StudentIDs []int64 `json:"student_ids"`
					}
					require.NoError(t, json.Unmarshal(envelope.Data, &result))
					require.Equal(t, ids, result.StudentIDs)
				} else {
					var rows []map[string]any
					require.NoError(t, json.Unmarshal(envelope.Data, &rows))
					expected := make([]map[string]any, 0, n)
					for _, id := range ids {
						expected = append(expected, map[string]any{"student_id": float64(id), "date": studentsTestToday.String(), "weekday_name": "Montag", route.timeKey: route.time, "is_exception": false})
					}
					require.Len(t, rows, n)
					require.ElementsMatch(t, expected, rows)
				}
				t.Logf("presence25 POST %s N%d: %d full-route statements", route.path, n, counter.Total())
				assertPresence25BulkBudget(t, route.name, n, counter.Measurement(counter.Queries()))
			}
		})
	}
}

func assertPresence25BulkBudget(t *testing.T, route string, n int, queries querybudgets.Measurement) {
	t.Helper()
	switch fmt.Sprintf("%s.n%d", route, n) {
	case "arrival_status.n3":
		querybudgets.AssertQueryBudget(t, "api.students.presence25.arrival_status.n3", queries)
	case "arrival_status.n8":
		querybudgets.AssertQueryBudget(t, "api.students.presence25.arrival_status.n8", queries)
	case "arrival_times.n3":
		querybudgets.AssertQueryBudget(t, "api.students.presence25.arrival_times.n3", queries)
	case "arrival_times.n8":
		querybudgets.AssertQueryBudget(t, "api.students.presence25.arrival_times.n8", queries)
	case "pickup_times.n3":
		querybudgets.AssertQueryBudget(t, "api.students.presence25.pickup_times.n3", queries)
	case "pickup_times.n8":
		querybudgets.AssertQueryBudget(t, "api.students.presence25.pickup_times.n8", queries)
	default:
		t.Fatalf("unregistered presence25 fixture %s N%d", route, n)
	}
}
