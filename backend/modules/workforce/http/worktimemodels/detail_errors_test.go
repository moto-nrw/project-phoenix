//go:build testhooks

package worktimemodels_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/workforce"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestWorkTimeModelDetailProtectedFailures(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name       string
		id         string
		permission string
		cancel     bool
		breakTable string
		missing    bool
		status     int
		detail     string
		statements int
	}{
		{name: "permission before transaction", status: http.StatusForbidden, detail: "Forbidden"},
		{name: "malformed id", id: "invalid", permission: "time_tracking:manage", status: http.StatusBadRequest, detail: `strconv.ParseInt: parsing "invalid": invalid syntax`, statements: 3},
		{name: "nonpositive id", id: "0", permission: "time_tracking:manage", status: http.StatusBadRequest, detail: "work time model ID is required", statements: 3},
		{name: "cancel before transaction", permission: "time_tracking:manage", cancel: true, status: http.StatusInternalServerError, detail: "Internal Server Error", statements: 0},
		{name: "model read error", permission: "time_tracking:manage", breakTable: "work_time_models", status: http.StatusInternalServerError,
			detail: `workforce postgres: find work-time model: `, statements: 4},
		{name: "nested entry read error", permission: "time_tracking:manage", breakTable: "work_time_model_entries", status: http.StatusInternalServerError,
			detail: `workforce postgres: load work-time model entries: `, statements: 5},
		{name: "missing model never reads unavailable entries", permission: "time_tracking:manage", missing: true, breakTable: "work_time_model_entries", status: http.StatusNotFound,
			detail: "work time model not found", statements: 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			db := testpkg.SetupIsolatedTestDB(t)
			models := api.NewAbsenceTypeTestCapability(testpkg.NewDatabase(t, db))
			created, err := models.CreateWorkTimeModel(testpkg.Ctx(t), workforce.CreateWorkTimeModel{WorkTimeModelFields: detailModelFields(3)})
			require.NoError(t, err)
			if test.missing {
				require.NoError(t, models.DeleteWorkTimeModel(testpkg.Ctx(t), created.ID))
			}
			notifications := 0
			router := chi.NewRouter()
			router.Mount("/api/work-time-models", api.NewWorkTimeModelsRouterForBudgetTest(models, func(context.Context) { notifications++ }))
			claims := testutil.DefaultTestClaims()
			claims.TenantID = testpkg.Tenant(t)
			claims.IsAdmin = false
			claims.Permissions = []string{test.permission}
			token := testutil.MintTestJWT(t, claims)
			if test.breakTable != "" {
				_, err = db.ExecContext(testpkg.Ctx(t), "ALTER TABLE config."+test.breakTable+" RENAME TO budget_unavailable_model_table")
				require.NoError(t, err)
				t.Cleanup(func() {
					_, restoreErr := db.ExecContext(testpkg.Ctx(t), "ALTER TABLE config.budget_unavailable_model_table RENAME TO "+test.breakTable)
					require.NoError(t, restoreErr)
				})
			}
			id := test.id
			if id == "" {
				id = fmt.Sprint(created.ID)
			}
			detail := test.detail
			if test.breakTable != "" && !test.missing {
				// Probe the same missing relation before query capture. Its SQLSTATE
				// and localized diagnostic keep the complete wire expectation exact.
				_, probeErr := db.ExecContext(testpkg.Ctx(t), "SELECT 1 FROM config."+test.breakTable)
				require.Equal(t, "42P01", testpkg.PostgresSQLState(probeErr))
				detail += probeErr.Error()
			}
			counter := testpkg.CaptureQueries(t, db)
			request := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/api/work-time-models/"+id, nil, testutil.WithJWTBearer(token))
			if test.cancel {
				ctx, cancel := context.WithCancel(request.Context())
				cancel()
				request = request.WithContext(ctx)
			}
			response := testutil.ExecuteRequestForTest(t, router, request)
			encoded, encodeErr := json.Marshal(counter.Queries())
			require.NoError(t, encodeErr)
			t.Logf("work_time_model_detail_failure=%s statements=%d body=%s queries_json=%s", test.name, counter.Total(), response.Body.String(), encoded)
			require.Equal(t, test.status, response.Code, response.Body.String())
			require.Equal(t, "application/json", response.Header().Get("Content-Type"))
			code, anchor := "general.input", "anleitung-eingabe-pruefen"
			switch test.status {
			case http.StatusForbidden:
				code, anchor = "general.permission", "anleitung-zugriff-pruefen"
			case http.StatusInternalServerError:
				code, anchor = "general.server", "anleitung-unerwarteter-fehler"
			}
			want, encodeErr := json.Marshal(map[string]any{
				"type": "https://moto-app.de/help/fehlermeldungen#" + anchor, "title": http.StatusText(test.status),
				"status": test.status, "detail": detail, "instance": "", "code": code,
			})
			require.NoError(t, encodeErr)
			require.JSONEq(t, string(want), response.Body.String())
			require.Len(t, counter.Queries(), test.statements)
			assertDetailReadOnly(t, counter, notifications)
			if test.breakTable != "" {
				if test.missing {
					require.Equal(t, "COMMIT", counter.Queries()[counter.Total()-1])
					require.Empty(t, counter.Selects("config.work_time_model_entries"))
				} else {
					require.Equal(t, "ROLLBACK", counter.Queries()[counter.Total()-1])
				}
			} else {
				require.Empty(t, counter.Selects("config.work_time_models"))
				require.Empty(t, counter.Selects("config.work_time_model_entries"))
			}
		})
	}
}
