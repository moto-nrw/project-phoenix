package students_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
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

	testpkg.CreateTestStudent(t, tc.db, "TimesBudget", "KindA", "TB1")
	testpkg.CreateTestStudent(t, tc.db, "TimesBudget", "KindB", "TB1")

	counter := testpkg.CaptureQueries(t, tc.db)

	req := testutil.NewRequest("GET", "/?include_pickup_times=true&include_arrival_times=true&page_size=50", nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

	// Each bulk load must run exactly once per list request (#2098).
	for _, table := range planningTimesTables {
		queries := counter.Selects(table)
		t.Logf("table %q: %d queries", table, len(queries))
		testpkg.AssertQueryBudget(t, "api.students.list.planning_times.per_table", queries)
	}
}
