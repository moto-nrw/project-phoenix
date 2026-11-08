//go:build testhooks

package staff_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestStaffDirectoryPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	for _, route := range []string{"available", "by-role?role=lehrkraft", "documents-directory"} {
		t.Run(route, func(t *testing.T) {
			testpkg.OwnTenant(t)
			testpkg.SetupIsolatedTestDB(t)
			db := testpkg.SetupTestDB(t)
			router, err := api.StaffGrowthTestRouter(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
			require.NoError(t, err)
			router = testpkg.TenantRuntimeMiddleware(t, db)(router)
			foreign := testpkg.NewTenantScope(t, db)
			foreignStaff, foreignAccount := testpkg.CreateTestStaffWithAccountForTenant(t, db, foreign.TenantID, "Foreign", "Directory")
			foreignTeacher := map[string]any{"tenant_id": foreign.TenantID, "staff_id": foreignStaff.ID}
			_, err = db.NewInsert().Model(&foreignTeacher).TableExpr("users.teachers").Exec(testpkg.Ctx(t))
			require.NoError(t, err)
			testpkg.AssignLehrkraftSystemRole(t, db, foreignAccount.ID, foreign.TenantID)
			ids := []int64{}
			counts := []int{}
			for _, n := range []int{3, 8} {
				for len(ids) < n {
					i := len(ids)
					teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "Growth", fmt.Sprintf("Directory%02d", i))
					testpkg.AssignLehrkraftSystemRole(t, db, account.ID, testpkg.Tenant(t))
					ids = append(ids, teacher.StaffID)
					_, err := db.NewUpdate().Table("users.staff_employment_profiles").Set("staff_notes = ?", "Private growth note").Set("employment_type = ?", "full_time").Where("tenant_id = ?", testpkg.Tenant(t)).Where("membership_id = ?", teacher.StaffID).Exec(testpkg.Ctx(t))
					require.NoError(t, err)
				}
				claims := testutil.DefaultTestClaims()
				claims.Permissions = []string{"users:read", "staff:documents"}
				req := httptest.NewRequest(http.MethodGet, "/staff/"+route, nil)
				req.Header.Set("Authorization", "Bearer "+testutil.MintTestJWT(t, claims))
				// Full composed resource request, tenant transaction and enrichment;
				// fixture creation and token signing are outside capture.
				counter := testpkg.CaptureQueries(t, db)
				rec := httptest.NewRecorder()
				testpkg.MountedQueryBudgetRouter("/api", router).ServeHTTP(rec, req)
				counter.Stop()
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				var body struct {
					Data []map[string]any `json:"data"`
				}
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				require.Len(t, body.Data, n)
				seen := map[string]bool{}
				for _, row := range body.Data {
					seen[fmt.Sprint(row["id"])] = true
					require.NotContains(t, fmt.Sprint(row), "Foreign")
					require.NotContains(t, fmt.Sprint(row), "Private growth note")
					for _, private := range []string{"staff_notes", "employment_type", "birthday", "address_street"} {
						require.NotContains(t, row, private)
					}
				}
				// Every directory shape identifies the actual staff membership.
				{
					for _, id := range ids {
						require.True(t, seen[strconv.FormatInt(id, 10)])
					}
				}
				if route == "documents-directory" {
					for i, row := range body.Data {
						require.Equal(t, fmt.Sprintf("Growth Directory%02d", i), row["name"])
						require.Len(t, row, 4)
					}
				}
				queries := counter.Measurement(counter.Queries())
				counts = append(counts, len(queries.Statements))
				t.Logf("route=/staff/%s N=%d statements=%d scope=full-resource-request", route, n, len(queries.Statements))
				switch route {
				case "available":
					querybudgets.AssertQueryBudget(t, "workforce.staff_growth.directory_available", queries)
				case "documents-directory":
					querybudgets.AssertQueryBudget(t, "workforce.staff_growth.directory_documents", queries)
				default:
					querybudgets.AssertQueryBudget(t, "workforce.staff_growth.directory_role", queries)
				}
			}
			require.Equal(t, counts[0], counts[1])
		})
	}
}

func TestStaffShiftPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)
	router, err := api.StaffShiftGrowthTestRouter(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	router = testpkg.TenantRuntimeMiddleware(t, db)(router)
	staff := testpkg.CreateTestStaff(t, db, "Growth", "Shift")
	foreign := testpkg.NewTenantScope(t, db)
	foreignStaff := testpkg.CreateTestStaffForTenant(t, db, foreign.TenantID, "Foreign", "Shift")
	add := func(tenant, id int64, i int) {
		row := map[string]any{"tenant_id": tenant, "staff_id": id, "date": fmt.Sprintf("2026-01-%02d", i+1), "start_time": "08:00:00", "end_time": "16:00:00", "break_minutes": 30, "notes": fmt.Sprintf("Shift %02d", i), "created_by": id}
		_, err := db.NewInsert().Model(&row).TableExpr("schedule.staff_shifts").Exec(testpkg.Ctx(t))
		require.NoError(t, err)
	}
	add(foreign.TenantID, foreignStaff.ID, 0)
	counts := []int{}
	for _, n := range []int{3, 8} {
		start := 0
		if n == 8 {
			start = 3
		}
		for i := start; i < n; i++ {
			add(testpkg.Tenant(t), staff.ID, i)
		}
		claims := testutil.DefaultTestClaims()
		claims.Permissions = []string{"time_tracking:manage"}
		req := httptest.NewRequest(http.MethodGet, "/staff-shifts/?from=2026-01-01&to=2026-01-31", nil)
		req.Header.Set("Authorization", "Bearer "+testutil.MintTestJWT(t, claims))
		counter := testpkg.CaptureQueries(t, db)
		rec := httptest.NewRecorder()
		testpkg.MountedQueryBudgetRouter("/api", router).ServeHTTP(rec, req)
		counter.Stop()
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var body struct {
			Data []map[string]any `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Len(t, body.Data, n)
		for i, row := range body.Data {
			require.Equal(t, fmt.Sprintf("2026-01-%02d", i+1), row["date"])
			require.Equal(t, fmt.Sprintf("Shift %02d", i), row["notes"])
			require.Equal(t, strconv.FormatInt(staff.ID, 10), fmt.Sprint(row["staff_id"]))
			require.Equal(t, float64(30), row["break_minutes"])
		}
		queries := counter.Measurement(counter.Queries())
		counts = append(counts, len(queries.Statements))
		t.Logf("route=/staff-shifts/ N=%d statements=%d scope=full-resource-request", n, len(queries.Statements))
		querybudgets.AssertQueryBudget(t, "workforce.staff_growth.shifts", queries)
	}
	require.Equal(t, counts[0], counts[1])
}
