package compose

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModuleCareExitRemovesOnlyPlansAndRestoresSnapshots(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	room := testpkg.CreateTestRoom(t, db, "Care exit snapshot room")
	module, err := New(Dependencies{
		DB:       db,
		Students: StudentDirectoryFunc(func(context.Context) ([]TargetStudent, error) { return nil, nil }),
		Rooms: timetable.RoomDirectoryFunc(func(_ context.Context, ids []int64) ([]timetable.RoomRef, error) {
			assert.Equal(t, []int64{room.ID}, ids)
			return []timetable.RoomRef{{ID: room.ID, TenantID: testpkg.Tenant(t)}}, nil
		}),
		CareDays: testCareDays(), CarePlan: unusedCarePlanDirectory{}, Observe: func(Observation) {},
	})
	require.NoError(t, err)
	fixture := newOwnedActivityInstanceFixture(t, db, "care-exit-roster")
	student := testpkg.CreateTestStudent(t, db, "CareExit", "Roster", "3a")
	ids := []int64{student.ID}
	last := createOwnedActivityInstance(t, module, ctx, fixture, "2027-11-01", "08:00:00", "Last care day")
	future := createOwnedActivityInstance(t, module, ctx, fixture, "2027-11-02", "08:00:00", "Future plan")
	actual := createOwnedActivityInstance(t, module, ctx, fixture, "2027-11-03", "08:00:00", "Recorded presence")
	createOwnedInstanceStudent(t, module, ctx, last.ID, student.ID, timetable.InstanceAttendanceExpected)
	input := ownedInstanceStudentInput(future.ID, student.ID, timetable.InstanceAttendanceAbsent)
	input.Note = stringText("Planned absence")
	input.RoomID = &room.ID
	_, err = module.CreateInstanceStudent(ctx, input)
	require.NoError(t, err)
	input = ownedInstanceStudentInput(actual.ID, student.ID, timetable.InstanceAttendancePresent)
	at := time.Date(2027, 11, 3, 8, 0, 0, 0, time.UTC)
	input.CheckedInAt = &at
	_, err = module.CreateInstanceStudent(ctx, input)
	require.NoError(t, err)
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
	assert.Equal(t, "Planned absence", *removed[0].Note)
	rows, err := module.RestoreRosterForCareExit(ctx, ids, removed)
	require.NoError(t, err)
	assert.EqualValues(t, 1, rows)
	rows, err = module.RestoreRosterForCareExit(ctx, ids, removed)
	require.NoError(t, err)
	assert.Zero(t, rows)
	assignments, err := module.ListInstanceStudents(ctx, timetable.InstanceStudentFilter{StudentIDs: ids})
	require.NoError(t, err)
	assert.Len(t, assignments, 3)
	for _, assignment := range assignments {
		if assignment.InstanceID == future.ID {
			assert.Equal(t, &room.ID, assignment.RoomID, "restore resolves valid room references itself")
			assert.Equal(t, stringText("Planned absence"), assignment.Note)
			assert.Equal(t, timetable.InstanceAttendanceAbsent, assignment.Status)
		}
	}
}

func TestModuleCareExitClosesOnlyRecordedOpenPresence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "care-exit-presence")
	student := testpkg.CreateTestStudent(t, db, "CareExit", "Presence", "3a")
	planned := testpkg.CreateTestStudent(t, db, "CareExit", "Planned", "3a")
	instance := createOwnedActivityInstance(t, module, ctx, fixture, "2027-11-01", "08:00:00", "Presence")
	input := ownedInstanceStudentInput(instance.ID, student.ID, timetable.InstanceAttendancePresent)
	at := time.Date(2027, 11, 1, 8, 0, 0, 0, time.UTC)
	input.CheckedInAt = &at
	_, err := module.CreateInstanceStudent(ctx, input)
	require.NoError(t, err)
	createOwnedInstanceStudent(t, module, ctx, instance.ID, planned.ID, timetable.InstanceAttendancePresent)
	ids := []int64{student.ID, planned.ID}
	open, err := module.ListOpenStudentAssignments(ctx, ids)
	require.NoError(t, err)
	assert.Equal(t, []int64{student.ID}, open)
	day, err := module.LatestStudentAssignmentAttendanceDate(ctx, student.ID)
	require.NoError(t, err)
	require.NotNil(t, day)
	assert.Equal(t, "2027-11-01", *day)
	day, err = module.LatestStudentAssignmentAttendanceDate(ctx, planned.ID)
	require.NoError(t, err)
	assert.Nil(t, day, "a status alone is not a recorded check-in")
	rows, err := module.CloseOpenStudentAssignments(ctx, ids, at.Add(time.Hour))
	require.NoError(t, err)
	assert.EqualValues(t, 1, rows)
	rows, err = module.CloseOpenStudentAssignments(ctx, ids, at.Add(2*time.Hour))
	require.NoError(t, err)
	assert.Zero(t, rows)
	open, err = module.ListOpenStudentAssignments(ctx, ids)
	require.NoError(t, err)
	assert.Empty(t, open)
}

func TestModuleCareExitAssignmentsRespectTwoTenantRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "care-exit-own")
	instance := createOwnedActivityInstance(t, module, ctx, fixture, "2027-11-02", "08:00:00", "Own")
	student := testpkg.CreateTestStudent(t, db, "CareExit", "Own", "3a")
	own := createOwnedInstanceStudent(t, module, ctx, instance.ID, student.ID, timetable.InstanceAttendanceExpected)
	var foreign timetable.InstanceStudent
	var foreignCtx context.Context
	t.Run("foreign fixture", func(t *testing.T) {
		testpkg.OwnTenant(t)
		foreignCtx = testpkg.Ctx(t)
		foreignFixture := newOwnedActivityInstanceFixture(t, db, "care-exit-foreign")
		foreignInstance := createOwnedActivityInstance(t, module, foreignCtx, foreignFixture, "2027-11-02", "08:00:00", "Foreign")
		foreignStudent := testpkg.CreateTestStudent(t, db, "CareExit", "Foreign", "3a")
		foreign = createOwnedInstanceStudent(t, module, foreignCtx, foreignInstance.ID, foreignStudent.ID, timetable.InstanceAttendanceExpected)
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
		assert.EqualValues(t, 1, restored)
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
	_, err = module.LatestStudentAssignmentAttendanceDate(cancelled, student.ID)
	require.ErrorIs(t, err, context.Canceled)
	_, err = module.ListOpenStudentAssignments(cancelled, []int64{student.ID})
	require.ErrorIs(t, err, context.Canceled)
}

func careExitSnapshot(row timetable.InstanceStudent) timetable.CareExitRosterRow {
	return timetable.CareExitRosterRow{
		TenantID: row.TenantID, StudentID: row.StudentID, InstanceID: row.InstanceID,
		RoomID: row.RoomID, Status: row.Status, Substatus: row.Substatus, Note: row.Note,
		IsUnplanned: row.IsUnplanned, NotScheduled: row.NotScheduled, ManualStatusAt: row.ManualStatusAt,
		StudentStatusDayID: row.StudentStatusDayID, PickupExceptionID: row.PickupExceptionID,
	}
}
