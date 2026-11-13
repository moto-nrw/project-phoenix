package enrollmenthttp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-ogs/project-phoenix/modules/enrollment/compose"
	enrollmentHTTP "github.com/moto-ogs/project-phoenix/modules/enrollment/http"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
)

// The real schema handlers and administration share one owner read per call.
// Authentication and the tenant transaction envelope are outside this budget.
func TestEnrollmentSchemaReadQueryBudgets(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	owner := enrollmentCompose.New()
	account := testpkg.CreateTestAccount(t, db, "schema-budget")
	service := enrollmentCompose.NewFormSchemas(owner, nil, nil)
	resource := &enrollmentHTTP.Resource{FormSchemaService: service}
	router := resource.Router(func(chi.Router) {})
	principal, err := permissions.NewPrincipal(permissions.PrincipalInput{AccountID: account.ID, TenantID: testpkg.Tenant(t), Permissions: []string{"config:read"}})
	require.NoError(t, err)
	var schemaIDs []int64
	add := func(count, fields int) {
		for range count {
			schema := &enrollment.FormSchema{Name: fmt.Sprintf("Budget-%d", len(schemaIDs)), Version: len(schemaIDs) + 1, IsActive: true, CreatedBy: account.ID,
				Fields: make([]enrollment.FormField, fields),
			}
			for i := range fields {
				schema.Fields[i] = enrollment.FormField{Key: fmt.Sprintf("extra%d", i), Label: "Extra", Type: enrollment.FormFieldText}
			}
			require.NoError(t, testpkg.TenantCommand1(t, testpkg.Ctx(t), db, owner.InsertSchemaVersion, schema))
			schemaIDs = append(schemaIDs, schema.ID)
		}
	}
	counter := testpkg.CaptureQueries(t, db)
	run := func(path string, expectedRows, expectedFields int) testpkg.QueryMeasurement {
		var queries []string
		require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
			req := httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx)
			req = req.WithContext(permissions.WithPrincipal(req.Context(), principal))
			response := httptest.NewRecorder()
			counter.Reset()
			testpkg.MeasuredHTTPRequest(counter, testpkg.MountedQueryBudgetRouter("/api/enrollment", router), response, req)
			queries = counter.Queries()
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			if expectedRows > 0 {
				var body struct {
					Data []enrollmentHTTP.FormSchemaResponse `json:"data"`
				}
				require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
				require.Len(t, body.Data, expectedRows)
			} else {
				var body struct {
					Data enrollmentHTTP.FormSchemaResponse `json:"data"`
				}
				require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
				require.NotEmpty(t, body.Data.ID)
				require.Len(t, body.Data.Fields, expectedFields)
			}
			require.Len(t, counter.Operation("SELECT"), len(queries), "read handlers must not write")
			return nil
		}))
		return counter.Measurement(queries)
	}
	add(3, 3)
	paths := []string{"/schema/", "/schema/versions", fmt.Sprintf("/schema/%d", schemaIDs[0])}
	small := make([]testpkg.QueryMeasurement, len(paths))
	for i, path := range paths {
		rows := 0
		if i == 1 {
			rows = 3
		}
		small[i] = run(path, rows, 3)
	}
	for _, id := range schemaIDs {
		require.NoError(t, testpkg.TenantCommand2(t, testpkg.Ctx(t), db, owner.SetSchemaActive, id, false))
	}
	add(5, 8)
	scenarios := []string{"api.enrollment.schema.active.reads", "api.enrollment.schema.versions.reads", "api.enrollment.schema.by_id.reads"}
	largeQueries := make([]testpkg.QueryMeasurement, len(paths))
	for i, path := range paths {
		rows := 0
		if i == 1 {
			rows = 8
		}
		if i == 2 {
			path = fmt.Sprintf("/schema/%d", schemaIDs[len(schemaIDs)-1])
		}
		large := run(path, rows, 8)
		require.Equal(t, len(small[i].Statements), len(large.Statements), path)
		t.Logf("%s: N3=%d N8=%d", scenarios[i], len(small[i].Statements), len(large.Statements))
		largeQueries[i] = large
	}

	querybudgets.AssertQueryBudget(t, "api.enrollment.schema.active.reads", largeQueries[0])
	querybudgets.AssertQueryBudget(t, "api.enrollment.schema.versions.reads", largeQueries[1])
	querybudgets.AssertQueryBudget(t, "api.enrollment.schema.by_id.reads", largeQueries[2])
}
