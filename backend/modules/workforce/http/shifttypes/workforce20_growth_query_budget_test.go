//go:build testhooks

package shifttypes_test

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

// These are staff-specific HTTP routes, not aliases of the tenant-wide report.
func TestWorkforce20StaffSessionGrowth(t *testing.T) {
	t.Parallel()
	for _, route := range []string{"export", "month-summary", "schedule-targets", "sessions/edits"} {
		t.Run(route, func(t *testing.T) {
			testpkg.OwnTenant(t)
			f := newStaffGrowthFixture(t)
			testpkg.SetTestTenantSetting(t, f.db, testpkg.Tenant(t), "operations.time_tracking_account_start_date", `"2026-01-01"`)
			foreign := testpkg.NewTenantScope(t, f.db)
			foreignStaff := testpkg.CreateTestStaffForTenant(t, f.db, foreign.TenantID, "Foreign", "Session")
			session := func(tenant, staff int64, i int) int64 {
				return staffGrowthInsert(t, f.db, "active.work_sessions", map[string]any{"tenant_id": tenant, "staff_id": staff, "date": fmt.Sprintf("2026-01-%02d", i+1), "status": "present", "source": "app", "check_in_time": time.Date(2026, 1, i+1, 8, 0, 0, 0, time.UTC), "check_out_time": time.Date(2026, 1, i+1, 16, 0, 0, 0, time.UTC), "created_by": staff})
			}
			foreignSession := session(foreign.TenantID, foreignStaff.ID, 0)
			staffGrowthInsert(t, f.db, "audit.work_session_edits", map[string]any{"tenant_id": foreign.TenantID, "session_id": foreignSession, "staff_id": foreignStaff.ID, "edited_by": foreignStaff.ID, "field_name": "reason", "new_value": "Foreign", "created_at": time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)})
			var sessions []int64
			var edits []int64
			counts := []int{}
			var deniedPath string
			for _, n := range []int{3, 8} {
				for len(sessions) < n {
					i := len(sessions)
					sessions = append(sessions, session(testpkg.Tenant(t), f.staff, i))
					edits = append(edits, staffGrowthInsert(t, f.db, "audit.work_session_edits", map[string]any{"tenant_id": testpkg.Tenant(t), "session_id": sessions[0], "staff_id": f.staff, "edited_by": f.staff, "field_name": "reason", "new_value": fmt.Sprintf("Edit %02d", i), "created_at": time.Date(2026, 1, i+1, 12, 0, 0, 0, time.UTC)}))
				}
				path := fmt.Sprintf("/staff/%d/time-tracking/%s?from=2026-01-01&to=2026-01-31&format=csv&year=2026&month=1", f.staff, route)
				if route == "sessions/edits" {
					path = fmt.Sprintf("/staff/%d/time-tracking/sessions/%d/edits", f.staff, sessions[0])
				}
				deniedPath = path
				rec, queries := f.get(t, path)
				require.NotContains(t, rec.Body.String(), "Foreign")
				switch route {
				case "export":
					require.Equal(t, "text/csv; charset=utf-8", rec.Header().Get("Content-Type"))
					reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(rec.Body.String(), "\xef\xbb\xbf")))
					reader.Comma = ';'
					rows, err := reader.ReadAll()
					require.NoError(t, err)
					require.Len(t, rows, n+1)
					for i := 0; i < n; i++ {
						require.Equal(t, fmt.Sprintf("%02d.01.2026", i+1), rows[i+1][0])
					}
				case "month-summary":
					data := staffGrowthData(t, rec).(map[string]any)
					require.Equal(t, float64(f.staff), data["staff_id"])
					require.Equal(t, float64(n*480), data["actual_minutes"])
				case "schedule-targets":
					rows := staffGrowthData(t, rec).([]any)
					require.Len(t, rows, 31)
					for i, row := range rows {
						data := row.(map[string]any)
						require.Equal(t, fmt.Sprintf("2026-01-%02d", i+1), data["date"])
						want := 0
						if i < n {
							want = 480
						}
						require.Equal(t, float64(want), data["actual_minutes"])
					}
				case "sessions/edits":
					rows := staffGrowthData(t, rec).([]any)
					require.Len(t, rows, n)
					for i, row := range rows {
						data := row.(map[string]any)
						require.Equal(t, float64(edits[n-1-i]), data["id"])
						require.Equal(t, fmt.Sprintf("Edit %02d", n-1-i), data["new_value"])
						require.Equal(t, "Growth Reader", data["editor_name"])
						require.Equal(t, true, data["is_self_edit"])
					}
				}
				switch route {
				case "export":
					querybudgets.AssertQueryBudget(t, "workforce.workforce20.staff_export", queries)
				case "month-summary":
					querybudgets.AssertQueryBudget(t, "workforce.workforce20.staff_month_summary", queries)
				case "schedule-targets":
					querybudgets.AssertQueryBudget(t, "workforce.workforce20.staff_schedule_targets", queries)
				case "sessions/edits":
					querybudgets.AssertQueryBudget(t, "workforce.workforce20.staff_session_edits", queries)
				}
				counts = append(counts, len(queries.Statements))
			}
			require.Equal(t, counts[0], counts[1], "session/edit growth must not add statements")
			// The production permission middleware must reject a valid token lacking
			// the route permission, independently of the successful positive capture.
			claims := testutil.DefaultTestClaims()
			claims.ID = int(f.account)
			claims.Roles = []string{"user"}
			claims.Permissions = nil

			req := httptest.NewRequest(http.MethodGet, deniedPath, nil)
			req.Header.Set("Authorization", "Bearer "+testutil.MintTestJWT(t, claims))
			rec := httptest.NewRecorder()
			f.router.ServeHTTP(rec, req)
			require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
		})
	}
}
