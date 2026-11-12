package timetracking_test

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/workforce/workforcetest"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestWorkforce20OwnGrowth(t *testing.T) {
	t.Parallel()
	for _, route := range []string{"export", "month-summary", "schedule-targets", "edits", "breaks", "absences", "presence-map"} {
		t.Run(route, func(t *testing.T) {
			testpkg.OwnTenant(t)
			testpkg.SetupIsolatedTestDB(t)
			db, router := workforcetest.OwnGrowthRouter(t)
			staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Own", "Reader")
			testpkg.SetTestTenantSetting(t, db, testpkg.Tenant(t), "operations.time_tracking_account_start_date", `"2026-01-01"`)
			foreign := testpkg.NewTenantScope(t, db)
			foreignStaff := testpkg.CreateTestStaffForTenant(t, db, foreign.TenantID, "Foreign", "Own")
			insert := func(table string, row map[string]any) int64 {
				var id int64
				require.NoError(t, db.NewInsert().Model(&row).TableExpr(table).Returning("id").Scan(testpkg.Ctx(t), &id))
				return id
			}
			session := func(tenant, staff int64, i int) int64 {
				date := fmt.Sprintf("2026-01-%02d", i+1)
				at := time.Date(2026, 1, i+1, 8, 0, 0, 0, time.UTC)
				if route == "presence-map" {
					date = testpkg.TodayDate().String()
					at = testpkg.TodayDate().BerlinMidnight().Add(8 * time.Hour)
				}
				return insert("active.work_sessions", map[string]any{"tenant_id": tenant, "staff_id": staff, "date": date, "status": "present", "source": "app", "check_in_time": at, "check_out_time": at.Add(8 * time.Hour), "created_by": staff})
			}
			foreignSession := session(foreign.TenantID, foreignStaff.ID, 0)
			insert("audit.work_session_edits", map[string]any{"tenant_id": foreign.TenantID, "session_id": foreignSession, "staff_id": foreignStaff.ID, "edited_by": foreignStaff.ID, "field_name": "reason", "new_value": "Foreign"})
			insert("active.work_session_breaks", map[string]any{"tenant_id": foreign.TenantID, "session_id": foreignSession, "started_at": time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC), "ended_at": time.Date(2026, 1, 1, 9, 1, 0, 0, time.UTC), "duration_minutes": 1})
			insert("active.staff_absences", map[string]any{"tenant_id": foreign.TenantID, "staff_id": foreignStaff.ID, "created_by": foreignStaff.ID, "absence_type": "vacation", "status": "requested", "date_start": "2026-02-01", "date_end": "2026-02-01", "requested_at": time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)})

			var sessions, edits, breaks, absences, staffIDs []int64
			counts := []int{}
			for _, n := range []int{3, 8} {
				for len(sessions) < n {
					i := len(sessions)
					sid := staff.ID
					if route == "presence-map" && i > 0 {
						sid = testpkg.CreateTestStaff(t, db, "Own", fmt.Sprintf("Member%02d", i)).ID
					}
					staffIDs = append(staffIDs, sid)
					sessions = append(sessions, session(testpkg.Tenant(t), sid, i))
					edits = append(edits, insert("audit.work_session_edits", map[string]any{"tenant_id": testpkg.Tenant(t), "session_id": sessions[0], "staff_id": staff.ID, "edited_by": staff.ID, "field_name": "reason", "new_value": fmt.Sprintf("Edit %02d", i), "created_at": time.Date(2026, 1, i+1, 12, 0, 0, 0, time.UTC)}))
					breaks = append(breaks, insert("active.work_session_breaks", map[string]any{"tenant_id": testpkg.Tenant(t), "session_id": sessions[0], "started_at": time.Date(2026, 1, 1, 9, i*2, 0, 0, time.UTC), "ended_at": time.Date(2026, 1, 1, 9, i*2+1, 0, 0, time.UTC), "duration_minutes": 1}))
					absences = append(absences, insert("active.staff_absences", map[string]any{"tenant_id": testpkg.Tenant(t), "staff_id": staff.ID, "created_by": staff.ID, "absence_type": "vacation", "status": "requested", "date_start": fmt.Sprintf("2026-02-%02d", i+1), "date_end": fmt.Sprintf("2026-02-%02d", i+1), "requested_at": time.Date(2026, 1, i+1, 12, 0, 0, 0, time.UTC)}))
				}
				path := "/time-tracking/" + route + "?from=2026-01-01&to=2026-01-31&format=csv&year=2026&month=1"
				if route == "edits" {
					path = fmt.Sprintf("/time-tracking/%d/edits", sessions[0])
				}
				if route == "breaks" {
					path = fmt.Sprintf("/time-tracking/breaks/%d", sessions[0])
				}
				if route == "absences" {
					path = "/time-tracking/absences?from=2026-02-01&to=2026-02-28"
				}
				token := workforcetest.OwnGrowthToken(t, account.ID, 0, "time_tracking:own", "users:read")
				req := httptest.NewRequest(http.MethodGet, path, nil)
				req.Header.Set("Authorization", "Bearer "+token)
				counter := testpkg.CaptureQueries(t, db)
				rec := httptest.NewRecorder()
				testpkg.MeasuredHTTPRequest(counter, testpkg.MountedQueryBudgetRouter("/api", router), rec, req)
				counter.Stop()
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				require.NotContains(t, rec.Body.String(), "Foreign")
				var body struct {
					Data any `json:"data"`
				}
				if route != "export" {
					require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				}
				switch route {
				case "export":
					reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(rec.Body.String(), "\xef\xbb\xbf")))
					reader.Comma = ';'
					rows, err := reader.ReadAll()
					require.NoError(t, err)
					require.Len(t, rows, n+1)
					for i := range n {
						require.Equal(t, fmt.Sprintf("%02d.01.2026", i+1), rows[i+1][0])
					}
				case "month-summary":
					require.Equal(t, float64(n*480-n), body.Data.(map[string]any)["actual_minutes"])
				case "schedule-targets":
					rows := body.Data.([]any)
					require.Len(t, rows, 31)
					for i, row := range rows {
						data := row.(map[string]any)
						require.Equal(t, fmt.Sprintf("2026-01-%02d", i+1), data["date"])
						want := 0
						if i < n {
							want = 480
						}
						if i == 0 {
							want -= n
						}
						require.Equal(t, float64(want), data["actual_minutes"])
					}
				case "edits":
					rows := body.Data.([]any)
					require.Len(t, rows, n)
					for i, row := range rows {
						data := row.(map[string]any)
						require.Equal(t, float64(edits[n-1-i]), data["id"])
						require.Equal(t, fmt.Sprintf("Edit %02d", n-1-i), data["new_value"])
						require.Equal(t, "Own Reader", data["editor_name"])
					}
				case "breaks":
					rows := body.Data.([]any)
					require.Len(t, rows, n)
					for i, row := range rows {
						require.Equal(t, fmt.Sprintf("%d", breaks[i]), row.(map[string]any)["id"])
					}
				case "absences":
					rows := body.Data.([]any)
					require.Len(t, rows, n)
					for i, row := range rows {
						require.Equal(t, float64(absences[i]), row.(map[string]any)["id"])
					}
				case "presence-map":
					data := body.Data.(map[string]any)
					require.Len(t, data, n)
					for _, id := range staffIDs {
						require.Equal(t, "checked_out", data[strconv.FormatInt(id, 10)])
					}
					require.NotContains(t, data, strconv.FormatInt(foreignStaff.ID, 10))
				}
				switch route {
				case "export":
					querybudgets.AssertQueryBudget(t, "workforce.workforce20.own_export", counter.Measurement(counter.Queries()))
				case "month-summary":
					querybudgets.AssertQueryBudget(t, "workforce.workforce20.own_month_summary", counter.Measurement(counter.Queries()))
				case "schedule-targets":
					querybudgets.AssertQueryBudget(t, "workforce.workforce20.own_schedule_targets", counter.Measurement(counter.Queries()))
				case "edits":
					querybudgets.AssertQueryBudget(t, "workforce.workforce20.own_edits", counter.Measurement(counter.Queries()))
				case "breaks":
					querybudgets.AssertQueryBudget(t, "workforce.workforce20.own_breaks", counter.Measurement(counter.Queries()))
				case "absences":
					querybudgets.AssertQueryBudget(t, "workforce.workforce20.own_absences", counter.Measurement(counter.Queries()))
				case "presence-map":
					querybudgets.AssertQueryBudget(t, "workforce.workforce20.own_presence_map", counter.Measurement(counter.Queries()))
				}
				counts = append(counts, len(counter.Queries()))
				denied := httptest.NewRequest(http.MethodGet, path, nil)
				denied.Header.Set("Authorization", "Bearer "+workforcetest.OwnGrowthToken(t, account.ID, 0))
				deniedRec := httptest.NewRecorder()
				testpkg.MeasuredHTTPRequest(counter, testpkg.MountedQueryBudgetRouter("/api", router), deniedRec, denied)
				require.Equal(t, http.StatusForbidden, deniedRec.Code, deniedRec.Body.String())
			}
			require.Equal(t, counts[0], counts[1])
		})
	}
}
