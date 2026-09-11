package active_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestListActiveGroupsQueryBudget guards GET /active/groups?active=true (the
// list with room + supervisor hydration) against per-group N+1 regressions:
// the query count must not grow with the number of active groups, and the
// total per request stays within the registered budget (#2940).
func TestListActiveGroupsQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc, router := setupProtectedRouter(t)

	created := 0
	addGroups := func(n int) {
		for range n {
			staff := testpkg.CreateTestStaff(t, tc.db, "GroupsBudget", fmt.Sprintf("Supervisor%d", created))
			room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("GroupsBudgetRoom%d", created))
			activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("GroupsBudgetActivity%d", created))
			activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)
			testpkg.CreateTestGroupSupervisor(t, tc.db, staff.ID, activeGroup.ID, "supervisor")
			created++
		}
	}

	counter := testpkg.CaptureQueries(t, tc.db)

	run := func() int {
		counter.Reset()
		req := testutil.NewRequest("GET", "/active/groups?active=true", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, testutil.AdminTestClaims(1), []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())
		return counter.Total()
	}

	addGroups(3)
	smallCount := run()

	addGroups(5)
	largeCount := run()

	t.Logf("query budget: 3 groups → %d queries, 8 groups → %d queries", smallCount, largeCount)

	assert.Equal(t, smallCount, largeCount,
		"query count must be independent of the active group count (no per-group N+1)")
	testpkg.AssertQueryBudget(t, "api.active.groups.list", counter.Queries())
}
