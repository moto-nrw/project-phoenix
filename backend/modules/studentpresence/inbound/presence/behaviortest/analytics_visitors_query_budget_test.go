package presence_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	presenceAPI "github.com/moto-ogs/project-phoenix/modules/studentpresence/inbound/presence"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestDashboardAnalyticsPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc, router := setupProtectedRouter(t)
	created := 0
	add := func(n int) {
		for range n {
			room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Analytics Room %d", created))
			activity := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("Analytics Activity %d", created))
			group := testpkg.CreateTestActiveGroup(t, tc.db, activity.ID, room.ID)
			staff := testpkg.CreateTestStaff(t, tc.db, "Analytics", fmt.Sprintf("Staff%d", created))
			testpkg.CreateTestGroupSupervisor(t, tc.db, staff.ID, group.ID, "supervisor")
			child := testpkg.CreateTestStudent(t, tc.db, "Analytics", fmt.Sprintf("Child%d", created), "3a")
			testpkg.CreateTestVisit(t, tc.db, child.ID, group.ID, time.Now(), nil)
			created++
		}
	}
	counter := testpkg.CaptureQueries(t, tc.db)
	run := func() testpkg.QueryMeasurement {
		counter.Reset()
		rr := testutil.ExecuteWithAuthPermissions(t, testpkg.MountedQueryBudgetRouter("/api", router), testutil.NewRequest("GET", "/active/analytics/dashboard", nil), testutil.AdminTestClaims(1), []string{"admin:*"})
		queries := counter.Queries()
		require.Equal(t, testutil.StatusOK, rr.Code, rr.Body.String())
		var response struct {
			Data presenceAPI.DashboardAnalyticsResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
		require.Equal(t, created, response.Data.StudentsPresent)
		require.Equal(t, created, response.Data.ActiveActivities)
		require.Equal(t, created, response.Data.SupervisorsToday)
		require.Len(t, response.Data.CurrentActivities, min(created, 5))
		return counter.Measurement(queries)
	}
	add(3)
	small := run()
	add(5)
	large := run()
	require.Equal(t, len(small.Statements), len(large.Statements), "positive N3/N8 dashboard capture")
	t.Logf("dashboard full route N3=%d N8=%d", len(small.Statements), len(large.Statements))
	querybudgets.AssertQueryBudget(t, "api.active.analytics.dashboard.positive", small)
	querybudgets.AssertQueryBudget(t, "api.active.analytics.dashboard.positive", large)
}

func TestCrossTenantStudentsPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc, router := setupProtectedRouter(t)
	host := testpkg.Tenant(t)
	group := testpkg.CreateTestActiveGroupForTenant(t, tc.db, host)
	local := testpkg.CreateTestStudent(t, tc.db, "Local", "Excluded", "3a")
	testpkg.CreateTestVisit(t, tc.db, local.ID, group.ID, time.Now(), nil)
	want := make(map[int64]string)
	add := func(n int) {
		for range n {
			home := testpkg.UniqueTestTenantID(t)
			testpkg.EnsureTestTenant(t, tc.db, home)
			name := fmt.Sprintf("Visitor%d", len(want))
			child := testpkg.CreateTestStudentForTenant(t, tc.db, home, name, "Guest", "3a")
			testpkg.CreateTestVisitForTenant(t, tc.db, host, child.ID, group.ID, time.Now(), nil)
			want[child.ID] = name
		}
	}
	counter := testpkg.CaptureQueries(t, tc.db)
	run := func() testpkg.QueryMeasurement {
		counter.Reset()
		rr := testutil.ExecuteWithAuthPermissions(t, testpkg.MountedQueryBudgetRouter("/api", router), testutil.NewRequest("GET", "/active/cross-tenant-students", nil), testutil.AdminTestClaims(1), []string{"admin:*"})
		queries := counter.Queries()
		require.Equal(t, testutil.StatusOK, rr.Code, rr.Body.String())
		var response struct {
			Data []struct {
				ID         int64  `json:"student_id"`
				FirstName  string `json:"first_name"`
				LastName   string `json:"last_name"`
				HomeTenant string `json:"home_tenant"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
		require.Len(t, response.Data, len(want))
		seen := make(map[int64]bool)
		for _, row := range response.Data {
			require.Contains(t, want, row.ID)
			require.False(t, seen[row.ID])
			seen[row.ID] = true
			require.Equal(t, want[row.ID], row.FirstName)
			require.Equal(t, "Guest", row.LastName)
			require.NotEmpty(t, row.HomeTenant)
		}
		return counter.Measurement(queries)
	}
	add(3)
	small := run()
	add(5)
	large := run()
	require.Equal(t, len(small.Statements), len(large.Statements), "positive N3/N8 visitor capture with N distinct home schools")
	t.Logf("cross-tenant full route N3=%d N8=%d", len(small.Statements), len(large.Statements))
	querybudgets.AssertQueryBudget(t, "api.active.cross_tenant_students.positive", small)
	querybudgets.AssertQueryBudget(t, "api.active.cross_tenant_students.positive", large)
}
