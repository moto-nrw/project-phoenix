package schedule_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/services"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Cancel and kiosk end must both lock the live group before its attendance.
// Hold the workflow's first lock, start real Cancel, then finish the real
// workflow. An inverse attendance lock makes the workflow time out.
func TestSessionEndSerializesWithTimetableCancel(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module, err := services.NewActiveTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	activity := testpkg.CreateTestActivityGroup(t, db, "Concurrent session end")
	room := testpkg.CreateTestRoom(t, db, "Concurrent session end")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	student := testpkg.CreateTestStudent(t, db, "Concurrent", "Child", "1a")
	checkedIn := time.Now().Add(-time.Hour)
	visit := testpkg.CreateTestVisit(t, db, student.ID, group.ID, checkedIn, nil)
	instance := testpkg.CreateTestActivityInstance(t, db, testpkg.TodayDate(), room.ID, testpkg.ActivityInstanceOpts{
		Status: "active", ActivityGroupID: &activity.ID, ActiveGroupID: &group.ID, IsSpontaneous: true,
	})
	assignment := testpkg.CreateTestInstanceStudent(t, db, instance.ID, student.ID, "present", testpkg.InstanceStudentOpts{CheckedInAt: &checkedIn})
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 15*time.Second)
	defer cancel()
	cancelDone := make(chan error, 1)
	endErr := testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
		var lockedID int64
		if err := tx.NewRaw("SELECT id FROM active.groups WHERE id = ? FOR UPDATE", group.ID).Scan(txCtx, &lockedID); err != nil {
			return err
		}
		go func() {
			_, err := module.Instance.Cancel(ctx, instance.ID, nil, nil)
			cancelDone <- err
		}()
		require.Eventually(t, func() bool {
			var waiting int
			err := db.NewRaw("SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'").Scan(ctx, &waiting)
			return err == nil && waiting > 0
		}, 5*time.Second, 10*time.Millisecond, "Cancel must wait for the workflow's group lock")
		if _, err := tx.NewRaw("SET LOCAL lock_timeout = '250ms'").Exec(txCtx); err != nil {
			return err
		}
		_, err := module.SessionEnd.EndSession(txCtx, group.ID)
		return err
	})
	var cancelErr error
	select {
	case cancelErr = <-cancelDone:
	case <-ctx.Done():
		t.Fatal("Cancel did not finish after the workflow released its group lock")
	}
	require.NoError(t, endErr, "the group-lock holder must not wait on attendance held by Cancel")
	require.ErrorIs(t, cancelErr, scheduleService.ErrInvalidInstanceTransition, "Cancel must reject the session completed while it waited")
	require.Equal(t, "completed", testpkg.InstanceStatus(t, db, instance.ID))
	require.NotNil(t, testpkg.VisitExitTime(t, db, visit.ID))
	require.NotNil(t, testpkg.InstanceStudentByID(t, db, assignment.ID).CheckedOutAt)
}
