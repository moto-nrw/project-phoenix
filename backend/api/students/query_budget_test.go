package students_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// informationSchemaRecorder records every query that touches
// information_schema (or the pg_catalog schema-introspection tables) issued
// through the hooked *bun.DB, including queries inside tenant transactions.
type informationSchemaRecorder struct {
	mu      sync.Mutex
	queries []string
}

func (h *informationSchemaRecorder) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	if isSchemaIntrospectionQuery(event.Query) {
		h.mu.Lock()
		h.queries = append(h.queries, event.Query)
		h.mu.Unlock()
	}
	return ctx
}

func isSchemaIntrospectionQuery(query string) bool {
	query = strings.ToLower(query)
	return strings.Contains(query, "information_schema") || strings.Contains(query, "pg_catalog")
}

func (h *informationSchemaRecorder) AfterQuery(context.Context, *bun.QueryEvent) {}

func (h *informationSchemaRecorder) recorded() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.queries...)
}

func TestInformationSchemaRecorderMatchesSchemaCatalogs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{name: "information schema", query: "SELECT * FROM information_schema.columns", want: true},
		{name: "PostgreSQL catalog", query: "SELECT * FROM pg_catalog.pg_attribute", want: true},
		{name: "quoted PostgreSQL catalog", query: `SELECT * FROM "pg_catalog"."pg_class"`, want: true},
		{name: "application table", query: "SELECT * FROM users.students", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isSchemaIntrospectionQuery(tt.query))
		})
	}
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

	recorder := &informationSchemaRecorder{}
	tc.db.AddQueryHook(recorder)

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

	assert.Empty(t, recorder.recorded(),
		"student requests must not run schema-detection queries (information_schema or pg_catalog) in the request path — resolve schema capabilities at startup instead (#2059)")
}
