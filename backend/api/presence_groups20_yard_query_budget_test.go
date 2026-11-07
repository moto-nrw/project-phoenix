package api

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	yardHTTP "github.com/moto-nrw/project-phoenix/modules/supervisiondashboard/http"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestPresenceGroups20YardQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	database := testpkg.NewDatabase(t, db)
	unit := testpkg.TenantRuntime(t, db)
	facilities := NewActiveTestFacilities(database, unit)
	caller := NewActiveTestCaller(database, unit)
	staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Yard20", "Reader")
	var activity *facilitiesModule.SystemActivity
	err := tenant.WithinCurrentTenant(testpkg.Ctx(t), func(ctx context.Context) error {
		var createErr error
		activity, createErr = facilities.SchulhofService.EnsureInfrastructure(ctx, staff.ID)
		return createErr
	})
	require.NoError(t, err)
	require.NotNil(t, activity.PlannedRoomID)
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, *activity.PlannedRoomID)
	resource := yardHTTP.NewSchulhofResource(newSupervisionYard(facilities.SchulhofService), func(ctx context.Context) (int64, bool, error) { return caller.CurrentStaffID(ctx) })
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Route("/api/active", func(r chi.Router) {
		ProtectedTenantRoutes(r, func(r chi.Router, tx common.Middleware) { resource.Register(r, tx) })
	})
	token := testutil.MintTestJWT(t, testutil.AdminTestClaimsForTenant(int(account.ID), testpkg.Tenant(t)))
	foreign := testpkg.NewTenantScope(t, db)
	foreignGroup := testpkg.CreateTestActiveGroupForTenant(t, db, foreign.TenantID)
	_, err = db.NewUpdate().TableExpr("facilities.rooms").Set("name = ?", "Schulhof").Where("id = ?", foreignGroup.RoomID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	var expected []yardHTTP.SupervisorInfoResponse
	counter := testpkg.CaptureQueries(t, db)
	counter.Stop()
	type sample struct {
		n   int
		sql []string
	}
	var samples []sample
	for _, n := range []int{3, 8} {
		for len(expected) < n {
			i := len(expected)
			supervisor := staff
			if i > 0 {
				supervisor = testpkg.CreateTestStaff(t, db, "Yard20", fmt.Sprintf("Supervisor%02d", i))
			}
			row := testpkg.CreateTestGroupSupervisor(t, db, supervisor.ID, group.ID, "supervisor")
			// FacilitiesGroupSupervisions currently projects IDs, not names.
			// Preserve that wire contract; name hydration is a separate defect.
			name := " "
			expected = append(expected, yardHTTP.SupervisorInfoResponse{ID: row.ID, StaffID: supervisor.ID, Name: name, IsCurrentUser: i == 0})
			child := testpkg.CreateTestStudent(t, db, "Yard20", fmt.Sprintf("Child%02d", i), "Y20")
			testpkg.CreateTestVisit(t, db, child.ID, group.ID, time.Date(2026, 11, 6, 9, i, 0, 0, time.UTC), nil)
		}
		counter.Reset()
		counter.Start()
		rr := testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, "GET", "/api/active/schulhof/status", nil, testutil.WithJWTBearer(token)))
		counter.Stop()
		require.Equal(t, 200, rr.Code, rr.Body.String())
		require.Zero(t, counter.WriteRows())
		for _, op := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
			require.Empty(t, counter.Operation(op))
		}
		var body struct {
			Data yardHTTP.SchulhofStatusResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		require.Equal(t, yardHTTP.SchulhofStatusResponse{Exists: true, RoomID: activity.PlannedRoomID, RoomName: "Schulhof", ActivityGroupID: &activity.ID, ActiveGroupID: &group.ID, IsUserSupervising: true, SupervisionID: &expected[0].ID, SupervisorCount: n, StudentCount: n, Supervisors: expected}, body.Data)
		t.Logf("PRESENCE20_YARD n=%d statements=%d sql=%q body=%s", n, counter.Total(), counter.Queries(), rr.Body.String())
		samples = append(samples, sample{n, counter.Queries()})
	}
	denied := testutil.ExecuteRequest(router, testutil.NewRequest("GET", "/api/active/schulhof/status", nil))
	require.Equal(t, 401, denied.Code)
	wrong := testutil.AdminTestClaimsForTenant(int(account.ID), testpkg.Tenant(t))
	wrong.Scope = "school"
	denied = testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, "GET", "/api/active/schulhof/status", nil, testutil.WithJWTBearer(testutil.MintTestJWT(t, wrong))))
	require.Equal(t, 401, denied.Code)
	for _, s := range samples {
		t.Run(fmt.Sprintf("n%d", s.n), func(t *testing.T) {
			if s.n == 3 {
				querybudgets.AssertQueryBudget(t, "api.active.presence20.yard.n3", s.sql)
			} else {
				querybudgets.AssertQueryBudget(t, "api.active.presence20.yard.n8", s.sql)
			}
		})
	}
}
