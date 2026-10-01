package presence_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Early-checkout note (#3324): the web checkout may carry an optional reason.
// It lands on the stay the call closes, is trimmed, and never overwrites the
// note of a checkout that already happened.

func TestCheckOutStudentWithNote_StoresTrimmedNoteOnClosedStay(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "Note", "Early", "3a")
	staff := testpkg.CreateTestStaff(t, db, "Note", "Staff")
	device := testpkg.CreateTestDevice(t, db, "checkout-note-device-001")
	testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, time.Now().Add(-1*time.Hour), nil)

	result, err := service.CheckOutStudentWithNote(ctx, student.ID, staff.ID, "  Arzttermin  ", true)
	require.NoError(t, err)
	require.True(t, result.Changed)

	status, err := service.GetStudentAttendanceStatus(ctx, student.ID)
	require.NoError(t, err)
	assert.Equal(t, "checked_out", status.Status)
	require.NotNil(t, status.CheckOutNote)
	assert.Equal(t, "Arzttermin", *status.CheckOutNote)

	// An idempotent retry with another note must not rewrite the closed stay.
	repeat, err := service.CheckOutStudentWithNote(ctx, student.ID, staff.ID, "Anderer Grund", true)
	require.NoError(t, err)
	assert.False(t, repeat.Changed)

	status, err = service.GetStudentAttendanceStatus(ctx, student.ID)
	require.NoError(t, err)
	require.NotNil(t, status.CheckOutNote)
	assert.Equal(t, "Arzttermin", *status.CheckOutNote)
}

func TestCheckOutStudentWithNote_BlankNoteStoresNothing(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "Note", "Blank", "3b")
	staff := testpkg.CreateTestStaff(t, db, "Note", "Blank")
	device := testpkg.CreateTestDevice(t, db, "checkout-note-device-002")
	testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, time.Now().Add(-1*time.Hour), nil)

	_, err := service.CheckOutStudentWithNote(ctx, student.ID, staff.ID, "   ", true)
	require.NoError(t, err)

	status, err := service.GetStudentAttendanceStatus(ctx, student.ID)
	require.NoError(t, err)
	assert.Equal(t, "checked_out", status.Status)
	assert.Nil(t, status.CheckOutNote)
}

func TestCheckOutStudentWithNote_RejectsTooLongNoteWithoutCheckingOut(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "Note", "Long", "3c")
	staff := testpkg.CreateTestStaff(t, db, "Note", "Long")
	device := testpkg.CreateTestDevice(t, db, "checkout-note-device-003")
	testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, time.Now().Add(-1*time.Hour), nil)

	// Multi-byte runes: the limit counts characters, not bytes.
	exact := strings.Repeat("ä", studentpresence.MaxCheckoutNoteLength)
	_, err := service.CheckOutStudentWithNote(ctx, student.ID, staff.ID, exact+"ä", true)
	require.Error(t, err)
	assert.True(t, errors.Is(err, studentpresence.ErrCheckoutNoteTooLong))

	status, err := service.GetStudentAttendanceStatus(ctx, student.ID)
	require.NoError(t, err)
	assert.Equal(t, "checked_in", status.Status, "a rejected note must leave the child checked in")

	_, err = service.CheckOutStudentWithNote(ctx, student.ID, staff.ID, exact, true)
	require.NoError(t, err)
	status, err = service.GetStudentAttendanceStatus(ctx, student.ID)
	require.NoError(t, err)
	require.NotNil(t, status.CheckOutNote)
	assert.Equal(t, exact, *status.CheckOutNote)
}
