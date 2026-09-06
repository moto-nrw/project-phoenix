package compose

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstanceAttendanceRecoveryRollsBackEarlierRowsOnSnapshotMismatch(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "recovery-rollback")
	instance := createOwnedActivityInstance(t, module, ctx, fixture, "2027-11-03", "08:00:00", "Recovery")
	first := testpkg.CreateTestStudent(t, db, "Recovery", "First", "3a")
	second := testpkg.CreateTestStudent(t, db, "Recovery", "Second", "3a")
	firstRow := createOwnedInstanceStudent(t, module, ctx, instance.ID, first.ID, timetable.InstanceAttendanceExpected)
	missingRow := createOwnedInstanceStudent(t, module, ctx, instance.ID, second.ID, timetable.InstanceAttendanceExpected)
	require.NoError(t, module.DeleteInstanceStudent(ctx, missingRow.ID))
	snapshot := []timetable.CompletionAttendance{
		{RowID: firstRow.ID, Status: timetable.InstanceAttendanceAbsent, Note: stringText("Restore this note")},
		{RowID: missingRow.ID, Status: timetable.InstanceAttendanceAbsent},
	}
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		require.NoError(t, module.LockInstanceStudentAssignments(txCtx, instance.ID))
		return module.RestoreInstanceStudentAttendance(txCtx, instance.ID, snapshot)
	})
	require.ErrorContains(t, err, "snapshot mismatch for attendance row")
	unchanged, err := module.FindInstanceStudent(ctx, firstRow.ID)
	require.NoError(t, err)
	assert.Equal(t, timetable.InstanceAttendanceExpected, unchanged.Status)
	assert.Nil(t, unchanged.Note)
	replacement := createOwnedInstanceStudent(t, module, ctx, instance.ID, second.ID, timetable.InstanceAttendanceExpected)
	snapshot[1].RowID = replacement.ID
	for range 2 {
		require.NoError(t, module.RestoreInstanceStudentAttendance(ctx, instance.ID, snapshot))
	}
	restored, err := module.FindInstanceStudent(ctx, firstRow.ID)
	require.NoError(t, err)
	assert.Equal(t, timetable.InstanceAttendanceAbsent, restored.Status)
	require.NotNil(t, restored.Note)
	assert.Equal(t, "Restore this note", *restored.Note)
}
