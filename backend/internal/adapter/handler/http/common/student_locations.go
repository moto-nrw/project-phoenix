package common

import (
	"context"
	"fmt"
	"time"

	activeModels "github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
)

// StudentLocationInfo contains resolved location data including timestamps
type StudentLocationInfo struct {
	Location string
	Since    *time.Time // When the student entered this location (nil if not in a room)
}

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
	snapshot := newEmptyLocationSnapshot()

	if len(uniqueIDs) == 0 {
		return snapshot, nil
	}

	// Load attendances
	attendances, err := svc.GetStudentsAttendanceStatuses(ctx, uniqueIDs)
	if err != nil {
		return nil, err
	}
	snapshot.Attendances = coalesceMap(attendances, snapshot.Attendances)

	// Find checked-in students for visit lookup
	checkedInIDs := filterCheckedInStudents(snapshot.Attendances)
	if len(checkedInIDs) == 0 {
		return snapshot, nil
	}

	// Load visits
	visits, err := svc.GetStudentsCurrentVisits(ctx, checkedInIDs)
	if err != nil {
		return nil, err
	}
	snapshot.Visits = coalesceVisitMap(visits, snapshot.Visits)

	// Load groups for active visits
	groupIDs := extractActiveGroupIDs(snapshot.Visits)
	if len(groupIDs) == 0 {
		return snapshot, nil
	}

	groups, err := svc.GetActiveGroupsByIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	snapshot.Groups = coalesceGroupMap(groups, snapshot.Groups)

	return snapshot, nil
}

// newEmptyLocationSnapshot creates a new snapshot with initialized empty maps
func newEmptyLocationSnapshot() *StudentLocationSnapshot {
	return &StudentLocationSnapshot{
		Attendances: make(map[int64]*activeService.AttendanceStatus),
		Visits:      make(map[int64]*activeModels.Visit),
		Groups:      make(map[int64]*activeModels.Group),
	}
}

// filterCheckedInStudents returns IDs of students with checked_in status
func filterCheckedInStudents(attendances map[int64]*activeService.AttendanceStatus) []int64 {
	result := make([]int64, 0, len(attendances))
	for studentID, status := range attendances {
		if status != nil && status.Status == activeService.StatusCheckedIn {
			result = append(result, studentID)
		}
	}
	return result
}

// extractActiveGroupIDs extracts unique group IDs from visits
func extractActiveGroupIDs(visits map[int64]*activeModels.Visit) []int64 {
	groupIDSet := make(map[int64]struct{})
	for _, visit := range visits {
		if visit != nil && visit.ActiveGroupID > 0 {
			groupIDSet[visit.ActiveGroupID] = struct{}{}
		}
	}
	result := make([]int64, 0, len(groupIDSet))
	for groupID := range groupIDSet {
		result = append(result, groupID)
	}
	return result
}

// coalesceMap returns m if non-nil, otherwise fallback
func coalesceMap(m, fallback map[int64]*activeService.AttendanceStatus) map[int64]*activeService.AttendanceStatus {
	if m != nil {
		return m
	}
	return fallback
}

// coalesceVisitMap returns m if non-nil, otherwise fallback
func coalesceVisitMap(m, fallback map[int64]*activeModels.Visit) map[int64]*activeModels.Visit {
	if m != nil {
		return m
	}
	return fallback
}

// coalesceGroupMap returns m if non-nil, otherwise fallback
func coalesceGroupMap(m, fallback map[int64]*activeModels.Group) map[int64]*activeModels.Group {
	if m != nil {
		return m
	}
	return fallback
}

// ResolveStudentLocation converts the cached data into the user-facing location string.
func (s *StudentLocationSnapshot) ResolveStudentLocation(studentID int64, hasFullAccess bool) string {
	info := s.ResolveStudentLocationWithTime(studentID, hasFullAccess)
	return info.Location
}

// ResolveStudentLocationWithTime converts the cached data into location info including entry time.
func (s *StudentLocationSnapshot) ResolveStudentLocationWithTime(studentID int64, hasFullAccess bool) StudentLocationInfo {
	if s == nil {
		return StudentLocationInfo{Location: "Abwesend"}
	}

	status, ok := s.Attendances[studentID]
	if !ok || status == nil {
		return StudentLocationInfo{Location: "Abwesend"}
	}

	// If checked out, return "Abwesend" with checkout time (for hasFullAccess users)
	if status.Status == activeService.StatusCheckedOut {
		if hasFullAccess && status.CheckOutTime != nil {
			return StudentLocationInfo{Location: "Abwesend", Since: status.CheckOutTime}
		}
		return StudentLocationInfo{Location: "Abwesend"}
	}

	// If not checked in at all, return "Abwesend" without time
	if status.Status != activeService.StatusCheckedIn {
		return StudentLocationInfo{Location: "Abwesend"}
	}

	if !hasFullAccess {
		return StudentLocationInfo{Location: "Anwesend"}
	}

	visit, ok := s.Visits[studentID]
	if !ok || visit == nil || visit.ActiveGroupID <= 0 {
		return StudentLocationInfo{Location: "Unterwegs"}
	}

	group, ok := s.Groups[visit.ActiveGroupID]
	if !ok || group == nil {
		return StudentLocationInfo{Location: "Unterwegs"}
	}

	if group.Room != nil && group.Room.Name != "" {
		return StudentLocationInfo{
			Location: fmt.Sprintf("Anwesend - %s", group.Room.Name),
			Since:    &visit.EntryTime,
		}
	}

	return StudentLocationInfo{Location: "Unterwegs"}
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
