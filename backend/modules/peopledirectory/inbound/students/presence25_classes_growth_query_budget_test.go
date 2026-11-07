package students_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestPresence25ClassesGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc := setupStudentsRoute(t)
	router := testutil.NewRouter()
	router.Mount("/api/students", tc.router())
	expected := make([]string, 0, 8)
	counter := testpkg.CaptureQueriesForContext(t, tc.db)
	for _, n := range []int{3, 8} {
		for len(expected) < n {
			class := fmt.Sprintf("P%d", len(expected)+1)
			testpkg.CreateTestStudent(t, tc.db, "Presence25", class, class)
			expected = append(expected, class)
		}
		counter.Reset()
		req := testutil.NewRequest(http.MethodGet, "/api/students/school-classes", nil)
		req = req.WithContext(counter.Context(req.Context()))
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, testutil.AdminTestClaims(1), []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var result struct {
			Data []string `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
		require.Equal(t, expected, result.Data)
		t.Logf("presence25 school-classes N%d: %d full-route statements", n, counter.Total())
		if n == 3 {
			querybudgets.AssertQueryBudget(t, "api.students.presence25.school_classes.n3", counter.Queries())
		} else {
			querybudgets.AssertQueryBudget(t, "api.students.presence25.school_classes.n8", counter.Queries())
		}
	}
}
