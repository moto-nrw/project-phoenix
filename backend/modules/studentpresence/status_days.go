package studentpresence

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
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

// ErrStudentStatusDayReassigned signals that a student left the caller's scope
// between the initial authorization and the locked re-read inside the write
// transaction. Handlers map it to 403.
var ErrStudentStatusDayReassigned = errors.New("student reassigned out of caller scope")

// ErrStudentStatusDayPartialAbsenceConflict prevents broad day statuses from
// erasing a more precise time-specific excusal without an explicit decision.
var ErrStudentStatusDayPartialAbsenceConflict = errors.New("student status day conflicts with a partial absence")

// MaxStudentStatusDayConflictDetails is the maximum number of conflict rows
// returned in a 409 response body. Larger totals are reported via Total only so
// bulk class-trip writes cannot serialize tens of thousands of rows.
const MaxStudentStatusDayConflictDetails = 32

// StudentStatusDayConflictError reports active rows that prevented an atomic
// planned-status write. Callers must surface these rows to the user instead of
// clearing or replacing them. Conflicts may be a capped sample; Total is the
// full count when set (or len(Conflicts) when zero).
type StudentStatusDayConflictError struct {
	Conflicts []*absencerecords.StudentStatusDay
	// Total is the full conflict count before sampling. Zero means "use
	// len(Conflicts)" for single-student writes that return the full set.
	Total int
}

func (e *StudentStatusDayConflictError) Error() string {
	return "student status days conflict with active rows"
}

// ConflictTotal returns the full conflict count for API payloads.
func (e *StudentStatusDayConflictError) ConflictTotal() int {
	if e == nil {
		return 0
	}
	if e.Total > 0 {
		return e.Total
	}
	return len(e.Conflicts)
}

// SampleConflicts returns at most MaxStudentStatusDayConflictDetails rows for
// a 409 body. Bulk writers should already keep Conflicts within this bound.
func (e *StudentStatusDayConflictError) SampleConflicts() []*absencerecords.StudentStatusDay {
	if e == nil {
		return nil
	}
	if len(e.Conflicts) <= MaxStudentStatusDayConflictDetails {
		return e.Conflicts
	}
	return e.Conflicts[:MaxStudentStatusDayConflictDetails]
}

// StatusDayWriteContext carries the collaborators the status-day write
// orchestration needs but the StudentStatusDayService is not constructed with,
// keeping the repo-only constructor stable. The authorize callback keeps the
// JWT-permission decision at the HTTP boundary while the transaction
// composition lives in the service. Durable notifications run inside the
// transaction; after-commit runs only the ephemeral SSE fan-out.
type StatusDayWriteContext struct {
	DB             DatabaseHandle
	TenantID       int64
	StudentService StatusDayStudents
	AfterCommit    func(studentID int64)
	// AfterCreate receives the students whose current-day status newly became a
	// reportable absence inside the write transaction. It is separate from
	// AfterCommit so bulk writes can emit one aggregate absence notification
	// while retaining the per-student SSE fan-out.
	AfterCreate func(context.Context, []int64) error
}

// ApplyLiveStatusForToday mutates a student's live sick/excused flags to reflect
// a status reported for today (mutually exclusive across sick/excused/class
// trip).
func ApplyLiveStatusForToday(student *StudentRecord, status string, now time.Time) {
	trueVal := true
	falseVal := false
	switch status {
	case absencerecords.StudentStatusDaySick:
		student.Sick = &trueVal
		student.SickSince = &now
		student.Excused = &falseVal
		student.ExcusedSince = nil
	case absencerecords.StudentStatusDayExcused:
		student.Excused = &trueVal
		student.ExcusedSince = &now
		student.Sick = &falseVal
		student.SickSince = nil
	case absencerecords.StudentStatusDayClassTrip:
		student.Sick = &falseVal
		student.SickSince = nil
		student.Excused = &falseVal
		student.ExcusedSince = nil
	}
}

// ClearLiveStatusForToday resets the live flags a today status set.
func ClearLiveStatusForToday(student *StudentRecord, status string) {
	falseVal := false
	switch status {
	case absencerecords.StudentStatusDaySick:
		student.Sick = &falseVal
		student.SickSince = nil
	case absencerecords.StudentStatusDayExcused:
		student.Excused = &falseVal
		student.ExcusedSince = nil
	case absencerecords.StudentStatusDayClassTrip:
		student.Sick = &falseVal
		student.SickSince = nil
		student.Excused = &falseVal
		student.ExcusedSince = nil
	}
}

// StatusDayOverviewGroup identifies an authorized group in the overview.
type StatusDayOverviewGroup struct {
	ID   int64
	Name string
}

type StatusDayOverviewEntry struct {
	StatusDay *absencerecords.StudentStatusDay
	Student   *StudentRecord
	Person    *PersonName
	Group     *StatusDayOverviewGroup
}

type StatusDayOverviewFilters struct {
	Query    string
	Status   string
	Page     int
	PageSize int
}

type StatusDayOverview struct {
	Entries []StatusDayOverviewEntry
	HasMore bool
}

// DatabaseHandle marks a live database behind the tenant unit of work. The
// retained services only test it for presence: a nil handle is the shape unit
// tests build, where every repository is a double and nothing needs a
// transaction, while production wiring always supplies one.
type DatabaseHandle interface {
	PingContext(context.Context) error
}
