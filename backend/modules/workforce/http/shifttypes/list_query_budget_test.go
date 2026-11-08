//go:build testhooks

package shifttypes_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestShiftTypesListQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	types, _, err := api.NewShiftTypeTestModule(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	router := api.NewShiftTypesRouterForBudgetTest(types)
	claims := testutil.DefaultTestClaims()
	claims.TenantID = testpkg.Tenant(t)
	claims.Permissions = []string{"time_tracking:manage"}
	claims.IsAdmin = false
	token := testutil.MintTestJWT(t, claims)
	foreign := testpkg.NewTenantScope(t, db)
	_, err = types.CreateShiftType(foreign.Context(), workforce.ShiftTypeInput{Name: "Foreign shift type", Color: "#5080D8"})
	require.NoError(t, err)

	expected := []map[string]any{}
	counter := testpkg.CaptureQueries(t, db)
	var previousCount int
	for _, size := range []int{0, 3, 8} {
		for len(expected) < size {
			index := len(expected)
			name := fmt.Sprintf("Budget shift %02d", 8-index)
			active := index != 0
			description := ""
			if index%2 == 0 {
				description = fmt.Sprintf("Shift description %d", index)
			}
			created, createErr := types.CreateShiftType(testpkg.Ctx(t), workforce.ShiftTypeInput{
				Name: name, Color: "#5080D8", Description: description, IsActive: &active,
			})
			require.NoError(t, createErr)
			row := map[string]any{"id": created.ID, "name": name, "color": "#5080D8", "is_active": active}
			if description != "" {
				row["description"] = description
			}
			expected = append([]map[string]any{row}, expected...)
		}
		counter.Reset()
		request := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/", nil, testutil.WithJWTBearer(token))
		response := testutil.ExecuteRequestForTest(t, router, request)
		queries := counter.Measurement(counter.Queries())
		encoded, encodeErr := json.Marshal(queries.Statements)
		require.NoError(t, encodeErr)
		t.Logf("shift_types_list n=%d statements=%d queries_json=%s", size, len(queries.Statements), encoded)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
		want, encodeErr := json.Marshal(expected)
		require.NoError(t, encodeErr)
		require.JSONEq(t, string(want), string(envelope.Data), "N=%d: preserve DTO, name order, retired types and tenant scope", size)
		require.Zero(t, counter.WriteRows(), "list request must not mutate rows")
		for _, operation := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
			require.Empty(t, counter.Operation(operation), "list must issue no %s", operation)
		}
		require.Len(t, counter.Selects("schedule.shift_types"), 1, "one tenant-scoped bulk read")
		if size != 0 {
			require.Len(t, queries.Statements, previousCount, "query count must not grow with the list")
		}
		querybudgets.AssertQueryBudget(t, "api.workforce.shift_types.list", queries)
		previousCount = len(queries.Statements)
	}
}
