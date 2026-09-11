package compose_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaleAttendanceClosureRollsBackAndDoesNotRewriteDeparture(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	student := testpkg.CreateTestStudent(t, db, "Stale", "Attendance", "3a")
	device := testpkg.CreateTestDevice(t, db, "stale-attendance-retry")
	day := testpkg.TodayDate().AddDays(-1)
	row, err := module.RecordAttendance(ctx, studentpresence.Attendance{StudentID: student.ID, DeviceID: device.ID, Date: day.String(), CheckInTime: day.BerlinMidnight().Add(time.Hour)})
	require.NoError(t, err)
	departure := day.EndOfDay()
	updatedAt := time.Now()
	injected := errors.New("failure after stale attendance closure")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		changed, err := module.CloseStaleAttendance(txCtx, row.ID, departure, updatedAt)
		require.NoError(t, err)
		require.EqualValues(t, 1, changed)
		return injected
	})
	require.ErrorIs(t, err, injected)
	stored, err := module.FindAttendance(ctx, row.ID)
	require.NoError(t, err)
	assert.Nil(t, stored.CheckOutTime)
	changed, err := module.CloseStaleAttendance(ctx, row.ID, departure, updatedAt)
	require.NoError(t, err)
	require.EqualValues(t, 1, changed)
	stored, err = module.FindAttendance(ctx, row.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.CheckOutTime)
	firstUpdatedAt := stored.UpdatedAt
	changed, err = module.CloseStaleAttendance(ctx, row.ID, departure.Add(time.Minute), updatedAt.Add(time.Minute))
	require.NoError(t, err)
	assert.Zero(t, changed, "a stale retry must not overwrite a completed stay")
	stored, err = module.FindAttendance(ctx, row.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.CheckOutTime)
	assert.WithinDuration(t, departure, *stored.CheckOutTime, time.Microsecond)
	assert.Equal(t, firstUpdatedAt, stored.UpdatedAt)
}

func TestAttendanceFacadeKeepsCompletedStaysAndRetriesExplicitActions(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	student := testpkg.CreateTestStudent(t, db, "Owner", "Attendance", "3a")
	device := testpkg.CreateTestDevice(t, db, "owner-attendance")
	at := time.Now()
	input := studentpresence.Attendance{StudentID: student.ID, Date: testpkg.TodayDate().String(), CheckInTime: at, DeviceID: device.ID}
	first, inserted, err := module.EnsureAttendance(ctx, input)
	require.NoError(t, err)
	require.True(t, inserted)
	require.NotZero(t, first.ID)
	_, inserted, err = module.EnsureAttendance(ctx, input)
	require.NoError(t, err)
	assert.False(t, inserted)
	close := studentpresence.AttendanceCheckout{StudentIDs: []int64{student.ID}, Date: input.Date, At: at.Add(time.Hour), DeviceID: device.ID}
	abort := errors.New("fail after attendance checkout")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		closed, err := module.CloseAttendance(txCtx, close)
		if err != nil {
			return err
		}
		require.Len(t, closed, 1)
		return abort
	})
	require.ErrorIs(t, err, abort)
	unchanged, err := module.FindAttendance(ctx, first.ID)
	require.NoError(t, err)
	assert.Nil(t, unchanged.CheckOutTime)
	closed, err := module.CloseAttendance(ctx, close)
	require.NoError(t, err)
	require.Len(t, closed, 1)
	assert.Equal(t, device.ID, *closed[0].CheckedOutDeviceID)
	closed, err = module.CloseAttendance(ctx, close)
	require.NoError(t, err)
	assert.Empty(t, closed)
	input.CheckInTime = at.Add(2 * time.Hour)
	second, inserted, err := module.EnsureAttendance(ctx, input)
	require.NoError(t, err)
	require.True(t, inserted)
	assert.NotEqual(t, first.ID, second.ID)
	stored, err := module.FindAttendance(ctx, first.ID)
	require.NoError(t, err)
	assert.Equal(t, close.At.Unix(), stored.CheckOutTime.Unix())
	rows, err := module.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{student.ID}, FromDate: input.Date, UntilDate: input.Date})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, first.ID, rows[0].ID)
	assert.Equal(t, second.ID, rows[1].ID)
}
