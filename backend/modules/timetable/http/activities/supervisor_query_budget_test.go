package httpadapter_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAvailableSupervisorsBySpecializationQueryBudget(t *testing.T) {
	t.Parallel()
	ctx := setupActivitiesRoute(t)
	specialization := fmt.Sprintf("Budget-%d", time.Now().UnixNano())
	add := func(count int) {
		for i := range count {
			teacher := testpkg.CreateTestTeacher(t, ctx.db, fmt.Sprintf("Budget%d", i), "Supervisor")
			_, err := ctx.db.NewUpdate().Model(teacher).ModelTableExpr(`users.teachers AS "teacher"`).
				Set("specialization = ?", specialization).Where("id = ?", teacher.ID).Exec(context.Background())
			require.NoError(t, err)
		}
	}
	add(3)
	counter := testpkg.CaptureQueriesForContext(t, ctx.db)
	run := func() testpkg.QueryMeasurement {
		counter.Reset()
		req := testutil.NewAuthenticatedRequest(t, "GET", "/activities/supervisors/available?specialization="+specialization, nil)
		req = req.WithContext(counter.Context(req.Context()))
		rr := testutil.ExecuteWithAuth(t, testpkg.MountedQueryBudgetRouter("/api", ctx.router), req, testutil.DefaultTestClaims())
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
		return counter.Measurement(counter.Operation("SELECT"))
	}
	small := run()
	add(5)
	large := run()
	assert.Equal(t, len(small.Statements), len(large.Statements))
	querybudgets.AssertQueryBudget(t, "api.activities.available_supervisors.specialization.reads", large)
}
