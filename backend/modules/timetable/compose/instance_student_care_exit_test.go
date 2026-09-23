package compose

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A care exit takes the planned rows after the last day and leaves the ones
// Student Presence observed; the restore brings the plan back with its room
// and returns the new participant ids so the attendance can follow.
func TestModuleCareExitRemovesOnlyPlansAndRestoresSnapshots(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	room := testpkg.CreateTestRoom(t, db, "Care exit snapshot room")
	facts := &fixedSessions{}
	module, err := New(Dependencies{
		LockStaffAssignment: func(context.Context, int64) error { return nil },
		DB:                  db,
		Students:            StudentDirectoryFunc(func(context.Context) ([]TargetStudent, error) { return nil, nil }),
		Rooms: timetable.RoomDirectoryFunc(func(_ context.Context, ids []int64) ([]timetable.RoomRef, error) {
			assert.Equal(t, []int64{room.ID}, ids)
			return []timetable.RoomRef{{ID: room.ID, TenantID: testpkg.Tenant(t)}}, nil
		}),
		Sessions: facts, Observe: func(Observation) {},
	})
	require.NoError(t, err)
	fixture := newOwnedActivityInstanceFixture(t, db, "care-exit-roster")
	student := testpkg.CreateTestStudent(t, db, "CareExit", "Roster", "3a")
	ids := []int64{student.ID}
	last := createOwnedActivityInstance(t, module, ctx, fixture, "2027-11-01", "08:00:00", "Last care day")
	future := createOwnedActivityInstance(t, module, ctx, fixture, "2027-11-02", "08:00:00", "Future plan")
	actual := createOwnedActivityInstance(t, module, ctx, fixture, "2027-11-03", "08:00:00", "Recorded presence")
	createOwnedInstanceStudent(t, module, ctx, last.ID, student.ID)
	input := ownedInstanceStudentInput(future.ID, student.ID)
	input.RoomID = &room.ID
	planned, err := module.CreateInstanceStudent(ctx, input)
	require.NoError(t, err)
	observed := createOwnedInstanceStudent(t, module, ctx, actual.ID, student.ID)
	facts.observed = []int64{observed.ID}

	preview, err := module.PreviewPlannedRosterForCareExit(ctx, ids, "2027-11-01")
	require.NoError(t, err)
	require.Len(t, preview, 1)
	assert.Equal(t, planned.ID, preview[0].ParticipantID)
	abort := errors.New("fail after removing roster plans")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		removed, removeErr := module.RemovePlannedRosterForCareExit(txCtx, ids, "2027-11-01")
		require.NoError(t, removeErr)
		require.Len(t, removed, 1)
		return abort
	})
	require.ErrorIs(t, err, abort)
	removed, err := module.RemovePlannedRosterForCareExit(ctx, ids, "2027-11-01")
	require.NoError(t, err)
	require.Len(t, removed, 1)
	assert.Equal(t, future.ID, removed[0].InstanceID)
	assert.Equal(t, planned.ID, removed[0].ParticipantID)
	assert.Equal(t, &room.ID, removed[0].RoomID)
	rows, err := module.RestoreRosterForCareExit(ctx, ids, removed)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, future.ID, rows[0].InstanceID)
	assert.NotZero(t, rows[0].ParticipantID)
	assert.NotEqual(t, planned.ID, rows[0].ParticipantID, "a restored participant is a new row")
	rows, err = module.RestoreRosterForCareExit(ctx, ids, removed)
	require.NoError(t, err)
	assert.Empty(t, rows)
	assignments, err := module.ListInstanceStudents(ctx, timetable.InstanceStudentFilter{StudentIDs: ids})
	require.NoError(t, err)
	assert.Len(t, assignments, 3)
	for _, assignment := range assignments {
		if assignment.InstanceID == future.ID {
			assert.Equal(t, &room.ID, assignment.RoomID, "restore resolves valid room references itself")
		}
	}
}

// A block that ended keeps its roster: the exit neither removes nor restores
// rows on it.
func TestModuleCareExitLeavesEndedBlocksAlone(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	facts := &fixedSessions{}
	module := buildModuleWithSessions(t, db, facts)
	fixture := newOwnedActivityInstanceFixture(t, db, "care-exit-ended")
	student := testpkg.CreateTestStudent(t, db, "CareExit", "Ended", "3a")
	ids := []int64{student.ID}
	ended := createOwnedActivityInstance(t, module, ctx, fixture, "2027-11-02", "08:00:00", "Ended")
	open := createOwnedActivityInstance(t, module, ctx, fixture, "2027-11-03", "08:00:00", "Open")
	createOwnedInstanceStudent(t, module, ctx, ended.ID, student.ID)
	planned := createOwnedInstanceStudent(t, module, ctx, open.ID, student.ID)
	facts.completed = []int64{ended.ID}

	removed, err := module.RemovePlannedRosterForCareExit(ctx, ids, "2027-11-01")
	require.NoError(t, err)
	require.Len(t, removed, 1)
	assert.Equal(t, planned.ID, removed[0].ParticipantID)
	facts.completed = append(facts.completed, open.ID)
	rows, err := module.RestoreRosterForCareExit(ctx, ids, removed)
	require.NoError(t, err)
	assert.Empty(t, rows, "a block that ended meanwhile takes no restored participant")
}

func TestModuleCareExitAssignmentsRespectTwoTenantRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "care-exit-own")
	instance := createOwnedActivityInstance(t, module, ctx, fixture, "2027-11-02", "08:00:00", "Own")
	student := testpkg.CreateTestStudent(t, db, "CareExit", "Own", "3a")
	own := createOwnedInstanceStudent(t, module, ctx, instance.ID, student.ID)
	var foreign timetable.InstanceStudent
	var foreignCtx context.Context
	t.Run("foreign fixture", func(t *testing.T) {
		testpkg.OwnTenant(t)
		foreignCtx = testpkg.Ctx(t)
		foreignFixture := newOwnedActivityInstanceFixture(t, db, "care-exit-foreign")
		foreignInstance := createOwnedActivityInstance(t, module, foreignCtx, foreignFixture, "2027-11-02", "08:00:00", "Foreign")
		foreignStudent := testpkg.CreateTestStudent(t, db, "CareExit", "Foreign", "3a")
		foreign = createOwnedInstanceStudent(t, module, foreignCtx, foreignInstance.ID, foreignStudent.ID)
	})
	ids := []int64{student.ID, foreign.StudentID}
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		count, err := module.CountStudentAssignments(txCtx, foreign.StudentID)
		require.NoError(t, err)
		assert.Zero(t, count)
		require.NoError(t, module.LockPlannedRosterForCareExit(txCtx, ids, "2027-11-01"))
		removed, err := module.RemovePlannedRosterForCareExit(txCtx, ids, "2027-11-01")
		require.NoError(t, err)
		require.Len(t, removed, 1)
		assert.Equal(t, own.InstanceID, removed[0].InstanceID)
		restored, err := module.RestoreRosterForCareExit(txCtx, ids, []timetable.CareExitRosterRow{careExitSnapshot(own), careExitSnapshot(foreign)})
		require.NoError(t, err)
		assert.Len(t, restored, 1)
		deleted, err := module.DeleteStudentAssignments(txCtx, foreign.StudentID)
		require.NoError(t, err)
		assert.Zero(t, deleted)
		return nil
	})
	require.NoError(t, err)
	_, err = module.FindInstanceStudent(foreignCtx, foreign.ID)
	require.NoError(t, err)
}

func TestModuleCareExitAssignmentReadErrorsAreNotSwallowed(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "CareExit", "Errors", "3a")
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err := module.CountStudentAssignments(cancelled, student.ID)
	require.ErrorIs(t, err, context.Canceled)
	_, err = module.RemovePlannedRosterForCareExit(cancelled, []int64{student.ID}, "2027-11-01")
	require.ErrorIs(t, err, context.Canceled)
	_, err = module.PreviewPlannedRosterForCareExit(cancelled, []int64{student.ID}, "2027-11-01")
	require.ErrorIs(t, err, context.Canceled)
}

func careExitSnapshot(row timetable.InstanceStudent) timetable.CareExitRosterRow {
	return timetable.CareExitRosterRow{
		ParticipantID: row.ID, TenantID: row.TenantID, StudentID: row.StudentID, InstanceID: row.InstanceID, RoomID: row.RoomID,
	}
}
