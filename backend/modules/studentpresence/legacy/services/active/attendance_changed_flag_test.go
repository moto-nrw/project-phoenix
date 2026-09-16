package active_test

import (
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AttendanceResult.Changed pins whether the CALL mutated the attendance row
// (review #2372): an absorbed in/in race or an already-closed row must report
// Changed=false so the web batch endpoint's no-op display stays truthful even
// when a concurrent request established the target state first.

func TestCheckInStudent_ChangedFlag(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "Changed", "CheckIn", "5d")
	staff := testpkg.CreateTestStaff(t, db, "Changed", "Staff")
	device := testpkg.CreateTestDevice(t, db, "changed-device-001")

	fresh, err := service.CheckInStudent(ctx, student.ID, staff.ID, device.ID, true)
	require.NoError(t, err)
	assert.True(t, fresh.Changed, "fresh check-in opened the row and must report Changed")

	repeat, err := service.CheckInStudent(ctx, student.ID, staff.ID, device.ID, true)
	require.NoError(t, err)
	assert.False(t, repeat.Changed, "absorbed second in must report Changed=false")
	assert.Equal(t, fresh.AttendanceID, repeat.AttendanceID, "absorbed call points at the existing open row")
}

func TestCheckOutStudent_ChangedFlag(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "Changed", "CheckOut", "5e")
	staff := testpkg.CreateTestStaff(t, db, "Changed", "Staff2")
	device := testpkg.CreateTestDevice(t, db, "changed-device-002")

	checkInTime := time.Now().Add(-1 * time.Hour)
	testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, checkInTime, nil)

	closed, err := service.CheckOutStudent(ctx, student.ID, staff.ID, true)
	require.NoError(t, err)
	assert.True(t, closed.Changed, "closing the open row must report Changed")

	repeat, err := service.CheckOutStudent(ctx, student.ID, staff.ID, true)
	require.NoError(t, err)
	assert.False(t, repeat.Changed, "idempotent second out must report Changed=false")
}
