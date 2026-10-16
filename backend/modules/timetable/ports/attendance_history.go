package ports

import (
	"context"
	"time"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
)

type AttendanceHistoryRecords interface {
	FindInstancesWithAttendanceByStudentAndDateRange(context.Context, int64, scheduleModels.Date, scheduleModels.Date) ([]*scheduleModels.ScheduledInstanceRow, error)
	HasPlannedSlotsInRange(context.Context, scheduleModels.Date, scheduleModels.Date) (bool, error)
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
