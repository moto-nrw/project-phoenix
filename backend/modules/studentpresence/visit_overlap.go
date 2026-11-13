package studentpresence

import (
	"errors"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// ErrVisitOverlap classifies VisitOverlapError with errors.Is (#5710).
var ErrVisitOverlap = errors.New("visit overlaps another visit of the student")

// VisitOverlapCode is the stable error code of a visit correction refused
// because the new interval overlaps another visit of the same child.
const VisitOverlapCode = "presence.visit_overlap"

// VisitOverlapError refuses a visit correction whose interval overlaps another
// visit of the same child (#5710). A child is in one room at a time; letting
// the intervals cross would leave the readers to guess which stay counts.
// Visits that only touch (one ends when the next starts) do not overlap.
// Nothing of the correction is applied.
type VisitOverlapError struct {
	VisitID          int64
	ConflictingVisit Visit
}

func (e *VisitOverlapError) Error() string {
	return fmt.Sprintf("visit %d overlaps visit %d of student %d", e.VisitID, e.ConflictingVisit.ID, e.ConflictingVisit.StudentID)
}

func (e *VisitOverlapError) Unwrap() error { return ErrVisitOverlap }

func (e *VisitOverlapError) ErrorCode() string { return VisitOverlapCode }

// VisitOverlapDetails are the values a refusal names, in wire form. The clock
// times are Berlin wall clock ("15:04"); ConflictingUntil is empty while the
// other visit is still open.
type VisitOverlapDetails struct {
	ConflictingVisitID int64  `json:"conflicting_visit_id"`
	ConflictingFrom    string `json:"conflicting_from"`
	ConflictingUntil   string `json:"conflicting_until"`
}

func (e *VisitOverlapError) ErrorDetails() any {
	details := VisitOverlapDetails{
		ConflictingVisitID: e.ConflictingVisit.ID,
		ConflictingFrom:    e.ConflictingVisit.EntryTime.In(calendar.Berlin).Format("15:04"),
	}
	if e.ConflictingVisit.ExitTime != nil {
		details.ConflictingUntil = e.ConflictingVisit.ExitTime.In(calendar.Berlin).Format("15:04")
	}
	return details
}

// VisitsOverlap reports whether two visit intervals share an instant beyond a
// common boundary. A nil exit is an open visit that runs on; empty intervals
// occupy no time, including when another visit spans their boundary.
func VisitsOverlap(a, b Visit) bool {
	if (a.ExitTime != nil && a.EntryTime.Equal(*a.ExitTime)) ||
		(b.ExitTime != nil && b.EntryTime.Equal(*b.ExitTime)) {
		return false
	}
	return startsBeforeEnd(a.EntryTime, b.ExitTime) && startsBeforeEnd(b.EntryTime, a.ExitTime)
}

func startsBeforeEnd(start time.Time, end *time.Time) bool {
	return end == nil || start.Before(*end)
}
