package domain

import (
	"time"
)

const InstanceStudentUniqueConstraint = "unique_instance_student"

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

// ArchivedAttendance is the attendance snapshot a grade transition keeps
// beside a removed participant. Timetable stores it verbatim. It mirrors the
// public ArchivedAttendance because the store result above cannot name the
// public type from this package.
type ArchivedAttendance struct {
	Status             string
	Substatus          *string
	Note               *string
	IsUnplanned        bool
	NotScheduled       bool
	ManualStatusAt     *time.Time
	StudentStatusDayID *int64
}
