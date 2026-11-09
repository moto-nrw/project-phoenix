//go:build testhooks

package absencetypes_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestAbsenceTypesListQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module, caller, err := api.NewAbsenceTypeTestModule(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	router := chi.NewRouter()
	router.Mount("/api/absence-types", api.NewAbsenceTypesRouterForBudgetTest(module.Administration, caller.Caller().StaffID))
	_, account := testpkg.CreateTestStaffWithAccount(t, db, "List", "Reader")
	claims := testutil.DefaultTestClaims()
	claims.ID = int(account.ID)
	claims.TenantID = testpkg.Tenant(t)
	claims.Permissions = []string{"time_tracking:own"}
	claims.IsAdmin = false
	token := testutil.MintTestJWT(t, claims)
	foreign := testpkg.NewTenantScope(t, db)
	_, err = module.Catalog.CreateAbsenceType(foreign.Context(), workforce.CreateAbsenceType{Name: "Foreign absence type"})
	require.NoError(t, err)

	expected := []map[string]any{}
	counter := testpkg.CaptureQueries(t, db)
	var previousCount int
	for _, size := range []int{0, 3, 8} {
		for len(expected) < size {
			index := len(expected)
			name := fmt.Sprintf("Budget absence %02d", 8-index)
			allowance := index%2 == 0
			carryover := ""
			var carryoverDTO any
			if allowance {
				carryover = "03-31"
				carryoverDTO = carryover
			}
			created, createErr := module.Catalog.CreateAbsenceType(testpkg.Ctx(t), workforce.CreateAbsenceType{
				Name: name, AllowanceEnabled: allowance, CarryoverUntil: carryover,
			})
			require.NoError(t, createErr)
			active := index != 0
			if !active {
				_, updateErr := module.Catalog.UpdateAbsenceType(testpkg.Ctx(t), workforce.UpdateAbsenceType{ID: created.ID, IsActive: &active})
				require.NoError(t, updateErr)
			}
			expected = append([]map[string]any{{
				"id": fmt.Sprint(created.ID), "name": name, "base_type": workforce.AbsenceTypeOther,
				"is_active": active, "allowance_enabled": allowance, "carryover_until": carryoverDTO,
			}}, expected...)
		}
		for _, suffix := range []string{"", "?include_usage=false", "?include_usage=true"} {
			counter.Reset()
			request := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/api/absence-types/"+suffix, nil, testutil.WithJWTBearer(token))
			response := testutil.ExecuteRequestForTest(t, router, request)
			queries := counter.Measurement(counter.Queries())
			encoded, encodeErr := json.Marshal(queries.Statements)
			require.NoError(t, encodeErr)
			t.Logf("absence_types_list n=%d suffix=%q statements=%d queries_json=%s", size, suffix, len(queries.Statements), encoded)
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			require.Equal(t, "application/json", response.Header().Get("Content-Type"))
			want, encodeErr := json.Marshal(map[string]any{"status": "success", "message": "Abwesenheitsarten geladen", "data": expected})
			require.NoError(t, encodeErr)
			require.JSONEq(t, string(want), response.Body.String(), "N=%d: include retired types, exclude other tenants, preserve full ordered DTO and empty array; usage flag is ignored", size)
			require.Zero(t, counter.WriteRows(), "list request must not mutate rows")
			for _, operation := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
				require.Empty(t, counter.Operation(operation), "list must issue no %s", operation)
			}
			require.Empty(t, counter.Matching(func(query string) bool { return strings.Contains(strings.ToUpper(query), "FOR UPDATE") }), "list must take no row locks")
			require.Len(t, counter.Selects("active.staff_absence_types"), 1, "one tenant-scoped bulk read")
			if size != 0 {
				require.Len(t, queries.Statements, previousCount, "query count must not grow with the list")
			}
			previousCount = len(queries.Statements)
			querybudgets.AssertQueryBudget(t, "api.workforce.absence_types.list", queries)
			require.Equal(t, "BEGIN", queries.Statements[0])
			require.Equal(t, fmt.Sprintf("SELECT set_config('role', 'phoenix_tenant', true), set_config('app.current_tenant_id', '%d', true)", testpkg.Tenant(t)), queries.Statements[1])
			require.Equal(t, "COMMIT", queries.Statements[len(queries.Statements)-1])
		}
	}
}
