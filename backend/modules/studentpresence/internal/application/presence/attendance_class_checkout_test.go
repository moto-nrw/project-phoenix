package presence_test

import (
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// "Geht zum Unterricht" (#5656): the checkout closes the stay with the class
// reason, the child can come back the same day, and going home stays unlabelled.

func TestCheckOutStudentToClass_LabelsTheStayAndAllowsReturn(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "Klasse", "Rueckkehr", "2a")
	staff := testpkg.CreateTestStaff(t, db, "Klasse", "Staff")
	device := testpkg.CreateTestDevice(t, db, "class-checkout-device-001")
	testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, time.Now().Add(-1*time.Hour), nil)

	result, err := service.CheckOutStudentWithNote(ctx, student.ID, staff.ID, "", studentpresence.CheckoutReasonClass, true)
	require.NoError(t, err)
	require.True(t, result.Changed)

	status, err := service.GetStudentAttendanceStatus(ctx, student.ID)
	require.NoError(t, err)
	assert.Equal(t, "checked_out", status.Status)
	assert.True(t, status.IsInClass())

	// A retry reports no change and cannot relabel the closed stay.
	repeat, err := service.CheckOutStudentWithNote(ctx, student.ID, staff.ID, "", studentpresence.CheckoutReasonClass, true)
	require.NoError(t, err)
	assert.False(t, repeat.Changed)

	// The return opens a new stay; the status no longer reads as in class.
	back, err := service.CheckInStudent(ctx, student.ID, staff.ID, device.ID, true)
	require.NoError(t, err)
	require.True(t, back.Changed)
	status, err = service.GetStudentAttendanceStatus(ctx, student.ID)
	require.NoError(t, err)
	assert.Equal(t, "checked_in", status.Status)
	assert.False(t, status.IsInClass())

	// Going home later is an ordinary checkout again.
	_, err = service.CheckOutStudent(ctx, student.ID, staff.ID, true)
	require.NoError(t, err)
	status, err = service.GetStudentAttendanceStatus(ctx, student.ID)
	require.NoError(t, err)
	assert.Equal(t, "checked_out", status.Status)
	assert.False(t, status.IsInClass())
	assert.Nil(t, status.CheckOutReason)
}

func TestCheckOutStudentToClass_DoesNotRelabelAHomeCheckout(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "Klasse", "Heim", "2b")
	staff := testpkg.CreateTestStaff(t, db, "Klasse", "Heim")
	device := testpkg.CreateTestDevice(t, db, "class-checkout-device-002")
	testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, time.Now().Add(-1*time.Hour), nil)

	_, err := service.CheckOutStudent(ctx, student.ID, staff.ID, true)
	require.NoError(t, err)
	repeat, err := service.CheckOutStudentWithNote(ctx, student.ID, staff.ID, "", studentpresence.CheckoutReasonClass, true)
	require.NoError(t, err)
	assert.False(t, repeat.Changed)

	status, err := service.GetStudentAttendanceStatus(ctx, student.ID)
	require.NoError(t, err)
	assert.False(t, status.IsInClass())
}

func TestProcessSchoolClassCheckoutBatch_LabelsEveryClosedStay(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)

	first := testpkg.CreateTestStudent(t, db, "Klasse", "Batch Eins", "2c")
	second := testpkg.CreateTestStudent(t, db, "Klasse", "Batch Zwei", "2c")
	wentHome := testpkg.CreateTestStudent(t, db, "Klasse", "Batch Heim", "2c")
	staff := testpkg.CreateTestStaff(t, db, "Klasse", "Batch")
	device := testpkg.CreateTestDevice(t, db, "class-checkout-device-003")
	for _, id := range []int64{first.ID, second.ID, wentHome.ID} {
		testpkg.CreateTestAttendance(t, db, id, staff.ID, device.ID, time.Now().Add(-1*time.Hour), nil)
	}
	_, err := service.CheckOutStudent(ctx, wentHome.ID, staff.ID, true)
	require.NoError(t, err)

	result, err := service.ProcessSchoolCheckinBatch(ctx, []int64{first.ID, second.ID, wentHome.ID}, staff.ID, "out", studentpresence.CheckoutReasonClass)
	require.NoError(t, err)
	require.Len(t, result.Results, 3)
	assert.True(t, result.Results[0].InClass)
	assert.True(t, result.Results[1].InClass)
	assert.False(t, result.Results[2].InClass, "an earlier home checkout stays what it was")
	assert.False(t, result.Results[2].Changed)
}

func TestConfirmDailyCheckout_DestinationClass(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "Klasse", "Kiosk", "2d")
	staff := testpkg.CreateTestStaff(t, db, "Klasse", "Kiosk")
	device := testpkg.CreateTestDevice(t, db, "class-checkout-device-004")
	testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, time.Now().Add(-1*time.Hour), nil)

	result, err := service.ConfirmDailyCheckout(ctx, student.ID, device.ID, "unterricht")
	require.NoError(t, err)
	assert.Equal(t, "checked_out_class", result.Action)

	status, err := service.GetStudentAttendanceStatus(ctx, student.ID)
	require.NoError(t, err)
	assert.Equal(t, "checked_out", status.Status)
	require.NotNil(t, status.CheckOutReason)
	assert.Equal(t, studentpresence.CheckoutReasonClass, *status.CheckOutReason)
}

// A confirmation on a closed stay changes nothing, so its answer names the
// checkout on record and not the destination asked for.
func TestConfirmDailyCheckout_RepeatAnswersWhatIsRecorded(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "Klasse", "Wiederholung")
	device := testpkg.CreateTestDevice(t, db, "class-checkout-device-005")

	home := testpkg.CreateTestStudent(t, db, "Klasse", "Daheim", "2e")
	testpkg.CreateTestAttendance(t, db, home.ID, staff.ID, device.ID, time.Now().Add(-1*time.Hour), nil)
	_, err := service.ConfirmDailyCheckout(ctx, home.ID, device.ID, "zuhause")
	require.NoError(t, err)
	result, err := service.ConfirmDailyCheckout(ctx, home.ID, device.ID, "unterricht")
	require.NoError(t, err)
	assert.Equal(t, "checked_out_daily", result.Action)

	inClass := testpkg.CreateTestStudent(t, db, "Klasse", "Unterricht", "2e")
	testpkg.CreateTestAttendance(t, db, inClass.ID, staff.ID, device.ID, time.Now().Add(-1*time.Hour), nil)
	_, err = service.ConfirmDailyCheckout(ctx, inClass.ID, device.ID, "unterricht")
	require.NoError(t, err)
	result, err = service.ConfirmDailyCheckout(ctx, inClass.ID, device.ID, "zuhause")
	require.NoError(t, err)
	assert.Equal(t, "checked_out_class", result.Action)
	result, err = service.ConfirmDailyCheckout(ctx, inClass.ID, device.ID, "unterwegs")
	require.NoError(t, err)
	assert.Equal(t, "checked_out", result.Action, "unterwegs keeps its answer")
}
