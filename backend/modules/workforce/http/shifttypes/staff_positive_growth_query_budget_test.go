//go:build testhooks

package shifttypes_test

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func init() { testutil.SeedTestJWTConfig() }

type staffGrowthFixture struct {
	db             *bun.DB
	router         http.Handler
	staff, account int64
}

func newStaffGrowthFixture(t *testing.T) staffGrowthFixture {
	t.Helper()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)
	router, err := api.StaffGrowthTestRouter(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	router = testpkg.TenantRuntimeMiddleware(t, db)(router)
	staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Growth", "Reader")
	return staffGrowthFixture{db, testpkg.MountedQueryBudgetRouter("/api", router), staff.ID, account.ID}
}
func (f staffGrowthFixture) get(t *testing.T, path string) (*httptest.ResponseRecorder, testpkg.QueryMeasurement) {
	t.Helper()
	require.Same(t, testpkg.SetupTestDB(t), f.db)
	claims := testutil.DefaultTestClaims()
	claims.ID = int(f.account)
	claims.Permissions = []string{"time_tracking:manage", "vacation:approve", "staff:stammdaten", "staff:documents"}
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+testutil.MintTestJWT(t, claims))
	// Capture the entire route-sized middleware/handler transaction, including
	// BEGIN, tenant role/scope setup, response enrichment and COMMIT. Fixture SQL
	// and JWT creation are outside capture. Outer server liveness is excluded.
	counter := testpkg.CaptureQueries(t, f.db)
	rec := httptest.NewRecorder()
	testpkg.MeasuredHTTPRequest(counter, f.router, rec, req)
	counter.Stop()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	return rec, counter.Measurement(counter.Queries())
}
func staffGrowthData(t *testing.T, rec *httptest.ResponseRecorder) any {
	t.Helper()
	var body struct {
		Data any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body.Data
}
func staffGrowthInsert(t *testing.T, db *bun.DB, table string, row map[string]any) int64 {
	t.Helper()
	require.Same(t, testpkg.SetupTestDB(t), db)
	var id int64
	require.NoError(t, db.NewInsert().Model(&row).TableExpr(table).Returning("id").Scan(testpkg.Ctx(t), &id))
	return id
}

func TestStaffAbsencePositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	for _, route := range []string{"pending", "requests", "subject"} {
		t.Run(route, func(t *testing.T) {
			testpkg.OwnTenant(t)
			f := newStaffGrowthFixture(t)
			foreign := testpkg.NewTenantScope(t, f.db)
			foreignStaff := testpkg.CreateTestStaffForTenant(t, f.db, foreign.TenantID, "Foreign", "Absence")
			foreignID := staffGrowthInsert(t, f.db, "active.staff_absences", map[string]any{"tenant_id": foreign.TenantID, "staff_id": foreignStaff.ID, "created_by": foreignStaff.ID, "absence_type": "vacation", "status": "requested", "date_start": "2026-01-01", "date_end": "2026-01-01", "requested_at": time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)})
			ids := []int64{}
			counts := []int{}
			for _, n := range []int{3, 8} {
				for len(ids) < n {
					i := len(ids)
					ids = append(ids, staffGrowthInsert(t, f.db, "active.staff_absences", map[string]any{"tenant_id": testpkg.Tenant(t), "staff_id": f.staff, "created_by": f.staff, "absence_type": "vacation", "status": "requested", "date_start": fmt.Sprintf("2026-01-%02d", i+1), "date_end": fmt.Sprintf("2026-01-%02d", i+1), "requested_at": time.Date(2026, 1, i+1, 12, 0, 0, 0, time.UTC)}))
				}
				path := "/staff/absences/" + route
				if route == "subject" {
					path = fmt.Sprintf("/staff/%d/absences?from=2026-01-01&to=2026-01-31", f.staff)
				}
				rec, queries := f.get(t, path)
				rows := staffGrowthData(t, rec).([]any)
				require.Len(t, rows, n)
				seen := map[string]bool{}
				for _, item := range rows {
					row := item.(map[string]any)
					require.Equal(t, "vacation", row["absence_type"])
					require.Equal(t, "requested", row["status"])
					id := fmt.Sprint(row["id"])
					require.NotEqual(t, strconv.FormatInt(foreignID, 10), id)
					seen[id] = true
				}
				for _, id := range ids {
					require.True(t, seen[strconv.FormatInt(id, 10)])
				}
				// Both pending and per-person lists explicitly order ascending by their
				// distinct requested_at/date_start fixture keys, as does the open
				// request work list. Ties are deliberately absent.
				for i, item := range rows {
					expected := ids[i]

					require.Equal(t, strconv.FormatInt(expected, 10), fmt.Sprint(item.(map[string]any)["id"]))
				}
				t.Logf("route=%s N=%d statements=%d scope=full-resource-request", path, n, len(queries.Statements))
				counts = append(counts, len(queries.Statements))
				switch route {
				case "pending":
					querybudgets.AssertQueryBudget(t, "workforce.staff_growth.absences_pending", queries)
				case "requests":
					querybudgets.AssertQueryBudget(t, "workforce.staff_growth.absences_requests", queries)
				default:
					querybudgets.AssertQueryBudget(t, "workforce.staff_growth.absences_subject", queries)
				}
			}
			require.Equal(t, counts[0], counts[1])
		})
	}
}

func TestStaffQualificationPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	for _, route := range []string{"qualifications", "me/qualifications", "stammdaten"} {
		t.Run(route, func(t *testing.T) {
			testpkg.OwnTenant(t)
			f := newStaffGrowthFixture(t)
			foreign := testpkg.NewTenantScope(t, f.db)
			foreignStaff := testpkg.CreateTestStaffForTenant(t, f.db, foreign.TenantID, "Foreign", "Qualification")
			staffGrowthInsert(t, f.db, "users.staff_qualifications", map[string]any{"tenant_id": foreign.TenantID, "staff_id": foreignStaff.ID, "name": "Foreign qualification"})
			counts := []int{}
			for _, n := range []int{3, 8} {
				start := 0
				if n == 8 {
					start = 3
				}
				for i := start; i < n; i++ {
					staffGrowthInsert(t, f.db, "users.staff_qualifications", map[string]any{"tenant_id": testpkg.Tenant(t), "staff_id": f.staff, "name": fmt.Sprintf("Growth %02d", i), "acquired_on": "2025-01-01", "expires_on": "2027-01-01"})
				}
				path := fmt.Sprintf("/staff/%d/%s", f.staff, route)
				if strings.HasPrefix(route, "me/") {
					path = "/staff/" + route
				}
				rec, queries := f.get(t, path)
				data := staffGrowthData(t, rec)
				var rows []any
				if route == "stammdaten" {
					rows = data.(map[string]any)["qualifikationen"].([]any)
				} else {
					rows = data.([]any)
				}
				// The three seeded mandatory kinds remain present in the status list;
				// free qualifications grow independently and retain null type IDs.
				matched := []string{}
				for _, v := range rows {
					row := v.(map[string]any)
					name, _ := row["name"].(string)
					require.NotEqual(t, "Foreign qualification", name)
					if strings.HasPrefix(name, "Growth ") {
						matched = append(matched, name)
						require.Nil(t, row["type_id"])
						require.Equal(t, "2025-01-01", row["acquired_on"])
						require.Equal(t, "2027-01-01", row["expires_on"])
					}
				}
				require.Len(t, matched, n)
				for i, name := range matched {
					require.Equal(t, fmt.Sprintf("Growth %02d", i), name)
				}
				t.Logf("route=%s N=%d statements=%d scope=full-resource-request", path, n, len(queries.Statements))
				counts = append(counts, len(queries.Statements))
				switch route {
				case "qualifications":
					querybudgets.AssertQueryBudget(t, "workforce.staff_growth.qualifications_subject", queries)
				case "me/qualifications":
					querybudgets.AssertQueryBudget(t, "workforce.staff_growth.qualifications_me", queries)
				default:
					querybudgets.AssertQueryBudget(t, "workforce.staff_growth.stammdaten", queries)
				}
			}
			require.Equal(t, counts[0], counts[1])
		})
	}
}

func TestStaffRecordPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	for _, route := range []string{"schedule", "adjustments", "documents"} {
		t.Run(route, func(t *testing.T) {
			testpkg.OwnTenant(t)
			f := newStaffGrowthFixture(t)
			foreign := testpkg.NewTenantScope(t, f.db)
			foreignStaff := testpkg.CreateTestStaffForTenant(t, f.db, foreign.TenantID, "Foreign", "Record")
			add := func(tenant, staff int64, i int) {
				switch route {
				case "schedule":
					staffGrowthInsert(t, f.db, "config.staff_work_schedules", map[string]any{"tenant_id": tenant, "staff_id": staff, "day_of_week": i % 7, "week_index": i / 7, "rotation_length": 2, "target_minutes": 300 + i, "valid_from": "2025-01-01"})
				case "adjustments":
					staffGrowthInsert(t, f.db, "active.staff_balance_adjustments", map[string]any{"tenant_id": tenant, "staff_id": staff, "type": "payout", "minutes_delta": -10 - i, "effective_date": fmt.Sprintf("2026-01-%02d", i+1), "note": fmt.Sprintf("Growth %02d", i), "decided_by": staff})
				default:
					staffGrowthInsert(t, f.db, "users.staff_documents", map[string]any{"tenant_id": tenant, "staff_id": staff, "category": "zeugnis", "filename_display": fmt.Sprintf("Growth %02d.pdf", i), "filename_stored": fmt.Sprintf("growth-%d-%d-%02d.pdf", tenant, staff, i), "size_bytes": 100 + i, "content_type": "application/pdf", "uploaded_by": f.account, "created_at": time.Date(2026, 1, i+1, 12, 0, 0, 0, time.UTC)})
				}
			}
			add(foreign.TenantID, foreignStaff.ID, 0)
			counts := []int{}
			for _, n := range []int{3, 8} {
				start := 0
				if n == 8 {
					start = 3
				}
				for i := start; i < n; i++ {
					add(testpkg.Tenant(t), f.staff, i)
				}
				path := fmt.Sprintf("/staff/%d/%s", f.staff, route)
				if route == "adjustments" {
					path = fmt.Sprintf("/staff/%d/time-tracking/adjustments?from=2026-01-01&to=2026-01-31", f.staff)
				}
				rec, queries := f.get(t, path)
				data := staffGrowthData(t, rec)
				var rows []any
				switch route {
				case "schedule":
					rows = data.(map[string]any)["entries"].([]any)
				case "documents":
					obj := data.(map[string]any)
					rows = obj["documents"].([]any)
					require.NotContains(t, obj["visible_categories"], "au_bescheinigung")
					require.NotContains(t, obj["visible_categories"], "lohnabrechnung")
				default:
					rows = data.([]any)
				}
				require.Len(t, rows, n)
				scheduleIndices := []int{}
				for i, v := range rows {
					row := v.(map[string]any)
					switch route {
					case "schedule":
						index := int(row["week_index"].(float64))*7 + int(row["day_of_week"].(float64))
						scheduleIndices = append(scheduleIndices, index)
						require.Less(t, index, n)
						require.Equal(t, float64(300+index), row["target_minutes"])
						if i > 0 {
							require.LessOrEqual(t, rows[i-1].(map[string]any)["day_of_week"].(float64), row["day_of_week"].(float64))
						}
					case "adjustments":
						require.Equal(t, fmt.Sprintf("Growth %02d", i), row["note"])
						require.Equal(t, float64(-10-i), row["minutes_delta"])
					default:
						require.Equal(t, fmt.Sprintf("Growth %02d.pdf", n-1-i), row["filename"])
						require.Equal(t, "zeugnis", row["category"])
						require.Equal(t, float64(f.staff), row["staff_id"])
					}
				}
				if route == "schedule" {
					sort.Ints(scheduleIndices)
					for i, index := range scheduleIndices {
						require.Equal(t, i, index)
					}
				}
				t.Logf("route=%s N=%d statements=%d scope=full-resource-request", path, n, len(queries.Statements))
				counts = append(counts, len(queries.Statements))
				switch route {
				case "schedule":
					querybudgets.AssertQueryBudget(t, "workforce.staff_growth.schedule", queries)
				case "adjustments":
					querybudgets.AssertQueryBudget(t, "workforce.staff_growth.adjustments", queries)
				default:
					querybudgets.AssertQueryBudget(t, "workforce.staff_growth.documents", queries)
				}
			}
			require.Equal(t, counts[0], counts[1])
		})
	}
}

func TestStaffReportingPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	for _, route := range []string{"overview", "export", "export/datev-report", "month-close", "audit-log"} {
		t.Run(route, func(t *testing.T) {
			testpkg.OwnTenant(t)
			f := newStaffGrowthFixture(t)
			foreign := testpkg.NewTenantScope(t, f.db)
			foreignStaff := testpkg.CreateTestStaffForTenant(t, f.db, foreign.TenantID, "Foreign", "Reporting")
			// The export uses real registry-backed settings and real personnel numbers,
			// not the incomplete-configuration refusal path.
			if route == "export/datev-report" {
				for key, value := range map[string]string{"payroll.lohnart_regelarbeit": "100", "payroll.datev_beraternummer": "1234567", "payroll.datev_mandantennummer": "54321"} {
					testpkg.SetTestTenantSetting(t, f.db, testpkg.Tenant(t), key, strconv.Quote(value))
				}
			}
			add := func(tenant, staff int64, i int) {
				if route == "month-close" {
					staffGrowthInsert(t, f.db, "active.staff_month_balance_snapshots", map[string]any{"tenant_id": tenant, "staff_id": staff, "year": 2026, "month": 1, "closing_balance_minutes": 60 + i, "closed_by": staff, "close_reason": fmt.Sprintf("Growth %02d", i)})
					return
				}
				if route == "audit-log" {
					staffGrowthInsert(t, f.db, "active.staff_balance_adjustments", map[string]any{"tenant_id": tenant, "staff_id": staff, "type": "payout", "minutes_delta": -10 - i, "effective_date": testpkg.TodayDate().String(), "note": fmt.Sprintf("Growth %02d", i), "decided_by": staff, "decided_at": time.Now().Add(time.Duration(i) * time.Second)})
					return
				}
				staffGrowthInsert(t, f.db, "active.work_sessions", map[string]any{"tenant_id": tenant, "staff_id": staff, "date": "2026-01-05", "status": "present", "source": "app", "check_in_time": time.Date(2026, 1, 5, 8, 0, 0, 0, time.UTC), "check_out_time": time.Date(2026, 1, 5, 16, 0, 0, 0, time.UTC), "created_by": staff})
			}
			add(foreign.TenantID, foreignStaff.ID, 99)
			ids := []int64{f.staff}
			add(testpkg.Tenant(t), f.staff, 0)
			counts := []int{}
			for _, n := range []int{3, 8} {
				for len(ids) < n {
					i := len(ids)
					staff := testpkg.CreateTestStaff(t, f.db, "Growth", fmt.Sprintf("Report%02d", i))
					ids = append(ids, staff.ID)
					add(testpkg.Tenant(t), staff.ID, i)
				}
				if route == "export/datev-report" {
					for i, id := range ids {
						_, err := f.db.NewUpdate().Table("users.staff_employment_profiles").Set("personnel_number = ?", fmt.Sprintf("%d", 1000+i)).Where("membership_id = ?", id).Where("tenant_id = ?", testpkg.Tenant(t)).Exec(testpkg.Ctx(t))
						require.NoError(t, err)
					}
				}
				path := "/staff/time-tracking/" + route + "?year=2026&month=1"
				if route == "export/datev-report" {
					path += "&format=datev_lodas"
				}
				if route == "audit-log" {
					path = "/staff/time-tracking/audit-log?sources=adjustment&limit=100"
				}
				rec, queries := f.get(t, path)
				require.NotContains(t, rec.Body.String(), "Foreign")
				switch route {
				case "export":
					require.Equal(t, "text/csv; charset=utf-8", rec.Header().Get("Content-Type"))
					reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(rec.Body.String(), "\xef\xbb\xbf")))
					reader.Comma = ';'
					csvRows, err := reader.ReadAll()
					require.NoError(t, err)
					require.Len(t, csvRows, n+1)
					require.Equal(t, "Reader", csvRows[1][0])
					require.Equal(t, "Growth", csvRows[1][1])
					for i := 1; i < n; i++ {
						require.Equal(t, fmt.Sprintf("Report%02d", i), csvRows[i+1][0])
						require.Equal(t, "Growth", csvRows[i+1][1])
					}
				case "export/datev-report":
					obj := staffGrowthData(t, rec).(map[string]any)
					require.Equal(t, float64(n), obj["staff_exported"])
					require.Equal(t, float64(n), obj["line_count"])
					require.Empty(t, obj["staff_skipped"])
				default:
					data := staffGrowthData(t, rec)
					var rows []any
					switch route {
					case "overview":
						rows = data.(map[string]any)["rows"].([]any)
					case "audit-log":
						rows = data.(map[string]any)["events"].([]any)
					default:
						rows = data.([]any)
					}
					require.Len(t, rows, n)
					seen := map[string]bool{}
					for i, v := range rows {
						row := v.(map[string]any)
						seen[fmt.Sprint(row["staff_id"])] = true
						if route == "month-close" {
							require.Equal(t, float64(ids[i]), row["staff_id"])
							require.Equal(t, float64(60+i), row["closing_balance_minutes"])
						}
						if route == "audit-log" {
							require.Equal(t, "adjustment", row["source"])
							require.Equal(t, fmt.Sprintf("Growth %02d", n-1-i), row["reason"])
							require.Equal(t, float64(ids[n-1-i]), row["staff"].(map[string]any)["id"])
						}
						if route == "overview" {
							last := "Reader"
							if i > 0 {
								last = fmt.Sprintf("Report%02d", i)
							}
							require.Equal(t, last, row["last_name"])
						}
						if route == "overview" {
							require.Equal(t, float64(480), row["ist_minutes"])
						}
						if route == "month-close" {
							require.Equal(t, float64(2026), row["year"])
							require.Equal(t, float64(1), row["month"])
						}
					}
					if route != "audit-log" {
						for _, id := range ids {
							require.True(t, seen[strconv.FormatInt(id, 10)])
						}
					}
				}
				t.Logf("route=%s N=%d statements=%d scope=full-resource-request", path, n, len(queries.Statements))
				counts = append(counts, len(queries.Statements))
				switch route {
				case "overview":
					querybudgets.AssertQueryBudget(t, "workforce.staff_growth.overview", queries)
				case "export":
					querybudgets.AssertQueryBudget(t, "workforce.staff_growth.export", queries)
				case "export/datev-report":
					querybudgets.AssertQueryBudget(t, "workforce.staff_growth.datev_report", queries)
				case "month-close":
					querybudgets.AssertQueryBudget(t, "workforce.staff_growth.month_close", queries)
				default:
					querybudgets.AssertQueryBudget(t, "workforce.staff_growth.audit_log", queries)
				}
			}
			// Overview/export may retain genuine per-person work. Their separately
			// measured exact cardinality contracts must not invent a constant floor.
			if route == "month-close" || route == "audit-log" {
				require.Equal(t, counts[0], counts[1])
			}
		})
	}
}
