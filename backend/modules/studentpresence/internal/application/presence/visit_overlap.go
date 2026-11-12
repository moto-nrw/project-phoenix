package presence

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// ensureRevisionDoesNotOverlap refuses a correction whose interval would
// overlap another visit of the same child (#5710). prepareVisitRevision runs it
// after locking the child, so no write of the same child can slip in between,
// and before any write, so a refusal leaves visits and attendance untouched.
// A correction that keeps child and interval checks nothing: a pure move of an
// open visit adds no overlap.
func (s *service) ensureRevisionDoesNotOverlap(ctx context.Context, existing, updated *studentpresence.Visit) error {
	if existing.StudentID == updated.StudentID && !visitIntervalChanged(existing, updated) {
		return nil
	}
	from := updated.EntryTime
	visits, err := s.SchoolPresence.ListVisits(ctx, studentpresence.VisitFilter{
		StudentIDs:   []int64{updated.StudentID},
		OverlapFrom:  &from,
		OverlapUntil: updated.ExitTime,
	})
	if err != nil {
		return &ActiveError{Op: "UpdateVisit", Err: ErrDatabaseOperation}
	}
	for _, other := range visits {
		if other.ID == updated.ID || !studentpresence.VisitsOverlap(*updated, other) {
			continue
		}
		if updated.ExitTime == nil && other.ExitTime == nil {
			// Two open visits keep their established answer: the open-visit
			// index refuses the write with ErrStudentAlreadyActive.
			continue
		}
		return &ActiveError{Op: "UpdateVisit", Err: &studentpresence.VisitOverlapError{VisitID: updated.ID, ConflictingVisit: other}}
	}
	return nil
}
