//go:build testhooks

package shifttypes_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/workforce"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestShiftTypesListQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	types, references, _, err := api.NewShiftTypeTestModule(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	router := api.NewShiftTypesRouterForBudgetTest(types, references)
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
		response := testutil.ExecuteRequestForTest(t, testpkg.MountedQueryBudgetRouter("/api/shift-types", router), request)
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

// TestShiftTypesListWithUsageQueryBudgetIsFlat: the removal dialog's counts
// (#6845) cost one grouped read however many types and shifts exist.
func TestShiftTypesListWithUsageQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	types, references, _, err := api.NewShiftTypeTestModule(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	router := testpkg.MountedQueryBudgetRouter("/api/shift-types", api.NewShiftTypesRouterForBudgetTest(types, references))
	claims := testutil.DefaultTestClaims()
	claims.TenantID = testpkg.Tenant(t)
	claims.Permissions = []string{"time_tracking:manage"}
	claims.IsAdmin = false
	token := testutil.MintTestJWT(t, claims)
	staff := testpkg.CreateTestStaff(t, db, "Budget", "Usage")
	foreign := testpkg.NewTenantScope(t, db)
	_, err = types.CreateShiftType(foreign.Context(), workforce.ShiftTypeInput{Name: "Foreign shift type", Color: "#5080D8"})
	require.NoError(t, err)

	counter := testpkg.CaptureQueries(t, db)
	created := 0
	for _, size := range []int{1, 4} {
		for created < size {
			shiftType, createErr := types.CreateShiftType(testpkg.Ctx(t), workforce.ShiftTypeInput{Name: fmt.Sprintf("Usage shift %02d", created), Color: "#5080D8"})
			require.NoError(t, createErr)
			_, insertErr := db.NewRaw(`
				INSERT INTO schedule.staff_shifts (tenant_id, staff_id, date, start_time, end_time, break_minutes, shift_type_id, created_by)
				VALUES (?, ?, '2026-07-06', ?, ?, 0, ?, ?)
			`, testpkg.Tenant(t), staff.ID, fmt.Sprintf("%02d:00", 6+created), fmt.Sprintf("%02d:30", 6+created), shiftType.ID, staff.ID).Exec(testpkg.Ctx(t))
			require.NoError(t, insertErr)
			created++
		}
		counter.Reset()
		request := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/?with_usage=true", nil, testutil.WithJWTBearer(token))
		response := testutil.ExecuteRequestForTest(t, router, request)
		queries := counter.Measurement(counter.Queries())
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		var envelope struct {
			Data []struct {
				Usage *struct {
					Shifts int `json:"shifts"`
				} `json:"usage"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
		require.Len(t, envelope.Data, size)
		for _, row := range envelope.Data {
			require.NotNil(t, row.Usage)
			require.Equal(t, 1, row.Usage.Shifts)
		}
		require.Zero(t, counter.WriteRows(), "list request must not mutate rows")
		querybudgets.AssertQueryBudget(t, "api.workforce.shift_types.list.with_usage", queries)
	}
}
