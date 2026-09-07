package compose_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttendanceFacadeRejectsForeignReferencesAndCannotModifyForeignRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	ownDevice := testpkg.CreateTestDevice(t, db, "attendance-own-device")
	var foreign studentpresence.Attendance
	var foreignCtx context.Context
	at := time.Now()
	t.Run("foreign fixture", func(t *testing.T) {
		testpkg.OwnTenant(t)
		foreignCtx = testpkg.Ctx(t)
		student := testpkg.CreateTestStudent(t, db, "Foreign", "Attendance", "3a")
		device := testpkg.CreateTestDevice(t, db, "attendance-foreign-device")
		foreign, err = module.RecordAttendance(foreignCtx, studentpresence.Attendance{
			StudentID: student.ID, Date: testpkg.TodayDate().String(), CheckInTime: at, DeviceID: device.ID,
		})
		require.NoError(t, err)
	})
	attempted := foreign
	attempted.ID = 0
	attempted.TenantID = 0
	attempted.DeviceID = ownDevice.ID
	_, err = module.RecordAttendance(ctx, attempted)
	require.Error(t, err, "composite student FK must reject a foreign student even when the submitted tenant is unset")
	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, err := module.FindAttendance(txCtx, foreign.ID)
		require.ErrorIs(t, err, studentpresence.ErrAttendanceNotFound)
		rows, err := module.ListAttendance(txCtx, studentpresence.AttendanceFilter{IDs: []int64{foreign.ID}, ForUpdate: true})
		require.NoError(t, err)
		assert.Empty(t, rows)
		exists, err := module.HasAttendance(txCtx, studentpresence.AttendanceFilter{IDs: []int64{foreign.ID}})
		require.NoError(t, err)
		assert.False(t, exists)
		rows, err = module.CloseAttendance(txCtx, studentpresence.AttendanceCheckout{StudentIDs: []int64{foreign.StudentID}, Date: foreign.Date, At: at.Add(time.Hour)})
		require.NoError(t, err)
		assert.Empty(t, rows)
		require.NoError(t, module.DeleteAttendance(txCtx, foreign.ID))
		return nil
	}))
	attempted = foreign
	attempted.TenantID = testpkg.Tenant(t)
	attempted.CheckOutTime = &at
	_, err = module.ReviseAttendance(ctx, attempted)
	require.Error(t, err, "a caller cannot transfer a foreign row into its own tenant")
	stored, err := module.FindAttendance(foreignCtx, foreign.ID)
	require.NoError(t, err)
	assert.Nil(t, stored.CheckOutTime)
	assert.Equal(t, foreign.TenantID, stored.TenantID)
}

func TestAttendanceFacadeConcurrentEnsureHasOneWinner(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	student := testpkg.CreateTestStudent(t, db, "Concurrent", "Attendance", "3a")
	device := testpkg.CreateTestDevice(t, db, "concurrent-attendance")
	input := studentpresence.Attendance{StudentID: student.ID, Date: testpkg.TodayDate().String(), CheckInTime: time.Now(), DeviceID: device.ID}
	type outcome struct {
		inserted bool
		err      error
	}
	ready := make(chan struct{})
	results := make(chan outcome, 2)
	for range 2 {
		go func() {
			<-ready
			_, inserted, err := module.EnsureAttendance(ctx, input)
			results <- outcome{inserted, err}
		}()
	}
	close(ready)
	winners := 0
	for range 2 {
		result := <-results
		require.NoError(t, result.err)
		if result.inserted {
			winners++
		}
	}
	assert.Equal(t, 1, winners)
	rows, err := module.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{student.ID}})
	require.NoError(t, err)
	assert.Len(t, rows, 1)
}
