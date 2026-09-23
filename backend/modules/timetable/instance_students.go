package timetable

import (
	"context"
	"time"
)

// InstanceStudent is one planned participant of an activity instance: the
// child and the room the plan places them in (#2762). What was observed or
// decided for the participant is Student Presence's session attendance,
// keyed by this row's ID.
type InstanceStudent struct {
	ID         int64
	TenantID   int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
	InstanceID int64
	StudentID  int64
	RoomID     *int64
}

type InstanceStudentInput struct {
	InstanceID int64
	StudentID  int64
	RoomID     *int64
}

// InstanceStudentFilter selects planned participants. Date, FromDate and
// ToDate name the instance's planning day; CurrentTime keeps the instances
// whose block covers that clock on Date and are not cancelled.
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

// PlannedInstanceStudent is a participant together with the planning day and
// block start of its instance, the shape the attendance rules of Student
// Presence resolve their participants from.
type PlannedInstanceStudent struct {
	ID         int64
	InstanceID int64
	StudentID  int64
	Date       string
	StartTime  string
}

// ArchivedAttendance is the attendance snapshot a grade transition keeps
// beside a removed participant, so a restore can hand it back to Student
// Presence. Timetable stores it verbatim and never interprets it.
type ArchivedAttendance struct {
	Status             string
	Substatus          *string
	Note               *string
	IsUnplanned        bool
	NotScheduled       bool
	ManualStatusAt     *time.Time
	StudentStatusDayID *int64
}

// RosterArchiveEntry is one participant a grade transition removes together
// with the attendance snapshot the caller read from its owner.
type RosterArchiveEntry struct {
	ParticipantID int64
	Attendance    ArchivedAttendance
}

// RestoredInstanceStudent is one participant a grade transition restored,
// with the planning facts and the archived attendance the caller hands back
// to Student Presence.
type RestoredInstanceStudent struct {
	ID         int64
	InstanceID int64
	StudentID  int64
	RoomID     *int64
	Date       string
	StartTime  string
	Attendance ArchivedAttendance
}

type RoomRef struct {
	ID       int64
	TenantID int64
}

type RoomDirectory interface {
	LockRoomsByID(context.Context, []int64) ([]RoomRef, error)
}

type RoomDirectoryFunc func(context.Context, []int64) ([]RoomRef, error)

func (f RoomDirectoryFunc) LockRoomsByID(ctx context.Context, ids []int64) ([]RoomRef, error) {
	return f(ctx, ids)
}

type InstanceStudentQuery interface {
	FindInstanceStudent(context.Context, int64) (InstanceStudent, error)
	ListInstanceStudents(context.Context, InstanceStudentFilter) ([]InstanceStudent, error)
	// ListPlannedInstanceStudents lists participants with their instance's
	// planning day and block start.
	ListPlannedInstanceStudents(context.Context, InstanceStudentFilter) ([]PlannedInstanceStudent, error)
	CountStudentAssignments(context.Context, int64) (int, error)
	CountStudentRosterRemovals(context.Context, int64) (int, error)
	ListStudentInstanceRefsBefore(context.Context, string) ([]StudentInstanceRef, error)
	ListPlannedStudentIDs(context.Context, []int64, string) ([]int64, error)
}

type InstanceStudentCommand interface {
	// LockInstanceStudentAssignments holds the participant rows of the
	// instance until the caller's tenant transaction ends.
	LockInstanceStudentAssignments(context.Context, int64) error
	DeleteStudentAssignments(context.Context, int64) (int64, error)
	CreateInstanceStudent(context.Context, InstanceStudentInput) (InstanceStudent, error)
	// EnsureInstanceStudent adds the child to the instance's roster when it is
	// not on it yet and reports whether it inserted the participant.
	EnsureInstanceStudent(context.Context, int64, int64) (InstanceStudent, bool, error)
	UpdateInstanceStudent(context.Context, int64, InstanceStudentInput) (InstanceStudent, error)
	DeleteInstanceStudent(context.Context, int64) error
	DeleteInstanceStudentsByInstance(context.Context, int64) error
	// ArchivePlannedInstanceStudents removes the participants of a grade
	// transition together with the attendance snapshots the caller read and
	// keeps them for a restore.
	ArchivePlannedInstanceStudents(context.Context, int64, []RosterArchiveEntry) (int, error)
	// RestoreArchivedInstanceStudents puts the archived participants of the
	// students back onto rosters from the date on, skipping instances that
	// ended, were cancelled, or list the child again, and returns what it
	// restored.
	RestoreArchivedInstanceStudents(context.Context, int64, []int64, string) ([]RestoredInstanceStudent, error)
}

type InstanceStudentCapability interface {
	InstanceStudentQuery
	InstanceStudentCommand
}

func validInstanceStudent(input InstanceStudentInput) bool {
	return input.InstanceID > 0 && input.StudentID > 0 && (input.RoomID == nil || *input.RoomID > 0)
}

func validInstanceStudentFilter(filter InstanceStudentFilter) bool {
	orders := 0
	for _, ordered := range []bool{filter.OrderByCreated, filter.OrderByInstanceStudent,
		filter.OrderByStudentActivityTime, filter.OrderByActivityDateTime} {
		if ordered {
			orders++
		}
	}
	return filter.Limit >= 0 && filter.Offset >= 0 && orders <= 1 &&
		!hasInvalidID(filter.IDs) && !hasInvalidID(filter.InstanceIDs) && !hasInvalidID(filter.StudentIDs) &&
		validOptionalDate(filter.Date) && validOptionalDate(filter.FromDate) && validOptionalDate(filter.ToDate) &&
		(filter.CurrentTime == nil || (filter.Date != nil && validClock(*filter.CurrentTime))) &&
		(filter.FromClock == nil || validClock(*filter.FromClock))
}

func validRosterArchiveEntries(entries []RosterArchiveEntry) bool {
	for _, entry := range entries {
		if entry.ParticipantID <= 0 || entry.Attendance.Status == "" {
			return false
		}
	}
	return true
}
