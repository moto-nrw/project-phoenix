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

// The attendance retention (#3957) runs only inside the school's cleanup
// transaction, so a deletion never commits without its deletion-log entry.
func TestAttendanceRetentionRequiresTheCleanupTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	student := testpkg.CreateTestStudent(t, db, "Frist", "Anwesenheit", "2a")
	device := testpkg.CreateTestDevice(t, db, "attendance-retention-tx")
	day := testpkg.TodayDate().AddDays(-400)
	row, err := module.RecordAttendance(ctx, studentpresence.Attendance{StudentID: student.ID, DeviceID: device.ID, Date: day.String(), CheckInTime: day.BerlinMidnight().Add(8 * time.Hour)})
	require.NoError(t, err)

	_, err = module.DeleteExpiredAttendance(ctx, 365)
	require.ErrorContains(t, err, "transaction is required")
	_, err = module.FindAttendance(ctx, row.ID)
	require.NoError(t, err, "a refused run deletes nothing")

	_, err = module.DeleteExpiredAttendance(ctx, 0)
	require.ErrorIs(t, err, studentpresence.ErrAttendanceRetentionWindow)

	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		deleted, err := module.DeleteExpiredAttendance(txCtx, 365)
		assert.Equal(t, 1, deleted)
		return err
	}))
	found, err := module.ListAttendance(ctx, studentpresence.AttendanceFilter{IDs: []int64{row.ID}})
	require.NoError(t, err)
	assert.Empty(t, found)
}
