package studentpresence

import (
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// HistoryInstanceCancelled is the status of a history slot whose block
// occurrence was cancelled; its bookings survive the cancellation.
const HistoryInstanceCancelled = "cancelled"

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

// VisitHistoryEntry is one room stay of a student's visit history.
type VisitHistoryEntry struct {
	EntryTime time.Time
	ExitTime  *time.Time
	RoomID    *int64
	RoomName  string
}

// DataAccessEvent is the GDPR evidence of one disclosure of student
// presence data.
type DataAccessEvent struct {
	ActorAccountID int64
	ActorRole      string
	ResourceType   string
	StudentID      *int64
	RangeStart     time.Time
	RangeEnd       time.Time
	AccessedAt     time.Time
	// Scope narrows a group-scoped read; nil for a single student's history.
	Scope *DataAccessScope
}

// DataAccessScope names the groups (and, for a one-day read, the date) a
// group-scoped disclosure covered.
type DataAccessScope struct {
	GroupIDs []int64
	// Date is empty when the read is not bound to one school day.
	Date string
}
