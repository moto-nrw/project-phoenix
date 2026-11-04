//go:build testhooks

package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
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
	router := api.NewAbsenceTypesRouterForBudgetTest(module.Administration, caller.Caller().StaffID)
	_, account := testpkg.CreateTestStaffWithAccount(t, db, "List", "Reader")
	claims := testutil.DefaultTestClaims()
	claims.ID = int(account.ID)
	claims.TenantID = testpkg.Tenant(t)
	claims.Permissions = []string{permissions.TimeTrackingOwn}
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
			name := fmt.Sprintf("Budget absence %02d", index)
			created, createErr := module.Catalog.CreateAbsenceType(testpkg.Ctx(t), workforce.CreateAbsenceType{Name: name})
			require.NoError(t, createErr)
			active := index != 0
			if !active {
				_, updateErr := module.Catalog.UpdateAbsenceType(testpkg.Ctx(t), workforce.UpdateAbsenceType{ID: created.ID, IsActive: &active})
				require.NoError(t, updateErr)
			}
			expected = append(expected, map[string]any{
				"id": fmt.Sprint(created.ID), "name": name, "base_type": workforce.AbsenceTypeOther,
				"is_active": active, "allowance_enabled": false, "carryover_until": nil,
			})
		}
		counter.Reset()
		request := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/", nil, testutil.WithJWTBearer(token))
		response := testutil.ExecuteRequestForTest(t, router, request)
		queries := counter.Queries()
		encoded, encodeErr := json.Marshal(queries)
		require.NoError(t, encodeErr)
		t.Logf("absence_types_list n=%d statements=%d queries_json=%s", size, len(queries), encoded)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
		want, encodeErr := json.Marshal(expected)
		require.NoError(t, encodeErr)
		require.JSONEq(t, string(want), string(envelope.Data), "N=%d: include retired types, exclude other tenants, preserve order and empty array", size)
		require.Zero(t, counter.WriteRows(), "list request must not mutate rows")
		require.Len(t, counter.Selects("active.staff_absence_types"), 1, "one tenant-scoped bulk read")
		if size != 0 {
			require.Len(t, queries, previousCount, "query count must not grow with the list")
		}
		previousCount = len(queries)
		querybudgets.AssertQueryBudget(t, "api.workforce.absence_types.list", queries)
	}
}
