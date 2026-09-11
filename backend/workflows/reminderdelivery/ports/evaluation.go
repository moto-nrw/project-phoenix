package ports

import "time"

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
	Title     string
	RoomID    int64
	StartTime time.Time
	EndTime   time.Time
	Status    string
}

type InstanceStaff struct {
	InstanceID   int64
	StaffID      int64
	IsAbsent     bool
	IsSubstitute bool
}
