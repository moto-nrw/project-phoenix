//go:build testhooks

package worktimemodels_test

import (
	"context"
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

func TestWorkTimeModelsListQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	models := api.NewAbsenceTypeTestCapability(testpkg.NewDatabase(t, db))
	notifications := 0
	router := api.NewWorkTimeModelsRouterForBudgetTest(models, func(context.Context) { notifications++ })
	claims := testutil.DefaultTestClaims()
	claims.TenantID = testpkg.Tenant(t)
	claims.Permissions = []string{"time_tracking:manage"}
	claims.IsAdmin = false
	token := testutil.MintTestJWT(t, claims)
	foreign := testpkg.NewTenantScope(t, db)
	_, err := models.CreateWorkTimeModel(foreign.Context(), workforce.CreateWorkTimeModel{WorkTimeModelFields: budgetWorkTimeModelFields("Foreign model", 1)})
	require.NoError(t, err)

	expected := []map[string]any{}
	counter := testpkg.CaptureQueries(t, db)
	var previousCount int
	for _, size := range []int{0, 3, 8} {
		for len(expected) < size {
			index := len(expected)
			name := fmt.Sprintf("Budget model %02d", 8-index)
			created, createErr := models.CreateWorkTimeModel(testpkg.Ctx(t), workforce.CreateWorkTimeModel{WorkTimeModelFields: budgetWorkTimeModelFields(name, index)})
			require.NoError(t, createErr)
			entries := []map[string]any{}
			totals := []int{0, 0}
			if index != 0 {
				entries = []map[string]any{
					{"week_index": 0, "day_of_week": 1, "target_minutes": 360},
					{"week_index": 0, "day_of_week": 4, "target_minutes": 120},
					{"week_index": 1, "day_of_week": 3, "target_minutes": 90},
				}
				totals = []int{480, 90}
			}
			row := map[string]any{
				"id": created.ID, "name": name, "rotation_length": 2,
				"rotation_anchor_date": "2026-10-26", "entries": entries, "weekly_totals": totals,
			}
			expected = append([]map[string]any{row}, expected...)
		}
		counter.Reset()
		request := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/", nil, testutil.WithJWTBearer(token))
		response := testutil.ExecuteRequestForTest(t, router, request)
		queries := counter.Queries()
		encoded, encodeErr := json.Marshal(queries)
		require.NoError(t, encodeErr)
		t.Logf("work_time_models_list n=%d statements=%d queries_json=%s", size, len(queries), encoded)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
		want, encodeErr := json.Marshal(expected)
		require.NoError(t, encodeErr)
		require.JSONEq(t, string(want), string(envelope.Data), "N=%d: preserve DTO, model/entry order, totals, empty arrays and tenant scope", size)
		require.Zero(t, counter.WriteRows(), "list request must not mutate rows")
		require.Zero(t, notifications, "list request must not notify a change")
		for _, operation := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
			require.Empty(t, counter.Operation(operation), "list must issue no %s", operation)
		}
		require.Len(t, counter.Selects("config.work_time_models"), 1, "one tenant-scoped model read")
		entryReads := 1
		if size == 0 {
			entryReads = 0
		}
		require.Len(t, counter.Selects("config.work_time_model_entries"), entryReads, "at most one bulk entry read")
		if size == 8 {
			require.Len(t, queries, previousCount, "query count must not grow with the nonempty list")
		}
		if size == 0 {
			querybudgets.AssertQueryBudget(t, "api.workforce.work_time_models.list.empty", queries)
		} else {
			querybudgets.AssertQueryBudget(t, "api.workforce.work_time_models.list", queries)
		}
		previousCount = len(queries)
	}
}

func budgetWorkTimeModelFields(name string, index int) workforce.WorkTimeModelFields {
	fields := workforce.WorkTimeModelFields{Name: name, RotationLength: 2, RotationAnchorDate: "2026-10-26"}
	if index != 0 {
		fields.Entries = []workforce.WorkTimeModelEntry{
			{WeekIndex: 1, DayOfWeek: 3, TargetMinutes: 90},
			{WeekIndex: 0, DayOfWeek: 4, TargetMinutes: 120},
			{WeekIndex: 0, DayOfWeek: 1, TargetMinutes: 360},
		}
	}
	return fields
}
