package schedule_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// #2697 names daily attendance and staff assignments even though ending a
// room session must not change either. Cover real rows, not empty tables,
// alongside rollback and RLS for every named table.
func TestSessionEndPreservesAttendanceAndStaffAcrossTenants(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewActiveTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	type fixture struct{ tenantID, groupID, instanceID int64 }
	create := func(t *testing.T, label string) fixture {
		t.Helper()
		activity := testpkg.CreateTestActivityGroup(t, db, label)
		room := testpkg.CreateTestRoom(t, db, label)
		group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		staff := testpkg.CreateTestStaff(t, db, "Preserved", label)
		device := testpkg.CreateTestDevice(t, db, label)
		student := testpkg.CreateTestStudent(t, db, "Preserved", label, "1a")
		testpkg.CreateTestGroupSupervisor(t, db, staff.ID, group.ID, "supervisor")
		checkedIn := time.Now().Add(-time.Hour)
		testpkg.CreateTestVisit(t, db, student.ID, group.ID, checkedIn, nil)
		testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, checkedIn, nil)
		instance := testpkg.CreateTestActivityInstance(t, db, testpkg.TodayDate(), room.ID, testpkg.ActivityInstanceOpts{
			Status: "active", ActivityGroupID: &activity.ID, ActiveGroupID: &group.ID, IsSpontaneous: true,
		})
		testpkg.CreateTestInstanceStudent(t, db, instance.ID, student.ID, "present", testpkg.InstanceStudentOpts{CheckedInAt: &checkedIn})
		testpkg.CreateTestInstanceStaff(t, db, instance.ID, staff.ID, testpkg.InstanceStaffOpts{IsPrimary: true})
		return fixture{testpkg.Tenant(t), group.ID, instance.ID}
	}
	own := create(t, "Own session preservation")
	var foreign fixture
	t.Run("foreign fixture", func(t *testing.T) {
		testpkg.OwnTenant(t)
		foreign = create(t, "Foreign session preservation")
	})
	tables := []string{
		"schedule.activity_instances", "schedule.instance_staff", "schedule.instance_students",
		"active.attendance", "active.visits", "active.groups", "active.group_supervisors",
	}
	snapshot := func(tenantID int64) map[string][]string {
		rows := make(map[string][]string, len(tables))
		for _, table := range tables {
			var values []string
			require.NoError(t, db.NewRaw("SELECT row_to_json(snapshot)::text FROM "+table+" AS snapshot WHERE tenant_id = ? ORDER BY id", tenantID).Scan(context.Background(), &values))
			require.NotEmpty(t, values, "fixture must populate %s", table)
			rows[table] = values
		}
		return rows
	}
	ownBefore, foreignBefore := snapshot(own.tenantID), snapshot(foreign.tenantID)
	// Probe actual least-privilege RLS in both directions over populated rows.
	for _, pair := range [][2]int64{{own.tenantID, foreign.tenantID}, {foreign.tenantID, own.tenantID}} {
		ctx := testpkg.ContextForTenant(testpkg.Ctx(t), pair[0])
		require.NoError(t, testpkg.WithTenantTx(t, ctx, db, pair[0], func(txCtx context.Context, tx bun.Tx) error {
			for _, table := range tables {
				var visible int
				require.NoError(t, tx.NewRaw("SELECT count(*) FROM "+table+" WHERE tenant_id = ?", pair[0]).Scan(txCtx, &visible))
				require.Positive(t, visible, "own rows in %s must be visible", table)
				require.NoError(t, tx.NewRaw("SELECT count(*) FROM "+table+" WHERE tenant_id = ?", pair[1]).Scan(txCtx, &visible))
				require.Zero(t, visible, "foreign rows in %s must be hidden", table)
			}
			return nil
		}))
	}
	abort := errors.New("abort after real session-end owners")
	err = testpkg.WithTenantTx(t, testpkg.Ctx(t), db, own.tenantID, func(txCtx context.Context, tx bun.Tx) error {
		result, err := module.SessionEnd.EndSession(txCtx, own.groupID)
		require.NoError(t, err)
		require.Equal(t, 1, result.StudentsCheckedOut)
		var status string
		require.NoError(t, tx.NewRaw("SELECT status FROM schedule.activity_instances WHERE id = ?", own.instanceID).Scan(txCtx, &status))
		require.Equal(t, "completed", status, "real Timetable mutation must happen before injected failure")
		return abort
	})
	require.ErrorIs(t, err, abort)
	require.Equal(t, ownBefore, snapshot(own.tenantID), "all owner writes must roll back")
	require.Equal(t, foreignBefore, snapshot(foreign.tenantID))

	_, err = module.SessionEnd.EndSession(testpkg.Ctx(t), own.groupID)
	require.NoError(t, err)
	ownAfter := snapshot(own.tenantID)
	for _, table := range []string{"schedule.instance_staff", "active.attendance"} {
		require.Equal(t, ownBefore[table], ownAfter[table], "session end must preserve every column of %s", table)
	}
	require.Equal(t, foreignBefore, snapshot(foreign.tenantID), "successful end must preserve every foreign row")
	require.Equal(t, "completed", testpkg.InstanceStatus(t, db, own.instanceID))
	require.Equal(t, "active", testpkg.InstanceStatus(t, db, foreign.instanceID))
}
