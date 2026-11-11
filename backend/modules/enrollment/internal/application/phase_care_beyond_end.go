package application

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// careBeyondShortenedEnd lists the confirmed children of the phase whose
// care now runs past its shortened end (#6598, ADR 0210). Nothing is
// written: the school ends each care on purpose through the care-end process.
//
// It reads after the follow steps, in the edit's transaction and under the
// recurrence gate the edit took with the PhaseBoundsReader. A care end takes
// the same gate, so the list matches every committed care end.
//
// A child is listed when
//   - it is pending or active; a child whose care ended is history,
//   - its care end lies after the new phase end and not after the old one:
//     this edit opened the gap, not an earlier edit or a later end the
//     school set by hand,
//   - its care end has not passed yet, and
//   - none of its approvals in another phase reaches its care end.
func (s *Phases) careBeyondShortenedEnd(ctx context.Context, before, after *enrollment.Phase) ([]enrollment.CareBeyondPhaseEnd, error) {
	oldEnd, newEnd := calendar.Date(before.ServiceEndDate), calendar.Date(after.ServiceEndDate)
	if s.deps.CareStudents == nil || !newEnd.Before(oldEnd) {
		return nil, nil
	}
	studentIDs, err := s.confirmedPhaseStudents(ctx, after.ID)
	if err != nil || len(studentIDs) == 0 {
		return nil, err
	}
	otherReach, err := s.otherPhaseReach(ctx, after.ID, studentIDs)
	if err != nil {
		return nil, err
	}
	students, err := s.deps.CareStudents.PhaseCareStudents(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("phase update: read care ends of confirmed children: %w", err)
	}
	today := s.todayDate()
	var result []enrollment.CareBeyondPhaseEnd
	for _, student := range students {
		end := student.CareEnd
		if student.Status != enrollment.PhaseCareStudentPending && student.Status != enrollment.PhaseCareStudentActive {
			continue
		}
		if end.IsZero() || !end.After(newEnd) || end.After(oldEnd) || end.Before(today) {
			continue
		}
		if reach, ok := otherReach[student.ID]; ok && !reach.Before(end) {
			continue
		}
		result = append(result, enrollment.CareBeyondPhaseEnd{
			StudentID: student.ID, FirstName: student.FirstName, LastName: student.LastName,
			SchoolClass: student.SchoolClass, CareEnd: end, PhaseEnd: newEnd,
		})
	}
	slices.SortFunc(result, func(a, b enrollment.CareBeyondPhaseEnd) int {
		return cmp.Or(cmp.Compare(a.LastName, b.LastName), cmp.Compare(a.FirstName, b.FirstName), cmp.Compare(a.StudentID, b.StudentID))
	})
	return result, nil
}

// confirmedPhaseStudents lists, in ascending order, the children an approval
// in this phase linked to a child record.
func (s *Phases) confirmedPhaseStudents(ctx context.Context, phaseID int64) ([]int64, error) {
	children, err := s.deps.Records.PhaseResponseChildren(ctx, phaseID, []string{enrollment.ChildStatusApproved})
	if err != nil {
		return nil, fmt.Errorf("phase update: list confirmed children: %w", err)
	}
	seen := map[int64]bool{}
	ids := []int64{}
	for _, row := range children {
		if !row.IsPhaseChild || row.Child == nil || row.Child.CreatedStudentID == nil || seen[*row.Child.CreatedStudentID] {
			continue
		}
		seen[*row.Child.CreatedStudentID] = true
		ids = append(ids, *row.Child.CreatedStudentID)
	}
	slices.Sort(ids)
	return ids, nil
}

// otherPhaseReach maps each child to the latest end of its approvals in
// other phases.
func (s *Phases) otherPhaseReach(ctx context.Context, phaseID int64, studentIDs []int64) (map[int64]calendar.Date, error) {
	periods, err := s.deps.Records.StudentCarePeriodsByStudentIDs(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("phase update: list approvals of confirmed children: %w", err)
	}
	reach := map[int64]calendar.Date{}
	for _, period := range periods {
		if period == nil || period.PhaseID == phaseID {
			continue
		}
		end := calendar.Date(period.ServiceEndDate)
		if current, ok := reach[period.StudentID]; !ok || end.After(current) {
			reach[period.StudentID] = end
		}
	}
	return reach, nil
}
