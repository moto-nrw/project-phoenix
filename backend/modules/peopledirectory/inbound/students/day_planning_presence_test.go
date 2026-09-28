package students_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/inbound/students"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestListStudents_UnplannedPresenceEndsAtCheckout(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	fixedNow := time.Date(2026, time.August, 21, 10, 0, 0, 0, time.UTC)
	tc.resource.Now = func() time.Time { return fixedNow }

	schoolClass := "Presence-2379"
	absent := testpkg.CreateTestStudent(t, tc.db, "Absent", "Student", schoolClass)
	present := testpkg.CreateTestStudent(t, tc.db, "Present", "Student", schoolClass)
	checkedOut := testpkg.CreateTestStudent(t, tc.db, "CheckedOut", "Student", schoolClass)
	plannedPresent := testpkg.CreateTestStudent(t, tc.db, "PlannedPresent", "Student", schoolClass)
	sickPresent := testpkg.CreateTestStudent(t, tc.db, "SickPresent", "Student", schoolClass)
	staff := testpkg.CreateTestStaff(t, tc.db, "Presence", "Supervisor")
	device := testpkg.CreateTestDevice(t, tc.db, "presence-2379-device")
	today := timezone.DateFromTime(fixedNow)

	for _, student := range []int64{absent.ID, present.ID, checkedOut.ID} {
		testpkg.CreateTestArrivalException(t, tc.db, student, today, staff.ID, "", "Kommt heute nicht")
	}
	testpkg.CreateTestStudentStatusDay(t, tc.db, sickPresent.ID, today, absencerecords.StudentStatusDaySick)

	checkIn := fixedNow.Add(-time.Hour)
	testpkg.CreateTestAttendance(t, tc.db, present.ID, staff.ID, device.ID, checkIn, nil)
	testpkg.CreateTestPickupSchedule(t, tc.db, plannedPresent.ID, 5, staff.ID, "15:30") // Friday
	testpkg.CreateTestAttendance(t, tc.db, plannedPresent.ID, staff.ID, device.ID, checkIn, nil)
	testpkg.CreateTestAttendance(t, tc.db, sickPresent.ID, staff.ID, device.ID, checkIn, nil)
	checkOut := fixedNow.Add(-30 * time.Minute)
	testpkg.CreateTestAttendance(t, tc.db, checkedOut.ID, staff.ID, device.ID, checkIn, &checkOut)

	req := testutil.NewRequest("GET", fmt.Sprintf("/?school_class=%s&page_size=50", schoolClass), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

	byID := decodeStudentsByID(t, rr.Body.Bytes())
	assert.Equal(t, students.DayPlanningStatusNotComingToday, byID[absent.ID].DayPlanningStatus)
	assert.Equal(t, "arrival_exception", byID[absent.ID].DayPlanningReason)
	assert.Equal(t, students.DayPlanningStatusComesToday, byID[present.ID].DayPlanningStatus)
	assert.Equal(t, "unplanned_attendance", byID[present.ID].DayPlanningReason)
	assert.Equal(t, students.DayPlanningStatusNotComingToday, byID[checkedOut.ID].DayPlanningStatus)
	assert.Equal(t, "arrival_exception", byID[checkedOut.ID].DayPlanningReason)
	assert.Equal(t, students.DayPlanningStatusComesToday, byID[plannedPresent.ID].DayPlanningStatus)
	assert.Equal(t, "pickup_schedule", byID[plannedPresent.ID].DayPlanningReason)
	assert.Equal(t, students.DayPlanningStatusComesToday, byID[sickPresent.ID].DayPlanningStatus)
	assert.Equal(t, "unplanned_attendance", byID[sickPresent.ID].DayPlanningReason)
}

// TestListStudents_ExpectedChildReadsSchoolBeforeFirstCheckIn pins #3260: an
// expected child without a check-in reads "Schule" until the pickup time; a
// checkout, a passed pickup time, no plan, a sick report and "Kommt heute
// nicht" keep "Abwesend" (the frontend's "Zuhause").
func TestListStudents_ExpectedChildReadsSchoolBeforeFirstCheckIn(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	// Friday 12:00 Berlin.
	fixedNow := time.Date(2026, time.August, 21, 10, 0, 0, 0, time.UTC)
	tc.resource.Now = func() time.Time { return fixedNow }

	schoolClass := "School-3260"
	waiting := testpkg.CreateTestStudent(t, tc.db, "Waiting", "Student", schoolClass)
	checkedOut := testpkg.CreateTestStudent(t, tc.db, "CheckedOut", "Student", schoolClass)
	pickupPassed := testpkg.CreateTestStudent(t, tc.db, "PickupPassed", "Student", schoolClass)
	noPlan := testpkg.CreateTestStudent(t, tc.db, "NoPlan", "Student", schoolClass)
	sick := testpkg.CreateTestStudent(t, tc.db, "Sick", "Student", schoolClass)
	notComing := testpkg.CreateTestStudent(t, tc.db, "NotComing", "Student", schoolClass)
	present := testpkg.CreateTestStudent(t, tc.db, "Present", "Student", schoolClass)
	staff := testpkg.CreateTestStaff(t, tc.db, "School", "Supervisor")
	device := testpkg.CreateTestDevice(t, tc.db, "school-3260-device")
	today := timezone.DateFromTime(fixedNow)

	for _, student := range []int64{waiting.ID, checkedOut.ID, sick.ID, notComing.ID, present.ID} {
		testpkg.CreateTestPickupSchedule(t, tc.db, student, 5, staff.ID, "15:30") // Friday
	}
	testpkg.CreateTestPickupSchedule(t, tc.db, pickupPassed.ID, 5, staff.ID, "11:45") // Friday
	testpkg.CreateTestStudentStatusDay(t, tc.db, sick.ID, today, absencerecords.StudentStatusDaySick)
	testpkg.CreateTestArrivalException(t, tc.db, notComing.ID, today, staff.ID, "", "Kommt heute nicht")

	checkIn := fixedNow.Add(-2 * time.Hour)
	checkOut := fixedNow.Add(-time.Hour)
	testpkg.CreateTestAttendance(t, tc.db, checkedOut.ID, staff.ID, device.ID, checkIn, &checkOut)
	testpkg.CreateTestAttendance(t, tc.db, present.ID, staff.ID, device.ID, checkIn, nil)

	req := testutil.NewRequest("GET", fmt.Sprintf("/?school_class=%s&page_size=50", schoolClass), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

	byID := decodeStudentsByID(t, rr.Body.Bytes())
	assert.Equal(t, "Schule", byID[waiting.ID].Location)
	assert.Equal(t, "Abwesend", byID[checkedOut.ID].Location, "a checkout sends the child home")
	assert.Equal(t, "Abwesend", byID[pickupPassed.ID].Location, "the care day ended at the pickup time")
	assert.Equal(t, "Abwesend", byID[noPlan.ID].Location, "a child nobody expects today stays at home")
	assert.Equal(t, "Abwesend", byID[sick.ID].Location, "sick wins over school")
	assert.Equal(t, "Abwesend", byID[notComing.ID].Location, "Kommt heute nicht wins over school")
	assert.NotEqual(t, "Schule", byID[present.ID].Location)
	assert.NotEqual(t, "Abwesend", byID[present.ID].Location)

	// The location filter must see the location resolved from the day plan,
	// rather than the initial absent location used while building the response.
	filteredReq := testutil.NewRequest("GET", fmt.Sprintf("/?school_class=%s&location=Schule&page_size=50", schoolClass), nil)
	filteredRR := authExec(t, tc, filteredReq, testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusOK, filteredRR.Code, "body: %s", filteredRR.Body.String())
	filtered := decodeStudentsByID(t, filteredRR.Body.Bytes())
	require.Len(t, filtered, 1)
	assert.Equal(t, "Schule", filtered[waiting.ID].Location)

	// The dashboard counts through the same rule.
	count, err := tc.resource.CountAtSchoolToday(testpkg.Ctx(t), []int64{
		waiting.ID, checkedOut.ID, pickupPassed.ID, noPlan.ID, notComing.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}
