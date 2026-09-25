package students_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Attendance/visits and instance students have independent RLS matrices in
// Presence and Class Day. Complete #2702's named inputs with the group tables.
func TestOGSGroupLiveGroupInputsEnforceRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	type fixture struct {
		ctx                                     context.Context
		tenantID, accountID, groupID, studentID int64
		tc                                      *testContext
		rows                                    map[string]int64
	}
	var fixtures []fixture
	for _, side := range []string{"own", "foreign"} {
		t.Run(side, func(t *testing.T) {
			testpkg.OwnTenant(t)
			ctx := testpkg.Ctx(t)
			tc := setupStudentsRoute(t, fixedCalendarClock)
			teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "LiveRLS", side)
			group := testpkg.CreateTestEducationGroup(t, db, "LiveRLS-"+side)
			testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)
			room := testpkg.CreateTestRoom(t, db, "LiveRLS-"+side)
			_, err := db.NewUpdate().Table("education.groups").Set("room_id = ?", room.ID).Where("id = ?", group.ID).Exec(ctx)
			require.NoError(t, err)
			activity := testpkg.CreateTestActivityGroup(t, db, "LiveRLS-"+side)
			activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
			supervisor := testpkg.CreateTestGroupSupervisor(t, db, teacher.Staff.ID, activeGroup.ID, "supervisor")
			student := testpkg.CreateTestStudent(t, db, "LiveRLS", side, "LR1")
			testpkg.AssignStudentToGroup(t, db, student.ID, group.ID)
			testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, fixedCalendarClock().Add(-time.Hour), nil)
			var combinedID, mappingID int64
			require.NoError(t, db.NewRaw("INSERT INTO active.combined_groups (tenant_id, start_time) VALUES (?, ?) RETURNING id", testpkg.Tenant(t), fixedCalendarClock().Add(-time.Hour)).Scan(ctx, &combinedID))
			require.NoError(t, db.NewRaw("INSERT INTO active.group_mappings (tenant_id, active_combined_group_id, active_group_id) VALUES (?, ?, ?) RETURNING id", testpkg.Tenant(t), combinedID, activeGroup.ID).Scan(ctx, &mappingID))
			fixtures = append(fixtures, fixture{ctx: ctx, tenantID: testpkg.Tenant(t), accountID: account.ID, groupID: group.ID, studentID: student.ID, tc: tc, rows: map[string]int64{"active.groups": activeGroup.ID, "active.combined_groups": combinedID, "active.group_mappings": mappingID, "active.group_supervisors": supervisor.ID}})
		})
	}
	for table, ownID := range fixtures[0].rows {
		t.Run(table, func(t *testing.T) {
			for _, fixture := range fixtures {
				require.NoError(t, testpkg.WithTenantTx(t, fixture.ctx, db, fixture.tenantID, func(txCtx context.Context, tx bun.Tx) error {
					var bypass bool
					require.NoError(t, tx.NewRaw("SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user").Scan(txCtx, &bypass))
					require.False(t, bypass)
					var ids []int64
					require.NoError(t, tx.NewSelect().Table(table).Column("id").Where("id IN (?, ?)", ownID, fixtures[1].rows[table]).Scan(txCtx, &ids))
					require.Equal(t, []int64{fixture.rows[table]}, ids)
					return nil
				}))
			}
		})
	}
	for _, fixture := range fixtures {
		claims := testutil.TeacherTestClaims(int(fixture.accountID))
		claims.TenantID = fixture.tenantID
		req := testutil.NewRequest("GET", fmt.Sprintf("/ogs-group-live?group_id=%d", fixture.groupID), nil)
		rr := authExec(t, fixture.tc, req, claims, ogsLivePerms)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var result ogsLiveEnvelope
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
		require.Len(t, result.Data.Students, 1, "both schools have populated live projection inputs")
		require.Equal(t, strconv.FormatInt(fixture.studentID, 10), result.Data.Students[0]["id"])
		require.Equal(t, new(strconv.FormatInt(fixture.groupID, 10)), result.Data.GroupID)
	}
}
