package ports

import (
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type Room struct {
	ID   int64
	Name string
}

type Student struct {
	ID          int64
	PersonID    int64
	SchoolClass string
	GroupID     *int64
}

type Person struct {
	ID   int64
	Name string
}

type GroupSupervisor struct {
	StaffID int64
	GroupID int64
}

type Group struct {
	RoomID int64
}

type StaffRoomSupervision struct {
	StaffID int64
	RoomID  int64
}

const (
	InstanceStatusPlanned = "planned"
	InstanceStatusActive  = "active"
)

type ActivityInstance struct {
	ID        int64
	Date      calendar.Date
	Title     string
	RoomID    int64
	StartTime time.Time
	EndTime   time.Time
	Status    string
	// IsSpontaneous marks a block started on the spot. Its planned end is a
	// placeholder (start plus one hour), not a time it was meant to end.
	IsSpontaneous bool
}

type InstanceStaff struct {
	InstanceID   int64
	StaffID      int64
	IsAbsent     bool
	IsSubstitute bool
}
