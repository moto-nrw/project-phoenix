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
