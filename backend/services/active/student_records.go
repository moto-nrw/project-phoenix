package active

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// StudentLifecycle is the presence view of where a child stands in the care
// lifecycle. Only the three states the presence flows branch on are named; the
// adapter maps every other owner status to StudentLifecycleOther, which is
// treated exactly like a pending enrollment. Modelling this as an enum rather
// than the owner's status string keeps the two vocabularies from drifting: a
// renamed owner constant breaks the adapter's switch at compile time.
type StudentLifecycle int

const (
	StudentLifecycleOther StudentLifecycle = iota
	StudentLifecycleActive
	StudentLifecycleInactive
	StudentLifecycleAlumnus
)

// Employment types as the workforce services group and label them. These are
// the stored column values; services/active owns the workforce domain, so it
// names them directly.
const (
	EmploymentTypeFullTime = "full_time"
	EmploymentTypePartTime = "part_time"
	EmploymentTypeMinijob  = "minijob"
)

// StudentRecord is the presence view of a child: identity, group membership,
// lifecycle, enrollment interval and the live sick/excused flags the
// check-in and status-day flows read and reset. It carries no departure plan
// or master data; the owner keeps those on its own row.
type StudentRecord struct {
	ID          int64
	TenantID    int64
	PersonID    int64
	GroupID     *int64
	SchoolClass string
	Lifecycle   StudentLifecycle

	EnrolledFrom  *timezone.Date
	EnrolledUntil *timezone.Date

	Sick         *bool
	SickSince    *time.Time
	Excused      *bool
	ExcusedSince *time.Time
}

// IsAlumnus reports whether the child has graduated out of care.
func (s *StudentRecord) IsAlumnus() bool {
	return s != nil && s.Lifecycle == StudentLifecycleAlumnus
}

// CareEndedOn reports whether the enrollment interval has ended before the
// given day; the lifecycle status may still lag behind the scheduler.
func (s *StudentRecord) CareEndedOn(day timezone.Date) bool {
	return s != nil && s.EnrolledUntil != nil && day.After(*s.EnrolledUntil)
}

// PresenceStudents is the student access the attendance service needs:
// row-locked reads for check-in guards and the live-flag write-back.
type PresenceStudents interface {
	FindByID(context.Context, int64) (*StudentRecord, error)
	FindByIDForUpdate(context.Context, int64) (*StudentRecord, error)
	FindByIDsForUpdate(context.Context, []int64) (map[int64]*StudentRecord, error)
	// UpdateLiveStatus persists the record's sick/excused flags and their
	// timestamps; no other column is touched.
	UpdateLiveStatus(context.Context, *StudentRecord) error
}

// StatusDayStudents supports the status-day write transaction: the locked
// read that re-authorizes the caller against the fresh row, and the live-flag
// write-back on that same row.
type StatusDayStudents interface {
	// LockForStatusWrite loads the student FOR UPDATE and re-checks that the
	// caller may write the given status for it. An unauthorized row yields
	// ErrStudentStatusDayReassigned.
	LockForStatusWrite(ctx context.Context, studentID int64, status string) (*StudentRecord, error)
	UpdateLiveStatus(context.Context, *StudentRecord) error
}

// PersonName is the display name of a person keyed by the person id.
type PersonName struct {
	ID        int64
	FirstName string
	LastName  string
}

// StaffScheduleBinding is the part of a staff row the schedule commands
// rewrite: the assigned work time model and the rotation anchor.
type StaffScheduleBinding struct {
	ID                 int64
	WorkTimeModelID    *int64
	RotationAnchorDate *timezone.Date
}
