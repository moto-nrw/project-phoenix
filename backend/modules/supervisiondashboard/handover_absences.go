package supervisiondashboard

import (
	"context"
	"fmt"
)

// applyHandoverAbsences marks the children of the dashboard a class handover
// marked "nicht da" today (#4794): the open visits, the released rooms and
// the planned blocks' rosters. Like sick and excused it needs users:read.
func (s *service) applyHandoverAbsences(ctx context.Context, projection *Projection, caller Caller, day Date) error {
	if s.deps.Handovers == nil || !caller.CanReadStudents {
		return nil
	}
	ids := handoverCandidateIDs(projection)
	if len(ids) == 0 {
		return nil
	}
	absences, err := s.deps.Handovers.HandoverAbsences(ctx, ids, day)
	if err != nil {
		return fmt.Errorf("load class handover absences: %w", err)
	}
	if len(absences) == 0 {
		return nil
	}
	mark := func(studentID int64) *HandoverAbsence {
		if absence, ok := absences[studentID]; ok {
			return &absence
		}
		return nil
	}
	for i := range projection.Visits {
		projection.Visits[i].HandoverAbsent = mark(projection.Visits[i].StudentID)
	}
	for i := range projection.OpenRooms {
		for j := range projection.OpenRooms[i].Students {
			visit := &projection.OpenRooms[i].Students[j].Visit
			visit.HandoverAbsent = mark(visit.StudentID)
		}
	}
	for i := range projection.PlannedNow {
		for j := range projection.PlannedNow[i].RosterPreview {
			row := &projection.PlannedNow[i].RosterPreview[j]
			row.HandoverAbsent = mark(row.StudentID)
		}
	}
	return nil
}

// handoverCandidateIDs is the union of the children the dashboard shows.
func handoverCandidateIDs(projection *Projection) []int64 {
	seen := map[int64]bool{}
	var ids []int64
	add := func(id int64) {
		if id > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for _, visit := range projection.Visits {
		add(visit.StudentID)
	}
	for _, room := range projection.OpenRooms {
		for _, student := range room.Students {
			add(student.Visit.StudentID)
		}
	}
	for _, instance := range projection.PlannedNow {
		for _, row := range instance.RosterPreview {
			add(row.StudentID)
		}
	}
	return ids
}
