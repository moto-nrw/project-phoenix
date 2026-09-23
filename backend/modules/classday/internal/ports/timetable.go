package ports

import (
	"context"
	"time"
)

// Block statuses as the lists read them: the plan of Timetable & Activities
// folded with the execution Student Presence owns since #2762. A block that
// nobody started is planned; a block with a session is active or completed;
// a cancelled plan stays cancelled.
const (
	InstanceStatusPlanned   = "planned"
	InstanceStatusActive    = "active"
	InstanceStatusCompleted = "completed"
	InstanceStatusCancelled = "cancelled"
)

// Attendance statuses of a roster row.
const (
	AttendanceExpected = "expected"
	AttendancePresent  = "present"
	AttendanceAbsent   = "absent"
)

// ActivityInstance is one block of the day as the lists read it.
type ActivityInstance struct {
	ID            int64
	Title         string
	Date          string
	StartTime     string
	EndTime       string
	RoomID        int64
	Status        string
	ActiveGroupID *int64
	ListKind      *string
}

// InstanceStudent is one roster row of a block: the planned participant with
// the attendance it carries. A participant without a stored attendance is
// expected.
type InstanceStudent struct {
	ID                 int64
	InstanceID         int64
	StudentID          int64
	RoomID             *int64
	Status             string
	Substatus          *string
	Note               *string
	CheckedInAt        *time.Time
	CheckedOutAt       *time.Time
	IsUnplanned        bool
	NotScheduled       bool
	ManualStatusAt     *time.Time
	StudentStatusDayID *int64
	PickupExceptionID  *int64
}

// TimetableReader is the read seam of the blocks and rosters the lists
// build on. The binding joins the Timetable plan with the Student Presence
// execution and attendance.
type TimetableReader interface {
	// ListActivityInstancesOn returns the blocks of the ISO day ordered by start
	// time and id.
	ListActivityInstancesOn(ctx context.Context, date string) ([]ActivityInstance, error)
	// ListRoster returns the roster rows of the blocks ordered by block and
	// participant.
	ListRoster(ctx context.Context, instanceIDs []int64) ([]InstanceStudent, error)
}
