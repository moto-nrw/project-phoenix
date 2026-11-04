package studentpresence

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Morning attendance statuses (#4790): the two states a Lehrkraft marks per
// child for one school morning. Lateness is no state of its own.
const (
	MorningAttendancePresent = "present"
	MorningAttendanceAbsent  = "absent"
)

// MorningAttendanceChild names exactly one child of a class: a student with
// an OGS record (StudentID) or a class-list-only entry without one (#2382,
// ClassListEntryID). The other ID is zero.
type MorningAttendanceChild struct {
	StudentID        int64
	ClassListEntryID int64
}

// MorningAttendanceMark is one child's mark as the Lehrkraft enters it.
type MorningAttendanceMark struct {
	Child  MorningAttendanceChild
	Status string
}

// MorningAttendanceEntry is one stored mark.
type MorningAttendanceEntry struct {
	Child MorningAttendanceChild
	// Date is the school morning the mark belongs to.
	Date       calendar.Date
	Status     string
	RecordedAt time.Time
}

// MorningAttendanceWrite records the marks of one school morning, attributed
// to the users.staff_school_memberships row of the Lehrkraft.
type MorningAttendanceWrite struct {
	Date       calendar.Date
	RecordedBy int64
	Marks      []MorningAttendanceMark
}

// MorningAttendance stores what the Lehrkraft of a class marked for the
// school morning (#4790), separate from the OGS attendance: no method here
// reads or writes active.attendance. Which children belong to the class and
// whether the caller may mark them is decided by the caller before.
type MorningAttendance interface {
	// MorningAttendanceOn returns the stored marks of date for the named
	// children; a child without a mark has no entry.
	MorningAttendanceOn(ctx context.Context, date calendar.Date, children []MorningAttendanceChild) ([]MorningAttendanceEntry, error)
	// MorningAttendanceBetween returns the stored marks of every date from
	// from to to (both included) for the named children, for the absence
	// list of a class (#4793).
	MorningAttendanceBetween(ctx context.Context, from, to calendar.Date, children []MorningAttendanceChild) ([]MorningAttendanceEntry, error)
	// RecordMorningAttendance stores the marks, replacing a child's earlier
	// mark of the same date. ErrMorningAttendanceInvalid refuses a mark that
	// names no child, both kinds of child, or an unknown status.
	RecordMorningAttendance(ctx context.Context, in MorningAttendanceWrite) error
}

// MorningAttendanceRetention deletes the morning attendance and the class
// handovers (#4794) once the school's retention window has passed (#4790),
// counted from the school day. Like
// AttendanceRetention it joins the nightly cleanup transaction.
type MorningAttendanceRetention interface {
	DeleteExpiredMorningAttendance(ctx context.Context, retentionDays int) (int, error)
}

// ErrMorningAttendanceInvalid refuses a malformed morning attendance write.
var ErrMorningAttendanceInvalid = errors.New("morning attendance mark is invalid")

// MorningAbsenceGuard holds an absent morning mark until the caller commits.
// Writers of that mark serialize through the same row lock.
type MorningAbsenceGuard interface {
	RequireAbsentMorningAttendance(context.Context, int64, calendar.Date) error
}

// ErrMorningAttendanceNotAbsent means the teacher has corrected the mark.
var ErrMorningAttendanceNotAbsent = errors.New("child is no longer marked absent")
