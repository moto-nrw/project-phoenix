package active

import (
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

type HistorySlot struct {
	Instance   *HistorySlotInstance
	Attendance *HistorySlotAttendance
}

type HistorySlotInstance struct {
	ID        int64
	Date      timezone.Date
	Title     string
	Status    string
	StartTime time.Time
	EndTime   time.Time
}

type HistorySlotAttendance struct {
	Status       string
	Substatus    *string
	Note         *string
	CheckedInAt  *time.Time
	CheckedOutAt *time.Time
	IsUnplanned  bool
}
