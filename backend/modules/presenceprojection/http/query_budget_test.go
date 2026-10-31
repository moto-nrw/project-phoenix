package presenceprojectionhttp_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// isSchemaIntrospectionQuery matches queries that touch information_schema
// (or the pg_catalog schema-introspection tables); the input is lowercased SQL.
func isSchemaIntrospectionQuery(query string) bool {
	query = strings.ToLower(query)
	return strings.Contains(query, "information_schema") || strings.Contains(query, "pg_catalog")
}

// TestStudentRequestsNoInformationSchemaQueries is the query-budget guard for
// issue #2059: schema capabilities are fixed at startup (VerifyStudentSchema),
// so no student request may re-detect them per request. The controlled staging
// benchmark measured 5,582 information_schema queries across 8,676 HTTP
// requests before this guard; the budget is ZERO.
func TestStudentRequestsNoInformationSchemaQueries(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc := setupStudentsRoute(t)

	student := testpkg.CreateTestStudent(t, tc.db, "Budget", "Student", "QB1")

	counter := testpkg.CaptureQueries(t, tc.db)

	exec := func(t *testing.T, method, path string, body map[string]interface{}) {
		t.Helper()
		var req *http.Request
		if body != nil {
			req = testutil.NewAuthenticatedRequest(t, method, path, body)
		} else {
			req = testutil.NewRequest(method, path, nil)
		}
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, "%s %s failed. Body: %s", method, path, rr.Body.String())
	}

	// List (the hot path from the benchmark), detail (single-student
	// hydration), and an update that rewrites the departure plan (the write
	// path that previously probed optional columns before every UPDATE).
	exec(t, "GET", "/?page_size=50", nil)
	exec(t, "GET", fmt.Sprintf("/%d", student.ID), nil)
	exec(t, "PUT", fmt.Sprintf("/%d", student.ID), map[string]interface{}{
		"first_name": "Budget",
		"bus_days":   map[string]bool{"mon": true, "fri": true},
	})

	// Student requests must not run schema-detection queries in the request
	// path; resolve schema capabilities at startup instead (#2059).
	querybudgets.AssertQueryBudget(t, "api.students.requests.schema_introspection",
		counter.Matching(isSchemaIntrospectionQuery))
}

// TestListStudentsQueryBudget guards the plain student list (the most
// frequently loaded staff screen) against per-student N+1 regressions: the
// query count must not grow with the number of students, and the total per
// request stays within the registered budget.
func TestListStudentsQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	for _, instant := range []string{
		"2026-10-19T10:00:00Z", // Monday before the clock change.
		"2026-10-20T10:00:00Z",
		"2026-10-21T10:00:00Z",
		"2026-10-22T10:00:00Z",
		"2026-10-23T10:00:00Z",
		"2026-10-24T10:00:00Z",
		"2026-10-25T22:30:00Z", // Last hour of the 25-hour Berlin Sunday.
		"2026-10-25T23:30:00Z", // Monday in Berlin, still Sunday in UTC.
	} {
		t.Run(instant, func(t *testing.T) {
			now, err := time.Parse(time.RFC3339, instant)
			require.NoError(t, err)
			t.Logf("Berlin day: %s", calendar.DateFromTime(now))
			testpkg.SetupIsolatedTestDB(t)
			tc := setupStudentsRoute(t, func() time.Time { return now })

			created := 0
			addStudents := func(n int) {
				for range n {
					testpkg.CreateTestStudent(t, tc.db, "ListBudget", fmt.Sprintf("Kind%d", created), "LB1")
					created++
				}
			}

			counter := testpkg.CaptureQueries(t, tc.db)

			run := func() int {
				counter.Reset()
				req := testutil.NewRequest("GET", "/?page_size=50", nil)
				rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
				require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())
				return counter.Total()
			}

			addStudents(3)
			smallCount := run()

			addStudents(7)
			largeCount := run()

			t.Logf("query budget: 3 students → %d queries, 10 students → %d queries", smallCount, largeCount)

			assert.Equal(t, smallCount, largeCount,
				"query count must be independent of the student count (no per-student N+1)")
			querybudgets.AssertQueryBudget(t, "api.students.list", counter.Queries())
		})
	}
}
