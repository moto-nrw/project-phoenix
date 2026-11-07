package presenceprojectionhttp_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

// planningTimesTables are the six tables behind the two bulk effective-time
// loads (GetBulkEffectiveArrivalTimesForDate / GetBulkEffectivePickupTimesForDate,
// three SELECTs each: exceptions, schedules, notes). Before #2098 a list
// request with both include flags hit every one of them twice: once in
// enrichWithDayPlanning and again in the paginated arrival/pickup enrichment.
var planningTimesTables = []string{
	"student_arrival_exceptions",
	"student_arrival_schedules",
	"student_arrival_notes",
	"student_pickup_exceptions",
	"student_pickup_schedules",
	"student_pickup_notes",
}

// TestListStudentsPlanningTimesQueryBudget is the #2098 acceptance test: one
// list request with include_pickup_times and include_arrival_times runs each
// bulk effective-time SELECT exactly once. Before the fix the paginated
// enrichment stage re-ran the same three SELECTs per kind that
// enrichWithDayPlanning had already issued, so every bucket held 2.
func TestListStudentsPlanningTimesQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc := setupStudentsRoute(t)
	tc.resource.Now = func() time.Time {
		return time.Date(2026, time.June, 1, 10, 0, 0, 0, time.UTC)
	}

	counter := testpkg.CaptureQueries(t, tc.db)
	created := 0
	for _, n := range []int{1, 3, 12} {
		for created < n {
			testpkg.CreateTestStudent(t, tc.db, "TimesBudget", fmt.Sprintf("Kind-%02d", created), "TB1")
			created++
		}
		counter.Reset()
		req := testutil.NewRequest("GET", "/?include_pickup_times=true&include_arrival_times=true&page_size=50", nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())
		require.Len(t, decodeStudentLocationsByID(t, rr.Body.Bytes()), n, "measure the expanded child roster")
		t.Logf("planning times N=%d total=%d", n, counter.Total())
		for _, table := range planningTimesTables {
			queries := counter.Selects(table)
			t.Logf("N=%d table %q: %d queries", n, table, len(queries))
			querybudgets.AssertQueryBudget(t, "api.students.list.planning_times.per_table", queries)
		}
		require.Zero(t, counter.WriteRows())
	}
}
