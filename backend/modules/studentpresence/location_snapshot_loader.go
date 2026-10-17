package studentpresence

import "context"

// StudentLocationReader is the presence surface the shared location snapshot
// reads (#3352): the tenant's presence mode, today's attendance rows, the open
// visits and the sessions those visits belong to. Inbound adapters bind it
// from the public Student Presence contract instead of the whole capability.
type StudentLocationReader interface {
	PresenceModes
	GetStudentsAttendanceStatuses(ctx context.Context, studentIDs []int64) (map[int64]*DailyAttendanceStatus, error)
	GetStudentsCurrentVisits(ctx context.Context, studentIDs []int64) (map[int64]*Visit, error)
	GetActiveGroupsByIDs(ctx context.Context, groupIDs []int64) (map[int64]*SessionDetail, error)
}

// The public capability satisfies the port; a contract change surfaces here.
var _ StudentLocationReader = Presence(nil)

// LoadStudentLocationSnapshot batches all data needed to resolve student locations.
// In binary-mode tenants it skips the visit/group queries as a perf win — those
// fields become irrelevant because the resolver won't read them anyway.
func LoadStudentLocationSnapshot(ctx context.Context, svc StudentLocationReader, studentIDs []int64) (*StudentLocationSnapshot, error) {
	uniqueIDs := uniqueStudentLocationIDs(studentIDs)
	mode, err := svc.GetPresenceMode(ctx)
	if err != nil {
		return nil, err
	}
	snapshot := NewStudentLocationSnapshot(mode)

	if len(uniqueIDs) == 0 {
		return snapshot, nil
	}

	// Load attendances (needed in both modes — it's the universal source of truth).
	attendances, err := svc.GetStudentsAttendanceStatuses(ctx, uniqueIDs)
	if err != nil {
		return nil, err
	}
	snapshot.Attendances = coalesce(attendances, snapshot.Attendances)

	// Binary mode ignores room/group data entirely. One extra lookup though:
	// the yard state has no visit behind it, so the Schulhof room's colour has
	// to come from the room itself for the badge to follow the school's colour
	// scheme (#2405).
	if mode == PresenceModeBinary {
		snapshot.YardRoomColor = ResolveYardRoomColor(ctx, svc)
		return snapshot, nil
	}

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
	snapshot.Visits = coalesce(visits, snapshot.Visits)

	// Load groups for active visits
	groupIDs := snapshot.ActiveGroupIDs()
	if len(groupIDs) == 0 {
		return snapshot, nil
	}

	groups, err := svc.GetActiveGroupsByIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	snapshot.Groups = coalesce(groups, snapshot.Groups)

	return snapshot, nil
}

// filterCheckedInStudents returns IDs of students with checked_in status
func filterCheckedInStudents(attendances map[int64]*DailyAttendanceStatus) []int64 {
	result := make([]int64, 0, len(attendances))
	for studentID, status := range attendances {
		if status != nil && status.Status == "checked_in" {
			result = append(result, studentID)
		}
	}
	return result
}

// coalesce returns m if non-nil, otherwise fallback.
func coalesce[K comparable, V any](m, fallback map[K]V) map[K]V {
	if m != nil {
		return m
	}
	return fallback
}

// uniqueStudentLocationIDs preserves the first occurrence of each requested student.
func uniqueStudentLocationIDs(studentIDs []int64) []int64 {
	uniqueIDs := make([]int64, 0, len(studentIDs))
	seen := make(map[int64]struct{}, len(studentIDs))
	for _, id := range studentIDs {
		if _, found := seen[id]; !found {
			seen[id] = struct{}{}
			uniqueIDs = append(uniqueIDs, id)
		}
	}
	return uniqueIDs
}
