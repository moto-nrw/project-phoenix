package compose_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClosePresenceRollsBackEachWriteAndRetriesIdempotently(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	first := testpkg.CreateTestStudent(t, db, "Attendance", "Only", "3a")
	second := testpkg.CreateTestStudent(t, db, "Attendance", "Visit", "3a")
	staff := testpkg.CreateTestStaff(t, db, "Presence", "Staff")
	device := testpkg.CreateTestDevice(t, db, "presence-care-exit")
	activity := testpkg.CreateTestActivityGroup(t, db, "Presence care exit")
	room := testpkg.CreateTestRoom(t, db, "Presence care exit")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	at := time.Now()
	testpkg.CreateTestAttendance(t, db, first.ID, staff.ID, device.ID, at.Add(-2*time.Hour), nil)
	testpkg.CreateTestAttendance(t, db, second.ID, staff.ID, device.ID, at.Add(-2*time.Hour), nil)
	testpkg.CreateTestVisit(t, db, second.ID, group.ID, at, nil)
	ids := []int64{first.ID, second.ID}
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, err := module.CloseOpenPresence(txCtx, ids, at.Add(-time.Hour))
		return err
	})
	require.Error(t, err, "visit time constraint fails after attendance was closed")
	open, err := module.ListOpenPresence(ctx, []int64{first.ID})
	require.NoError(t, err)
	assert.Equal(t, []int64{first.ID}, open, "attendance close must roll back with the failed visit close")
	abort := errors.New("failure after both presence writes")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		rows, err := module.CloseOpenPresence(txCtx, ids, at.Add(time.Hour))
		if err != nil {
			return err
		}
		assert.EqualValues(t, 3, rows)
		return abort
	})
	require.ErrorIs(t, err, abort)
	for _, expectedRows := range []int64{3, 0} {
		require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
			rows, err := module.CloseOpenPresence(txCtx, ids, at.Add(time.Hour))
			assert.Equal(t, expectedRows, rows)
			return err
		}))
	}
	open, err = module.ListOpenPresence(ctx, ids)
	require.NoError(t, err)
	assert.Empty(t, open)
}

func TestPresenceHistoryUsesBerlinVisitDateAndPreservesReadFailures(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	student := testpkg.CreateTestStudent(t, db, "History", "Presence", "3a")
	staff := testpkg.CreateTestStaff(t, db, "History", "Staff")
	device := testpkg.CreateTestDevice(t, db, "presence-history")
	activity := testpkg.CreateTestActivityGroup(t, db, "History")
	room := testpkg.CreateTestRoom(t, db, "History")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	day, err := module.LatestPresenceDate(ctx, student.ID)
	require.NoError(t, err)
	assert.Nil(t, day)
	at := time.Date(2027, 7, 1, 22, 30, 0, 0, time.UTC)
	testpkg.CreateTestAttendanceForDate(t, db, student.ID, staff.ID, device.ID, testpkg.Date(2027, 7, 1), at.Add(-time.Hour), &at)
	testpkg.CreateTestVisit(t, db, student.ID, group.ID, at, nil)
	day, err = module.LatestPresenceDate(ctx, student.ID)
	require.NoError(t, err)
	require.NotNil(t, day)
	assert.Equal(t, "2027-07-02", *day)
	count, err := module.CountAttendanceRecords(ctx, student.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	require.Error(t, module.LockOpenPresence(ctx, []int64{student.ID}), "row locks require a transaction")
	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		return module.LockOpenPresence(txCtx, []int64{student.ID})
	}))
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = module.LatestPresenceDate(cancelled, student.ID)
	require.ErrorIs(t, err, context.Canceled)
	_, err = module.CountAttendanceRecords(cancelled, student.ID)
	require.ErrorIs(t, err, context.Canceled)
	_, err = module.ListOpenPresence(cancelled, []int64{student.ID})
	require.ErrorIs(t, err, context.Canceled)
}
