package studentpresence

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ClassHandoverChange hands one class over to the OGS after lessons (#4794),
// attributed to the users.staff_school_memberships row of the Lehrkraft, or
// takes the handover of that class and date back (Withdraw). On a handover,
// PresentStudentIDs and AbsentStudentIDs are the children with an OGS record
// she handed over "da" and "nicht da"; either list may be empty.
type ClassHandoverChange struct {
	SchoolClass       string
	Date              calendar.Date
	HandedOverBy      int64
	PresentStudentIDs []int64
	AbsentStudentIDs  []int64
	Withdraw          bool
}

// ClassHandover is the stored handover of one class and school day.
// HandedOverByStaffID is zero when the staff member was removed since.
type ClassHandover struct {
	SchoolClass         string
	Date                calendar.Date
	HandedOverAt        time.Time
	HandedOverByStaffID int64
	PresentStudentIDs   []int64
	AbsentStudentIDs    []int64
}

// ClassHandovers stores the handover of a class after lessons (#4794): when
// and by whom the Lehrkraft handed her class to the OGS, and which children
// she handed over "da" and "nicht da". The handover is a moment of its own: no method here reads
// or writes the morning attendance or the OGS attendance. Which children
// belong to the class and whether the caller may hand it over is decided by
// the caller before.
type ClassHandovers interface {
	// ClassHandoversOn returns every handover of date, each with its
	// children; a school day has at most one per class.
	ClassHandoversOn(ctx context.Context, date calendar.Date) ([]ClassHandover, error)
	// ChangeClassHandover stores a handover, replacing one of the same class
	// (matched trimmed and lowercased) and date with the new moment, staff
	// member and children, or deletes it on Withdraw. It reports whether a
	// handover of the class and date existed before.
	// ErrClassHandoverInvalid refuses a change without class or date, a
	// handover without staff member, a child ID that is not positive, or a
	// child both "da" and "nicht da".
	ChangeClassHandover(ctx context.Context, in ClassHandoverChange) (bool, error)
}

// ErrClassHandoverInvalid refuses a malformed class handover.
var ErrClassHandoverInvalid = errors.New("class handover is invalid")
