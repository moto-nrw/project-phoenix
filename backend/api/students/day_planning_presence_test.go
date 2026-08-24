package students_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/students"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestListStudents_UnplannedPresenceEndsAtCheckout(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
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
	testpkg.CreateTestStudentStatusDay(t, tc.db, sickPresent.ID, today, activeModel.StudentStatusDaySick)

	checkIn := fixedNow.Add(-time.Hour)
	testpkg.CreateTestAttendance(t, tc.db, present.ID, staff.ID, device.ID, checkIn, nil)
	testpkg.CreateTestPickupSchedule(t, tc.db, plannedPresent.ID, scheduleModel.WeekdayFriday, staff.ID, "15:30")
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
