package behaviortest

import (
	"context"
	"fmt"
	"testing"

	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Exercise the merged page and badge with multiple children, not merely
// multiple request types belonging to one child.
func TestRequestReviewQueryBudgetGrowth(t *testing.T) {
	t.Parallel()
	pool := testpkg.SetupTestDB(t)
	WithGrowthQuery(t, func(db *bun.DB, query GrowthQuery) {
		require.Same(t, pool, db)
		for _, n := range []int{3, 8} {
			t.Run(fmt.Sprintf("N%d", n), func(t *testing.T) {
				assertReviewGrowth(t, db, query, n)
			})
		}
	})
}

func assertReviewGrowth(t *testing.T, db *bun.DB, query GrowthQuery, n int) {
	t.Helper()
	testpkg.OwnTenant(t)
	byStudent := make(map[string]testpkg.RequestReviewSchool, n)
	var ctx context.Context
	for i := range n {
		school := testpkg.CreateTestRequestReviewSchool(t, db, fmt.Sprintf("growth-%d-%d", n, i))
		byStudent[fmt.Sprint(school.StudentID)] = school
		ctx = GrowthWriteContext(school.Context)
	}
	counter := testpkg.CaptureQueriesForContext(t, db)
	require.NoError(t, testpkg.WithTenantTx(t, counter.Context(ctx), db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		page, err := query.ListRequests(txCtx, GrowthListQuery{Search: "ReviewRLS", Limit: GrowthMaxLimit})
		require.NoError(t, err)
		require.Len(t, page.Items, 4*n)
		require.Empty(t, page.NextCursor)
		assertReviewGrowthPage(t, page, byStudent)
		if n == 3 {
			querybudgets.AssertQueryBudget(t, "modules.requestreview.open_page.three_children", counter.Measurement(counter.Queries()))
		} else {
			querybudgets.AssertQueryBudget(t, "modules.requestreview.open_page.eight_children", counter.Measurement(counter.Queries()))
		}
		return nil
	}))
	badge := testpkg.CaptureQueriesForContext(t, db)
	require.NoError(t, testpkg.WithTenantTx(t, badge.Context(ctx), db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		count, err := query.PendingCount(txCtx)
		require.NoError(t, err)
		require.Equal(t, 4*n, count)
		if n == 3 {
			querybudgets.AssertQueryBudget(t, "modules.requestreview.pending_count.three_children", badge.Measurement(badge.Queries()))
		} else {
			querybudgets.AssertQueryBudget(t, "modules.requestreview.pending_count.eight_children", badge.Measurement(badge.Queries()))
		}
		return nil
	}))
}

func assertReviewGrowthPage(t *testing.T, page GrowthPage, byStudent map[string]testpkg.RequestReviewSchool) {
	t.Helper()
	counts := make(map[string]int)
	for _, item := range page.Items {
		school, ok := byStudent[item.StudentID]
		require.True(t, ok, "the page must not include another tenant's child")
		require.Equal(t, school.GroupName, item.GroupName)
		counts[item.StudentID+":"+item.RequestType]++
	}
	for id := range byStudent {
		for _, typ := range GrowthRequestTypes() {
			require.Equal(t, 1, counts[id+":"+typ])
		}
	}

}
