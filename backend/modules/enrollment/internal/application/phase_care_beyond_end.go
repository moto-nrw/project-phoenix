package application

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// careBeyondShortenedEnd lists the confirmed children of the phase whose
// care now runs past its shortened end (#6598, ADR 0211). Nothing is
// written: the school ends each care on purpose through the care-end process.
//
// It reads after the follow steps, in the edit's transaction and under the
// recurrence gate the edit took with the PhaseBoundsReader. A care end takes
// the same gate, so the list matches every committed care end.
//
// A child is listed when
//   - it is in care (pending or active); a child whose care ended is history,
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
		if !cutByEdit(student, oldEnd, newEnd, today, otherReach) {
			continue
		}
		result = append(result, enrollment.CareBeyondPhaseEnd{
			StudentID: student.ID, FirstName: student.FirstName, LastName: student.LastName,
			SchoolClass: student.SchoolClass, CareEnd: student.CareEnd, PhaseEnd: newEnd,
		})
	}
	slices.SortFunc(result, compareCareBeyondEnd)
	return result, nil
}

// cutByEdit reports whether the edit from oldEnd to newEnd cut off the
// child's care: it is in care, its care end lies after the new and not after
// the old end, has not passed, and no approval in another phase reaches it.
func cutByEdit(student enrollment.PhaseCareStudent, oldEnd, newEnd, today calendar.Date, otherReach map[int64]calendar.Date) bool {
	if !student.InCare {
		return false
	}
	end := student.CareEnd
	if end.IsZero() || !end.After(newEnd) || end.After(oldEnd) || end.Before(today) {
		return false
	}
	reach, ok := otherReach[student.ID]
	return !ok || reach.Before(end)
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

// compareCareBeyondEnd orders the children like every other name list
// (German collation, last name first), then by id.
func compareCareBeyondEnd(a, b enrollment.CareBeyondPhaseEnd) int {
	if c := compareGermanNames(a.LastName, a.FirstName, b.LastName, b.FirstName); c != 0 {
		return c
	}
	switch {
	case a.StudentID < b.StudentID:
		return -1
	case a.StudentID > b.StudentID:
		return 1
	}
	return 0
}
