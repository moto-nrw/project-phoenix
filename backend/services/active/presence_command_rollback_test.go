package active_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingAttendanceWrite struct {
	activeService.StudentPresence
	afterWrite  error
	changedRows int
}

func (p *failingAttendanceWrite) EnsureAttendanceBatch(ctx context.Context, input []studentpresence.Attendance) ([]int64, error) {
	ids, err := p.StudentPresence.EnsureAttendanceBatch(ctx, input)
	if err == nil {
		p.changedRows = len(ids)
		err = p.afterWrite
	}
	return ids, err
}

func TestBatchAttendanceCommandsRollbackEveryRowAndRetry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module := testSchoolPresence(t, db)
	injected := errors.New("fail after batch attendance write")
	presence := &failingAttendanceWrite{StudentPresence: module}
	svc, broadcaster := newServiceWithBroadcaster(t, db, presence)
	testpkg.SetTenantRuntime(t, svc, db)
	testpkg.EnsureWebManualDevice(t, db)
	staff := testpkg.CreateTestStaff(t, db, "Batch", "Staff")
	first := testpkg.CreateTestStudent(t, db, "Batch", "First", "3a")
	second := testpkg.CreateTestStudent(t, db, "Batch", "Second", "3a")
	ids := []int64{first.ID, second.ID}
	filter := studentpresence.AttendanceFilter{StudentIDs: ids}
	for _, action := range []string{activeService.SchoolCheckinActionIn, activeService.SchoolCheckinActionOut} {
		broadcaster.Reset()
		presence.afterWrite = injected
		_, err := svc.ProcessSchoolCheckinBatch(ctx, ids, staff.ID, action)
		require.ErrorIs(t, err, injected)
		require.Equal(t, len(ids), presence.changedRows, "failure must follow both authoritative row changes")
		assert.Empty(t, broadcaster.Calls(), "failed batch must not publish a refresh")
		rows, err := module.ListAttendance(ctx, filter)
		require.NoError(t, err)
		if action == activeService.SchoolCheckinActionIn {
			assert.Empty(t, rows, "every inserted attendance must roll back")
		} else {
			require.Len(t, rows, len(ids))
			for _, row := range rows {
				assert.Nil(t, row.CheckOutTime, "every checkout must roll back")
			}
		}
		presence.afterWrite = nil
		result, err := svc.ProcessSchoolCheckinBatch(ctx, ids, staff.ID, action)
		require.NoError(t, err)
		require.Equal(t, len(ids), result.Succeeded)
		require.Len(t, result.Results, len(ids))
		for _, item := range result.Results {
			assert.True(t, item.Changed)
		}
		repeated, err := svc.ProcessSchoolCheckinBatch(ctx, ids, staff.ID, action)
		require.NoError(t, err)
		require.Len(t, repeated.Results, len(ids))
		for _, item := range repeated.Results {
			assert.True(t, item.OK)
			assert.False(t, item.Changed, "repeat must not create another state transition")
		}
		rows, err = module.ListAttendance(ctx, filter)
		require.NoError(t, err)
		require.Len(t, rows, len(ids), "retries must not duplicate attendance")
		for _, row := range rows {
			if action == activeService.SchoolCheckinActionIn {
				assert.Nil(t, row.CheckOutTime)
			} else {
				assert.NotNil(t, row.CheckOutTime)
			}
		}
	}
}

func (p *failingAttendanceWrite) EnsureAttendance(ctx context.Context, input studentpresence.Attendance) (studentpresence.Attendance, bool, error) {
	row, inserted, err := p.StudentPresence.EnsureAttendance(ctx, input)
	if err == nil {
		err = p.afterWrite
	}
	return row, inserted, err
}

func (p *failingAttendanceWrite) CloseAttendance(ctx context.Context, input studentpresence.AttendanceCheckout) ([]studentpresence.Attendance, error) {
	rows, err := p.StudentPresence.CloseAttendance(ctx, input)
	if err == nil {
		p.changedRows = len(rows)
		err = p.afterWrite
	}
	return rows, err
}

func TestAttendanceCommandsRollbackFacadeWritesAndRetry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module := testSchoolPresence(t, db)
	injected := errors.New("fail after authoritative attendance write")
	presence := &failingAttendanceWrite{StudentPresence: module, afterWrite: injected}
	svc, broadcaster := newServiceWithBroadcaster(t, db, presence)
	testpkg.SetTenantRuntime(t, svc, db)
	student := testpkg.CreateTestStudent(t, db, "Rollback", "Attendance", "3a")
	staff := testpkg.CreateTestStaff(t, db, "Rollback", "Staff")
	device := testpkg.CreateTestDevice(t, db, "attendance-command-rollback")
	filter := studentpresence.AttendanceFilter{StudentIDs: []int64{student.ID}}

	_, err := svc.CheckInStudent(ctx, student.ID, staff.ID, device.ID, true)
	require.ErrorIs(t, err, injected)
	rows, err := module.ListAttendance(ctx, filter)
	require.NoError(t, err)
	assert.Empty(t, rows, "failed command must not commit its attendance insert")
	assert.Empty(t, broadcaster.Calls(), "failed command must not emit a refresh")

	presence.afterWrite = nil
	first, err := svc.CheckInStudent(ctx, student.ID, staff.ID, device.ID, true)
	require.NoError(t, err)
	assert.True(t, first.Changed)
	retry, err := svc.CheckInStudent(ctx, student.ID, staff.ID, device.ID, true)
	require.NoError(t, err)
	assert.False(t, retry.Changed)
	assert.Equal(t, first.AttendanceID, retry.AttendanceID)
	rows, err = module.ListAttendance(ctx, filter)
	require.NoError(t, err)
	require.Len(t, rows, 1, "retry must not create a duplicate stay")

	broadcaster.Reset()
	presence.afterWrite = injected
	_, err = svc.CheckOutStudent(ctx, student.ID, staff.ID, true)
	require.ErrorIs(t, err, injected)
	rows, err = module.ListAttendance(ctx, filter)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Nil(t, rows[0].CheckOutTime, "failed command must not commit its checkout")
	assert.Empty(t, broadcaster.Calls(), "failed checkout must not emit a refresh")

	presence.afterWrite = nil
	closed, err := svc.CheckOutStudent(ctx, student.ID, staff.ID, true)
	require.NoError(t, err)
	assert.True(t, closed.Changed)
	closedAgain, err := svc.CheckOutStudent(ctx, student.ID, staff.ID, true)
	require.NoError(t, err)
	assert.False(t, closedAgain.Changed)
	rows, err = module.ListAttendance(ctx, filter)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.NotNil(t, rows[0].CheckOutTime)
}
