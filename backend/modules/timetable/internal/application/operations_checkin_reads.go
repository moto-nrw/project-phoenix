package application

import (
	"context"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
)

// checkInReads holds what a batch check-in read once for all its children
// before the first write (#5815, decision E2): each child's open visit and,
// for an assignment-bound portal, the block's planned rows, the session's
// visitors and the children's rows. Each child still runs its checks in
// request order, so the first failing child answers as before. A child that
// appears again in the request reads fresh: its first check-in changed what
// the shared read saw.
type checkInReads struct {
	open     map[int64]*studentpresence.Visit
	roster   bool
	planned  map[int64]bool
	visitors map[int64]bool
	excluded map[int64]bool
	used     map[int64]bool
}

func (s *operations) loadCheckInReads(ctx context.Context, inst *scheduleModels.ActivityInstance, instanceID int64, studentIDs []int64) (*checkInReads, error) {
	ids := slices.Compact(slices.Sorted(slices.Values(studentIDs)))
	reads := &checkInReads{open: make(map[int64]*studentpresence.Visit, len(ids)), used: make(map[int64]bool, len(ids))}
	visits, err := s.deps.Visits.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: ids, OpenOnly: true, NewestFirst: true})
	if err != nil {
		return nil, err
	}
	for i := range visits {
		if reads.open[visits[i].StudentID] == nil {
			reads.open[visits[i].StudentID] = &visits[i]
		}
	}
	if !s.isAssignmentBoundPortal(ctx) {
		return reads, nil
	}
	if err := reads.loadRoster(ctx, s, inst, instanceID, ids); err != nil {
		return nil, err
	}
	return reads, nil
}

// loadRoster reads the roster union blockHoldsStudent checks and the
// exclusion rosterStudentExcluded applies, once for every child.
func (r *checkInReads) loadRoster(ctx context.Context, s *operations, inst *scheduleModels.ActivityInstance, instanceID int64, ids []int64) error {
	planned, err := participantsOfInstance(ctx, s.deps.Participants.Reads, instanceID)
	if err != nil {
		return err
	}
	r.roster, r.planned, r.visitors = true, make(map[int64]bool, len(planned)), map[int64]bool{}
	for _, row := range planned {
		r.planned[row.StudentID] = true
	}
	if inst != nil && inst.ActiveGroupID != nil {
		visits, err := s.deps.Visits.ListVisits(ctx, studentpresence.VisitFilter{ActiveGroupIDs: []int64{*inst.ActiveGroupID}})
		if err != nil {
			return err
		}
		for _, visit := range visits {
			r.visitors[visit.StudentID] = true
		}
	}
	students, err := s.deps.Students.FindByIDs(ctx, ids)
	if err != nil {
		return err
	}
	r.excluded = rosterExcludedAlumni(inst, students, s.today())
	return nil
}

// requireRosterStudent is operations.requireRosterStudent over the shared
// reads: an assignment-bound portal writes only a child on the roster who is
// not excluded.
func (r *checkInReads) requireRosterStudent(studentID int64) error {
	if !r.roster {
		return nil
	}
	if !r.planned[studentID] && !r.visitors[studentID] {
		return timetable.ErrTimetableOperationForbidden
	}
	if r.excluded[studentID] {
		return timetable.ErrTimetableOperationForbidden
	}
	return nil
}

// take reports whether the child's shared reads still hold, and marks them
// used: only a child's first check-in in the request may use them.
func (r *checkInReads) take(studentID int64) bool {
	if r == nil || r.used[studentID] {
		return false
	}
	r.used[studentID] = true
	return true
}
