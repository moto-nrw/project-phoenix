package compose_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/timetabletest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCareExitRosterCommandsPreservePlansAndOuterRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	facts := &timetabletest.FixedSessions{}
	module := timetabletest.NewWithSessions(t, db, facts)
	room := testpkg.CreateTestRoom(t, db, "Care exit roster")
	student := testpkg.CreateTestStudent(t, db, "Roster", "Roundtrip", "2a")
	after := testpkg.Date(2027, 9, 6)
	planned := testpkg.CreateTestActivityInstance(t, db, after.AddDays(1), room.ID, testpkg.ActivityInstanceOpts{})
	observed := testpkg.CreateTestActivityInstance(t, db, after.AddDays(2), room.ID, testpkg.ActivityInstanceOpts{})
	lastDay := testpkg.CreateTestActivityInstance(t, db, after, room.ID, testpkg.ActivityInstanceOpts{})
	testpkg.CreateTestInstanceStudent(t, db, planned.ID, student.ID, "absent")
	at := after.BerlinMidnight()
	observedRow := testpkg.CreateTestInstanceStudent(t, db, observed.ID, student.ID, "present", testpkg.InstanceStudentOpts{CheckedInAt: &at})
	testpkg.CreateTestInstanceStudent(t, db, lastDay.ID, student.ID, "expected")
	facts.Observed = []int64{observedRow.ID}
	studentIDs := []int64{student.ID}
	abort := errors.New("ledger write failed")
	err := testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
		if err := module.LockPlannedRosterForCareExit(txCtx, studentIDs, after.String()); err != nil {
			return err
		}
		rows, err := module.RemovePlannedRosterForCareExit(txCtx, studentIDs, after.String())
		if err != nil {
			return err
		}
		require.Len(t, rows, 1)
		assert.Equal(t, planned.ID, rows[0].InstanceID)
		return abort
	})
	require.ErrorIs(t, err, abort)
	before, err := module.ListInstanceStudents(ctx, timetable.InstanceStudentFilter{StudentIDs: studentIDs})
	require.NoError(t, err)
	require.Len(t, before, 3, "failed outer ledger write must roll back the roster removal")

	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	otherCtx := testpkg.ContextForTenant(ctx, otherTenant)
	foreign, err := module.RemovePlannedRosterForCareExit(otherCtx, studentIDs, after.String())
	require.NoError(t, err)
	assert.Empty(t, foreign)
	rows, err := module.RemovePlannedRosterForCareExit(ctx, studentIDs, after.String())
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, planned.ID, rows[0].InstanceID)
	restored, err := module.RestoreRosterForCareExit(otherCtx, studentIDs, rows)
	require.NoError(t, err)
	assert.Empty(t, restored, "a foreign tenant cannot replay the owner's snapshot")
	restored, err = module.RestoreRosterForCareExit(ctx, studentIDs, rows)
	require.NoError(t, err)
	assert.Len(t, restored, 1)
	restored, err = module.RestoreRosterForCareExit(ctx, studentIDs, rows)
	require.NoError(t, err)
	assert.Empty(t, restored, "replaying the same ledger must be idempotent")
	afterRestore, err := module.ListInstanceStudents(ctx, timetable.InstanceStudentFilter{InstanceIDs: []int64{planned.ID}})
	require.NoError(t, err)
	require.Len(t, afterRestore, 1)
	assert.Equal(t, planned.ID, afterRestore[0].InstanceID, "the plan is back; its attendance is Student Presence's to replay")
}
