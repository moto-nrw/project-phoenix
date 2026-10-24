package ports

import (
	"time"
)

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
