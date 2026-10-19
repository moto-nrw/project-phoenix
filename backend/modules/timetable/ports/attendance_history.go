package ports

import (
	"context"
	"time"

	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type AttendanceHistoryRecords interface {
	FindInstancesWithAttendanceByStudentAndDateRange(context.Context, int64, calendar.Date, calendar.Date) ([]*scheduleModels.ScheduledInstanceRow, error)
	HasPlannedSlotsInRange(context.Context, calendar.Date, calendar.Date) (bool, error)
}

// AttendanceHistorySlot contains only the timetable facts used in student history.
type AttendanceHistorySlot struct {
	Instance   *AttendanceHistoryInstance
	Attendance *AttendanceHistoryPresence
}

type AttendanceHistoryInstance struct {
	ID        int64
	Date      string
	Title     string
	Status    string
	StartTime time.Time
	EndTime   time.Time
}

type AttendanceHistoryPresence struct {
	Status       string
	Substatus    *string
	Note         *string
	CheckedInAt  *time.Time
	CheckedOutAt *time.Time
	IsUnplanned  bool
}
