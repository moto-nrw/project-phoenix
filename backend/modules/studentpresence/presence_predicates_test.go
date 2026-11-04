package studentpresence_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

func TestAttendanceCompletionReportsOpenAndCompletedDays(t *testing.T) {
	t.Parallel()
	out := time.Now()
	open, completed := studentpresence.AttendanceCompletion(nil)
	assert.False(t, open)
	assert.False(t, completed)
	open, completed = studentpresence.AttendanceCompletion([]studentpresence.Attendance{{}, {CheckOutTime: &out}})
	assert.True(t, open)
	assert.True(t, completed)
	open, completed = studentpresence.AttendanceCompletion([]studentpresence.Attendance{{CheckOutTime: &out}})
	assert.False(t, open)
	assert.True(t, completed)
}

func TestAttendanceCompletion_ClassIsNoDeparture(t *testing.T) {
	t.Parallel()
	morning := time.Date(2026, 9, 29, 8, 30, 0, 0, time.UTC)
	noon := morning.Add(4 * time.Hour)
	reason := studentpresence.CheckoutReasonClass

	_, completed := studentpresence.AttendanceCompletion([]studentpresence.Attendance{{CheckOutTime: &morning, CheckOutReason: &reason}})
	assert.False(t, completed, "a child in class has not left for the day")

	early := morning.Add(-time.Hour)
	back := morning.Add(time.Hour)
	_, completed = studentpresence.AttendanceCompletion([]studentpresence.Attendance{
		{CheckInTime: back, CheckOutTime: &noon},
		{CheckInTime: early, CheckOutTime: &morning, CheckOutReason: &reason},
	})
	assert.True(t, completed, "going home after the class stay is a departure, whatever the row order")

	_, completed = studentpresence.AttendanceCompletion([]studentpresence.Attendance{
		{CheckInTime: early, CheckOutTime: &morning},
		{CheckInTime: back, CheckOutTime: &noon, CheckOutReason: &reason},
	})
	assert.False(t, completed, "the last stay decides, as for the parent gate")
}

// A corrected exit can leave stay A ending after the later stay B: the visit
// amendment accepts it. The stay with the latest check-in decides, as in
// ListSchoolStatuses, so staff, parents and the pickup gate read one fact.
func TestAttendanceCompletion_OverlapFollowsTheLatestCheckIn(t *testing.T) {
	t.Parallel()
	day := func(hour int) time.Time { return time.Date(2026, 9, 29, hour, 0, 0, 0, time.UTC) }
	aIn, aOut, bIn, bOut := day(8), day(12), day(10), day(11)
	reason := studentpresence.CheckoutReasonClass

	_, completed := studentpresence.AttendanceCompletion([]studentpresence.Attendance{
		{CheckInTime: aIn, CheckOutTime: &aOut, CheckOutReason: &reason},
		{CheckInTime: bIn, CheckOutTime: &bOut},
	})
	assert.True(t, completed, "B went home, whatever A was corrected to")

	_, completed = studentpresence.AttendanceCompletion([]studentpresence.Attendance{
		{CheckInTime: aIn, CheckOutTime: &aOut},
		{CheckInTime: bIn, CheckOutTime: &bOut, CheckOutReason: &reason},
	})
	assert.False(t, completed, "B went to class, whatever A was corrected to")
}

func TestArrivalKeepsTheArrivalAcrossAReturnFromClass(t *testing.T) {
	t.Parallel()
	arrived := time.Date(2026, 9, 29, 7, 30, 0, 0, time.UTC)
	back := arrived.Add(5 * time.Hour)

	status := &studentpresence.DailyAttendanceStatus{CheckInTime: &back, ArrivalTime: &arrived}
	assert.Equal(t, &arrived, status.Arrival())
	assert.Equal(t, &back, (&studentpresence.DailyAttendanceStatus{CheckInTime: &back}).Arrival())
	assert.Nil(t, (*studentpresence.DailyAttendanceStatus)(nil).Arrival())

	other := &studentpresence.AttendanceStatus{CheckInTime: &back, ArrivalTime: &arrived}
	assert.Equal(t, &arrived, other.Arrival())
	assert.Equal(t, &back, (&studentpresence.AttendanceStatus{CheckInTime: &back}).Arrival())
}

func TestGroupSupervisionEndsOnItsEndDate(t *testing.T) {
	t.Parallel()
	today := calendar.Date("2026-10-19")
	end := func(value string) *string { return &value }
	assert.False(t, studentpresence.GroupSupervision{}.EndedOn(today), "an open supervision runs")
	assert.False(t, studentpresence.GroupSupervision{EndDate: end("2026-10-20")}.EndedOn(today), "a planned future end still supervises")
	assert.True(t, studentpresence.GroupSupervision{EndDate: end("2026-10-19")}.EndedOn(today))
	assert.True(t, studentpresence.GroupSupervision{EndDate: end("2026-10-01")}.EndedOn(today))
	assert.False(t, studentpresence.GroupSupervision{EndDate: end("19.10.2026")}.EndedOn(today), "an unreadable end has not ended")
}

func TestLiveGroupStartedOnItsStartDay(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 10, 19, 23, 30, 0, 0, calendar.Berlin)
	assert.True(t, studentpresence.LiveGroup{StartTime: start}.StartedOn("2026-10-19"))
	assert.False(t, studentpresence.LiveGroup{StartTime: start}.StartedOn("2026-10-20"))
}

func TestOnlyChildrenOfAnotherSchoolAreVisitors(t *testing.T) {
	t.Parallel()
	assert.True(t, studentpresence.VisitsFromAnotherSchool(21, 31))
	assert.False(t, studentpresence.VisitsFromAnotherSchool(31, 31))
}

func TestSupervisionStaffLookupIncludesRemovedStaff(t *testing.T) {
	t.Parallel()
	assert.True(t, studentpresence.SupervisionStaffIncludesRemoved, "supervision history keeps naming removed staff")
}

func TestBinarySnapshotReadsNoRooms(t *testing.T) {
	t.Parallel()
	assert.False(t, studentpresence.NewStudentLocationSnapshot(studentpresence.PresenceModeBinary).ReadsRooms())
	assert.True(t, studentpresence.NewStudentLocationSnapshot(studentpresence.PresenceModeDetailed).ReadsRooms())
	assert.True(t, studentpresence.NewStudentLocationSnapshot("").ReadsRooms(), "an empty mode is the detailed default")
}

func TestKeepCurrentVisitsKeepsEachStudentsFirstListedVisit(t *testing.T) {
	t.Parallel()
	student, other := int64(41), int64(43)
	newest, older, otherVisit := int64(71), int64(73), int64(79)
	filter := studentpresence.CurrentVisitsFilter([]int64{student, other})
	assert.Equal(t, studentpresence.VisitFilter{StudentIDs: []int64{student, other}, OpenOnly: true, StudentOrder: true, NewestFirst: true}, filter)

	snapshot := studentpresence.NewStudentLocationSnapshot(studentpresence.PresenceModeDetailed)
	snapshot.KeepCurrentVisits([]studentpresence.Visit{
		{ID: newest, StudentID: student}, {ID: older, StudentID: student}, {ID: otherVisit, StudentID: other},
	})
	assert.Equal(t, newest, snapshot.Visits[student].ID)
	assert.Equal(t, otherVisit, snapshot.Visits[other].ID)

	snapshot.KeepCurrentVisits([]studentpresence.Visit{{ID: older, StudentID: student}})
	assert.Equal(t, newest, snapshot.Visits[student].ID, "a recorded visit stays")
}
