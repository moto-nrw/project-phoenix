package active_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestCombinationGroupsQueryBudget guards GET /active/combined/{id}/groups
// against per-mapping N+1 regressions: the combination, its mappings and the
// mapped sessions load in a fixed number of statements however many sessions
// the combination holds (#2940).
func TestCombinationGroupsQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc, router := setupProtectedRouter(t)
	presence := testPresenceQueries(t, tc.db)
	ctx := testpkg.Ctx(t)
	combined, err := presence.RecordCombination(ctx, time.Now(), nil)
	require.NoError(t, err)

	created := 0
	add := func(count int) {
		for range count {
			room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("CombinationBudgetRoom%d", created))
			activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("CombinationBudgetActivity%d", created))
			group := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)
			require.NoError(t, presence.AddGroupToCombination(ctx, combined.ID, group.ID))
			created++
		}
	}

	counter := testpkg.CaptureQueries(t, tc.db)
	run := func(want int) []string {
		counter.Reset()
		req := testutil.NewRequest("GET", fmt.Sprintf("/active/combined/%d/groups", combined.ID), nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, testutil.AdminTestClaims(1), []string{"admin:*"})
		require.Equal(t, testutil.StatusOK, rr.Code, "body: %s", rr.Body.String())
		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, _ := response["data"].([]any)
		require.Len(t, data, want)
		// The tenant transaction's set_config statement is route plumbing, not a read.
		return counter.Matching(func(sqlLower string) bool {
			return strings.HasPrefix(sqlLower, "select") && !strings.Contains(sqlLower, "set_config")
		})
	}

	run(0)
	add(3)
	small := run(3)
	add(5)
	large := run(8)
	assert.Equal(t, len(small), len(large), "query count must be independent of the mapped session count")
	testpkg.AssertQueryBudget(t, "api.active.combination_groups.reads", large)
}
