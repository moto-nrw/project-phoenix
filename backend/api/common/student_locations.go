package common

import (
	"context"
	"fmt"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
)

// StudentLocationSnapshot caches attendance, visit, and group data for a set of students.
// Callers can reuse the snapshot to resolve location strings without triggering N+1 queries.
type StudentLocationSnapshot struct {
	Attendances map[int64]*activeService.AttendanceStatus
	Visits      map[int64]*activeModels.Visit
	Groups      map[int64]*activeModels.Group
}

// LoadStudentLocationSnapshot batches all data needed to resolve student locations.
func LoadStudentLocationSnapshot(ctx context.Context, svc activeService.Service, studentIDs []int64) (*StudentLocationSnapshot, error) {
	uniqueIDs := uniqueInt64(studentIDs)
	snapshot := &StudentLocationSnapshot{
		Attendances: make(map[int64]*activeService.AttendanceStatus),
		Visits:      make(map[int64]*activeModels.Visit),
		Groups:      make(map[int64]*activeModels.Group),
	}

	if len(uniqueIDs) == 0 {
		return snapshot, nil
	}

	attendances, err := svc.GetStudentsAttendanceStatuses(ctx, uniqueIDs)
	if err != nil {
		return nil, err
	}
	if attendances == nil {
		attendances = make(map[int64]*activeService.AttendanceStatus)
	}
	snapshot.Attendances = attendances

	checkedInIDs := make([]int64, 0, len(attendances))
	for studentID, status := range attendances {
		if status != nil && status.Status == "checked_in" {
			checkedInIDs = append(checkedInIDs, studentID)
		}
	}

	if len(checkedInIDs) == 0 {
		return snapshot, nil
	}

	visits, err := svc.GetStudentsCurrentVisits(ctx, checkedInIDs)
	if err != nil {
		return nil, err
	}
	if visits == nil {
		visits = make(map[int64]*activeModels.Visit)
	}
	snapshot.Visits = visits

	groupIDSet := make(map[int64]struct{})
	for _, visit := range visits {
		if visit != nil && visit.ActiveGroupID > 0 {
			groupIDSet[visit.ActiveGroupID] = struct{}{}
		}
	}

	if len(groupIDSet) == 0 {
		return snapshot, nil
	}

	groupIDs := make([]int64, 0, len(groupIDSet))
	for groupID := range groupIDSet {
		groupIDs = append(groupIDs, groupID)
	}

	groups, err := svc.GetActiveGroupsByIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	if groups == nil {
		groups = make(map[int64]*activeModels.Group)
	}
	snapshot.Groups = groups

	return snapshot, nil
}

// ResolveStudentLocation converts the cached data into the user-facing location string.
func (s *StudentLocationSnapshot) ResolveStudentLocation(studentID int64, hasFullAccess bool) string {
	if s == nil {
		return "Abwesend"
	}

	status, ok := s.Attendances[studentID]
	if !ok || status == nil || status.Status != "checked_in" {
		return "Abwesend"
	}

	if !hasFullAccess {
		return "Anwesend"
	}

	visit, ok := s.Visits[studentID]
	if !ok || visit == nil || visit.ActiveGroupID <= 0 {
		return "Unterwegs"
	}

	group, ok := s.Groups[visit.ActiveGroupID]
	if !ok || group == nil {
		return "Unterwegs"
	}

	if group.Room != nil && group.Room.Name != "" {
		return fmt.Sprintf("Anwesend - %s", group.Room.Name)
	}

	return "Unterwegs"
}

func uniqueInt64(ids []int64) []int64 {
	if len(ids) == 0 {
		return ids
	}

	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}

	return result
}
