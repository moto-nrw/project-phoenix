package timetablehttp

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// TestListInstancesQueryBudget guards GET /instances over a week against
// per-instance N+1 regressions: the query count must not grow with the number
// of instances (each carrying a staff assignment and an enrolled student), and
// the total per request stays within the registered budget (#2940).
func TestListInstancesQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	s := buildListSetup(t)
	rows, err := RootCompositions.Rows(s.db)
	require.NoError(t, err)
	s.res.BlockGroups = rows.Timetable

	from, fromDate := listFutureDate(1)
	to, _ := listFutureDate(7)
	educationGroup := testpkg.CreateTestEducationGroup(t, s.db, "Budget-Gruppe")
	template := testpkg.CreateTestActivityGroup(t, s.db, "Budget-Vorlage")
	_, err = s.db.NewUpdate().
		TableExpr("activities.groups").
		Set("is_template = TRUE").
		Set("education_group_id = ?", educationGroup.ID).
		Where("id = ?", template.ID).
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Exec(s.ctx)
	require.NoError(t, err)

	// Pickup cutoffs are read once per distinct date (bounded by the range
	// cap), so both runs spread their instances over the same three days and
	// the count must stay flat as instances per day grow.
	created := 0
	addInstances := func(n int) {
		for range n {
			startHour := 13 + created/3
			inst := testpkg.CreateTestActivityInstance(t, s.db, fromDate.AddDays(created%3), s.roomID, testpkg.ActivityInstanceOpts{
				ActivityGroupID: &template.ID,
				StartHHMM:       fmt.Sprintf("%02d:00", startHour),
				EndHHMM:         fmt.Sprintf("%02d:00", startHour+1),
				Title:           fmt.Sprintf("Budget-Block-%d", created),
			})
			staff := testpkg.CreateTestStaff(t, s.db, "InstancesBudget", fmt.Sprintf("Staff%d", created))
			student := testpkg.CreateTestStudent(t, s.db, "InstancesBudget", fmt.Sprintf("Kind%d", created), "1a")
			testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, staff.ID, testpkg.InstanceStaffOpts{IsPrimary: true})
			testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, student.ID, timetable.SlotAttendanceExpected)
			created++
		}
	}

	counter := testpkg.CaptureQueries(t, s.db)
	router := testpkg.MeasuredTenantRequest(t, s.db, testpkg.TenantIDFromContext(s.ctx), counter, listRouter(s.ctx, s.res))

	run := func() int {
		counter.Reset()
		w := doList(t, router, fmt.Sprintf("/instances?from=%s&to=%s", from, to))
		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		require.Len(t, decodeList(t, w).Instances, created)
		return counter.Total()
	}

	addInstances(3)
	smallCount := run()
	filteredSmallCount := func() int {
		counter.Reset()
		w := doList(t, router, fmt.Sprintf("/instances?from=%s&to=%s&group_id=%d", from, to, educationGroup.ID))
		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		require.Len(t, decodeList(t, w).Instances, created)
		return counter.Total()
	}()

	addInstances(5)
	largeCount := run()
	querybudgets.AssertQueryBudget(t, "api.timetable.instances.list", counter.Queries())
	filteredLargeCount := func() int {
		counter.Reset()
		w := doList(t, router, fmt.Sprintf("/instances?from=%s&to=%s&group_id=%d", from, to, educationGroup.ID))
		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		require.Len(t, decodeList(t, w).Instances, created)
		return counter.Total()
	}()

	t.Logf("query budget: unfiltered 3 → %d, 8 → %d; group filter 3 → %d, 8 → %d", smallCount, largeCount, filteredSmallCount, filteredLargeCount)

	assert.Equal(t, smallCount, largeCount,
		"query count must be independent of the instance count (no per-instance N+1)")
	assert.Equal(t, filteredSmallCount, filteredLargeCount,
		"group-filter query count must be independent of the instance count")
	querybudgets.AssertQueryBudget(t, "api.timetable.instances.list.group_filter", counter.Queries())
}
