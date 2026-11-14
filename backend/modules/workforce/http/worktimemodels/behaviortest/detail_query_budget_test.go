//go:build testhooks

package worktimemodels_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/workforce"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestWorkTimeModelDetailQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	models := api.NewAbsenceTypeTestCapability(testpkg.NewDatabase(t, db))
	notifications := 0
	router := chi.NewRouter()
	router.Mount("/api/work-time-models", api.NewWorkTimeModelsRouterForBudgetTest(models, func(context.Context) { notifications++ }))
	claims := testutil.DefaultTestClaims()
	claims.TenantID = testpkg.Tenant(t)
	claims.Permissions = []string{"time_tracking:manage"}
	claims.IsAdmin = false
	token := testutil.MintTestJWT(t, claims)
	foreign := testpkg.NewTenantScope(t, db)
	foreignModel, err := models.CreateWorkTimeModel(foreign.Context(), workforce.CreateWorkTimeModel{WorkTimeModelFields: detailModelFields(8)})
	require.NoError(t, err)
	// A previously existing, now deleted ID is a genuine missing-ID control,
	// distinct from a successful model with zero nested entries.
	deleted, err := models.CreateWorkTimeModel(testpkg.Ctx(t), workforce.CreateWorkTimeModel{WorkTimeModelFields: detailModelFields(0)})
	require.NoError(t, err)
	require.NoError(t, models.DeleteWorkTimeModel(testpkg.Ctx(t), deleted.ID))
	counter := testpkg.CaptureQueries(t, db)
	for _, size := range []int{0, 3, 8} {
		created, createErr := models.CreateWorkTimeModel(testpkg.Ctx(t), workforce.CreateWorkTimeModel{WorkTimeModelFields: detailModelFields(size)})
		require.NoError(t, createErr)
		// Set valid TIME fixture values directly: the current create adapter's
		// year-zero timestamp binding rejects a nonempty StartTime before reads.
		for index := 0; index < size; index += 2 {
			_, fixtureErr := db.NewUpdate().TableExpr("config.work_time_model_entries").
				Set("start_time = ?::time", fmt.Sprintf("%02d:15:00", 7+index)).
				Where("model_id = ? AND week_index = ? AND day_of_week = ?", created.ID, index/5, index%5+1).
				Exec(testpkg.Ctx(t))
			require.NoError(t, fixtureErr)
		}
		entries := []map[string]any{}
		totals := []int{0, 0}
		for index := range size {
			entry := map[string]any{"week_index": index / 5, "day_of_week": index%5 + 1, "target_minutes": 60 + index*15}
			if index%2 == 0 {
				entry["start_time"] = fmt.Sprintf("%02d:15", 7+index)
			}
			entries = append(entries, entry)
			totals[index/5] += 60 + index*15
		}
		want, encodeErr := json.Marshal(map[string]any{
			"status": "success", "message": "Work time model retrieved",
			"data": map[string]any{
				"id": created.ID, "name": created.Name, "rotation_length": 2,
				"rotation_anchor_date": "2026-10-26", "entries": entries, "weekly_totals": totals,
			},
		})
		require.NoError(t, encodeErr)
		counter.Reset()
		request := testutil.NewAuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/api/work-time-models/%d", created.ID), nil, testutil.WithJWTBearer(token))
		response := testutil.ExecuteRequestForTest(t, router, request)
		queries := counter.Measurement(counter.Queries())
		encoded, encodeErr := json.Marshal(queries.Statements)
		require.NoError(t, encodeErr)
		t.Logf("work_time_model_detail entries=%d statements=%d queries_json=%s", size, len(queries.Statements), encoded)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		require.Equal(t, "application/json", response.Header().Get("Content-Type"))
		require.JSONEq(t, string(want), response.Body.String(), "preserve complete ordered DTO, HH:MM clocks, omitted clocks, empty entries and weekly totals")
		assertDetailReadOnly(t, counter, notifications)
		require.Len(t, counter.Selects("config.work_time_models"), 1)
		require.Len(t, counter.Selects("config.work_time_model_entries"), 1, "one read regardless of nested entry cardinality")
		querybudgets.AssertQueryBudget(t, "api.workforce.work_time_models.detail", queries)
		assertDetailTenantTransaction(t, queries.Statements)
	}
	for _, id := range []int64{deleted.ID, foreignModel.ID} {
		counter.Reset()
		request := testutil.NewAuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/api/work-time-models/%d", id), nil, testutil.WithJWTBearer(token))
		response := testutil.ExecuteRequestForTest(t, router, request)
		queries := counter.Measurement(counter.Queries())
		encoded, encodeErr := json.Marshal(queries.Statements)
		require.NoError(t, encodeErr)
		t.Logf("work_time_model_detail missing_id=%d statements=%d body=%s queries_json=%s", id, len(queries.Statements), response.Body.String(), encoded)
		require.Equal(t, http.StatusNotFound, response.Code, response.Body.String())
		require.Equal(t, "application/json", response.Header().Get("Content-Type"))
		require.JSONEq(t, `{"type":"https://moto-app.de/help/fehlermeldungen#anleitung-eingabe-pruefen","title":"Not Found","status":404,"detail":"work time model not found","instance":"","code":"general.input"}`, response.Body.String())
		assertDetailReadOnly(t, counter, notifications)
		require.Len(t, counter.Selects("config.work_time_models"), 1)
		require.Empty(t, counter.Selects("config.work_time_model_entries"), "missing or foreign models must not load entries")
		querybudgets.AssertQueryBudget(t, "api.workforce.work_time_models.detail.not_found", queries)
		assertDetailTenantTransaction(t, queries.Statements)
	}
}

func assertDetailTenantTransaction(t *testing.T, queries []string) {
	t.Helper()
	require.Equal(t, "BEGIN", queries[0])
	require.Equal(t, fmt.Sprintf("SELECT set_config('role', 'phoenix_tenant', true), set_config('app.current_tenant_id', '%d', true)", testpkg.Tenant(t)), queries[1])
	require.Equal(t, "COMMIT", queries[len(queries)-1])
}

func detailModelFields(size int) workforce.WorkTimeModelFields {
	fields := workforce.WorkTimeModelFields{Name: fmt.Sprintf("Detail %d", size), RotationLength: 2, RotationAnchorDate: "2026-10-26"}
	// Insert in reverse order so the HTTP assertion proves persisted ordering.
	for index := size - 1; index >= 0; index-- {
		entry := workforce.WorkTimeModelEntry{WeekIndex: index / 5, DayOfWeek: index%5 + 1, TargetMinutes: 60 + index*15}
		fields.Entries = append(fields.Entries, entry)
	}
	return fields
}

func assertDetailReadOnly(t *testing.T, counter *testpkg.QueryCounter, notifications int) {
	t.Helper()
	require.Zero(t, counter.WriteRows())
	require.Zero(t, notifications)
	for _, operation := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
		require.Empty(t, counter.Operation(operation), "detail must issue no %s", operation)
	}
	require.Empty(t, counter.Matching(func(query string) bool { return strings.Contains(strings.ToUpper(query), "FOR UPDATE") }), "detail must take no row locks")
}
