//go:build testhooks

package absencetypes_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestAbsenceTypesListProtectedFailures(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name       string
		permission string
		cancel     bool
		breakRead  bool
		status     int
		detail     string
		statements int
	}{
		{name: "permission before transaction", status: http.StatusForbidden, detail: "Forbidden", statements: 0},
		{name: "cancel before transaction", permission: "time_tracking:own", cancel: true, status: http.StatusInternalServerError, detail: "Internal Server Error", statements: 0},
		{name: "catalog read error rolls back", permission: "time_tracking:own", breakRead: true, status: http.StatusInternalServerError,
			detail: `database error during list all staff absence types: workforce postgres: list staff absence types: `, statements: 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			db := testpkg.SetupIsolatedTestDB(t)
			module, caller, err := api.NewAbsenceTypeTestModule(testpkg.NewDatabase(t, db))
			require.NoError(t, err)
			router := chi.NewRouter()
			router.Mount("/api/absence-types", api.NewAbsenceTypesRouterForBudgetTest(module.Administration, caller.Caller().StaffID))
			claims := testutil.DefaultTestClaims()
			claims.TenantID = testpkg.Tenant(t)
			claims.IsAdmin = false
			claims.Permissions = []string{test.permission}
			token := testutil.MintTestJWT(t, claims)
			if test.breakRead {
				_, err = db.ExecContext(testpkg.Ctx(t), "ALTER TABLE active.staff_absence_types RENAME TO budget_unavailable_absence_types")
				require.NoError(t, err)
				t.Cleanup(func() {
					_, restoreErr := db.ExecContext(testpkg.Ctx(t), "ALTER TABLE active.budget_unavailable_absence_types RENAME TO staff_absence_types")
					require.NoError(t, restoreErr)
				})
			}
			detail := test.detail
			if test.breakRead {
				// Probe the same missing relation before query capture. Its SQLSTATE
				// and localized diagnostic keep the complete wire expectation exact.
				_, probeErr := db.ExecContext(testpkg.Ctx(t), "SELECT 1 FROM active.staff_absence_types")
				require.Equal(t, "42P01", testpkg.PostgresSQLState(probeErr))
				detail += probeErr.Error()
			}
			counter := testpkg.CaptureQueries(t, db)
			for _, suffix := range []string{"?include_usage=false", "?include_usage=true"} {
				counter.Reset()
				request := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/api/absence-types/"+suffix, nil, testutil.WithJWTBearer(token))
				if test.cancel {
					ctx, cancel := context.WithCancel(request.Context())
					cancel()
					request = request.WithContext(ctx)
				}
				response := testutil.ExecuteRequestForTest(t, router, request)
				encoded, encodeErr := json.Marshal(counter.Queries())
				require.NoError(t, encodeErr)
				t.Logf("absence_types_failure=%s flag=%s statements=%d body=%s queries_json=%s", test.name, suffix, counter.Total(), response.Body.String(), encoded)
				require.Equal(t, test.status, response.Code, response.Body.String())
				require.Equal(t, "application/json", response.Header().Get("Content-Type"))
				code, anchor := "general.server", "anleitung-unerwarteter-fehler"
				if test.status == http.StatusForbidden {
					code, anchor = "general.permission", "anleitung-zugriff-pruefen"
				}
				want, encodeErr := json.Marshal(map[string]any{
					"type": "https://moto-app.de/help/fehlermeldungen#" + anchor, "title": http.StatusText(test.status),
					"status": test.status, "detail": detail, "instance": "", "code": code,
				})
				require.NoError(t, encodeErr)
				require.JSONEq(t, string(want), response.Body.String())
				require.Len(t, counter.Queries(), test.statements)
				require.Zero(t, counter.WriteRows())
				for _, operation := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
					require.Empty(t, counter.Operation(operation), fmt.Sprintf("failure must not issue %s", operation))
				}
				if test.breakRead {
					require.Equal(t, "ROLLBACK", counter.Queries()[counter.Total()-1])
				} else {
					require.Empty(t, counter.Selects("active.staff_absence_types"))
				}
			}
		})
	}
}
