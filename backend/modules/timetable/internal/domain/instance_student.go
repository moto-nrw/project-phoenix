package domain

import (
	"errors"
	"time"
)

var ErrInstanceStudentNotFound = errors.New("instance student assignment not found")

const InstanceStudentUniqueConstraint = "unique_instance_student"

// InstanceStudent is one planned participant of an activity instance. Its
// attendance is Student Presence's session attendance, keyed by ID.
type InstanceStudent struct {
	ID         int64
	TenantID   int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
	InstanceID int64
	StudentID  int64
	RoomID     *int64
}

type InstanceStudentFields struct {
	InstanceID int64
	StudentID  int64
	RoomID     *int64
}

type InstanceStudentFilter struct {
	IDs                        []int64
	InstanceIDs                []int64
	StudentIDs                 []int64
	Date                       *string
	FromDate                   *string
	ToDate                     *string
	CurrentTime                *string
	FromClock                  *string
	ExcludeCancelled           bool
	OrderByCreated             bool
	OrderByInstanceStudent     bool
	OrderByStudentActivityTime bool
	OrderByActivityDateTime    bool
	Limit                      int
	Offset                     int
}

type InstanceStudentKey struct {
	InstanceID int64
	StudentID  int64
}

type StudentInstanceRef struct {
	StudentID  int64
	InstanceID int64
}

// PlannedInstanceStudent is a participant with the planning day and block
// start of its instance.
type PlannedInstanceStudent struct {
	ID         int64
	InstanceID int64
	StudentID  int64
	Date       string
	StartTime  string
}

// ArchivedAttendance is the attendance snapshot a grade transition keeps
// beside a removed participant. Timetable stores it verbatim.
type ArchivedAttendance struct {
	Status             string
	Substatus          *string
	Note               *string
	IsUnplanned        bool
	NotScheduled       bool
	ManualStatusAt     *time.Time
	StudentStatusDayID *int64
}

type RosterArchiveEntry struct {
	ParticipantID int64
	Attendance    ArchivedAttendance
}

type RestoredInstanceStudent struct {
	ID         int64
	InstanceID int64
	StudentID  int64
	RoomID     *int64
	Date       string
	StartTime  string
	Attendance ArchivedAttendance
}

// RosterRemoval is one archived participant of a grade transition: the
// planning facts plus the attendance snapshot as archived.
type RosterRemoval struct {
	ID           int64
	TenantID     int64
	TransitionID int64
	InstanceID   int64
	StudentID    int64
	RoomID       *int64
	Attendance   ArchivedAttendance
	CreatedAt    time.Time
}

type RoomRef struct {
	ID       int64
	TenantID int64
}
