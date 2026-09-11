package timetable

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestListInstancesQueryBudget guards GET /instances over a week against
// per-instance N+1 regressions: the query count must not grow with the number
// of instances (each carrying a staff assignment and an enrolled student), and
// the total per request stays within the registered budget (#2940).
func TestListInstancesQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	s := buildListSetup(t)

	from, fromDate := listFutureDate(1)
	to, _ := listFutureDate(7)

	// Pickup cutoffs are read once per distinct date (bounded by the range
	// cap), so both runs spread their instances over the same three days and
	// the count must stay flat as instances per day grow.
	created := 0
	addInstances := func(n int) {
		for range n {
			inst := testpkg.CreateTestActivityInstance(t, s.db, fromDate.AddDays(created%3), s.roomID, testpkg.ActivityInstanceOpts{
				StartHHMM: "13:00", EndHHMM: "14:00", Title: fmt.Sprintf("Budget-Block-%d", created),
			})
			staff := testpkg.CreateTestStaff(t, s.db, "InstancesBudget", fmt.Sprintf("Staff%d", created))
			student := testpkg.CreateTestStudent(t, s.db, "InstancesBudget", fmt.Sprintf("Kind%d", created), "1a")
			testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, staff.ID, testpkg.InstanceStaffOpts{IsPrimary: true})
			testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, student.ID, schedule.AttendanceStatusExpected)
			created++
		}
	}

	counter := testpkg.CaptureQueries(t, s.db)
	router := listRouter(s.ctx, s.res)

	run := func() int {
		counter.Reset()
		w := doList(t, router, fmt.Sprintf("/instances?from=%s&to=%s", from, to))
		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		require.Len(t, decodeList(t, w).Instances, created)
		return counter.Total()
	}

	addInstances(3)
	smallCount := run()

	addInstances(5)
	largeCount := run()

	t.Logf("query budget: 3 instances → %d queries, 8 instances → %d queries", smallCount, largeCount)

	assert.Equal(t, smallCount, largeCount,
		"query count must be independent of the instance count (no per-instance N+1)")
	testpkg.AssertQueryBudget(t, "api.timetable.instances.list", counter.Queries())
}
