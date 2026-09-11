package active_test

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestSupervisionDashboardInputsEnforceRLS proves the supervision read
// projection (#2703) stays inside the tenant: two schools seed the same
// shape (session, supervisor, visit, attendance), the projection's named
// inputs are invisible across the tenant boundary under the least-privilege
// role, and each school's dashboard shows only its own child.
func TestSupervisionDashboardInputsEnforceRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	type fixture struct {
		ctx                                   context.Context
		tenantID, accountID, groupID, student int64
		tc                                    *testContext
		router                                chi.Router
		rows                                  map[string]int64
	}
	var fixtures []fixture
	for _, side := range []string{"own", "foreign"} {
		t.Run(side, func(t *testing.T) {
			testpkg.OwnTenant(t)
			ctx := testpkg.Ctx(t)
			tc, router := setupDashboardContext(t)
			teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "DashRLS", side)
			room := testpkg.CreateTestRoom(t, db, "DashRLS-"+side)
			activity := testpkg.CreateTestActivityGroup(t, db, "DashRLS-"+side)
			activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
			supervisor := testpkg.CreateTestGroupSupervisor(t, db, teacher.Staff.ID, activeGroup.ID, "supervisor")
			eduGroup := testpkg.CreateTestEducationGroup(t, db, "DashRLS-"+side)
			testpkg.CreateTestGroupTeacher(t, db, eduGroup.ID, teacher.ID)
			student := testpkg.CreateTestStudent(t, db, "DashRLS", side, "DR1")
			testpkg.AssignStudentToGroup(t, db, student.ID, eduGroup.ID)
			device := testpkg.CreateTestDevice(t, db, "DashRLS-"+side)
			checkIn := time.Now().Add(-time.Hour)
			attendance := testpkg.CreateTestAttendance(t, db, student.ID, teacher.Staff.ID, device.ID, checkIn, nil)
			visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, checkIn, nil)
			fixtures = append(fixtures, fixture{
				ctx: ctx, tenantID: testpkg.Tenant(t), accountID: account.ID, groupID: activeGroup.ID, student: student.ID, tc: tc, router: router,
				rows: map[string]int64{
					"active.groups":            activeGroup.ID,
					"active.group_supervisors": supervisor.ID,
					"active.visits":            visit.ID,
					"active.attendance":        attendance.ID,
				},
			})
		})
	}
	require.Len(t, fixtures, 2)
	for table, ownID := range fixtures[0].rows {
		t.Run(table, func(t *testing.T) {
			for _, fixture := range fixtures {
				require.NoError(t, testpkg.WithTenantTx(t, fixture.ctx, db, fixture.tenantID, func(txCtx context.Context, tx bun.Tx) error {
					var bypass bool
					require.NoError(t, tx.NewRaw("SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user").Scan(txCtx, &bypass))
					require.False(t, bypass, "the tenant transaction must run under the least-privilege role")
					var ids []int64
					require.NoError(t, tx.NewSelect().Table(table).Column("id").Where("id IN (?, ?)", ownID, fixtures[1].rows[table]).Scan(txCtx, &ids))
					require.Equal(t, []int64{fixture.rows[table]}, ids, "%s leaks across the tenant boundary", table)
					return nil
				}))
			}
		})
	}
	for _, fixture := range fixtures {
		claims := testutil.TeacherTestClaims(int(fixture.accountID))
		claims.TenantID = fixture.tenantID
		req := testutil.NewRequest("GET", fmt.Sprintf("/active/supervision-dashboard?group_id=%d", fixture.groupID), nil)
		rr := testutil.ExecuteWithAuthPermissions(t, fixture.router, req, claims, dashboardPerms)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		data := decodeDashboard(t, rr.Body.Bytes()).Data
		require.Len(t, data.Groups, 1, "each school sees exactly its own session")
		require.Equal(t, strconv.FormatInt(fixture.groupID, 10), data.Groups[0].ID)
		require.Len(t, data.Visits, 1, "both schools have populated projection inputs")
		require.Equal(t, strconv.FormatInt(fixture.student, 10), data.Visits[0].StudentID)
		require.NotNil(t, data.Visits[0].ActualArrivalTime, "the attendance row of the own tenant is visible")
	}
	// The foreign school's session is a hard 403, never a silently empty
	// projection: the error contract survives the cutover to the projection.
	claims := testutil.TeacherTestClaims(int(fixtures[0].accountID))
	claims.TenantID = fixtures[0].tenantID
	req := testutil.NewRequest("GET", fmt.Sprintf("/active/supervision-dashboard?group_id=%d", fixtures[1].groupID), nil)
	rr := testutil.ExecuteWithAuthPermissions(t, fixtures[0].router, req, claims, dashboardPerms)
	require.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
}
