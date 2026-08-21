package common

import (
	"context"
	"maps"
	"slices"

	"github.com/moto-nrw/project-phoenix/internal/sliceutil"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
)

// The snapshot type and its location resolver live in services/active, where
// the underlying attendance/visit/group data originates. These aliases keep
// the historical api/common call sites working unchanged; the batch loader
// below stays here as the shared api-layer read helper.

// StudentLocationInfo contains resolved location data including timestamps.
type StudentLocationInfo = activeService.StudentLocationInfo

// StudentLocationSnapshot caches attendance, visit, and group data for a set of students.
type StudentLocationSnapshot = activeService.StudentLocationSnapshot

// Presence mode labels for StudentLocationSnapshot.Mode.
const (
	PresenceModeDetailed = activeService.PresenceModeDetailed
	PresenceModeBinary   = activeService.PresenceModeBinary
)

// ResolveBinaryLocation maps an attendance row's derived status to a simple
// label for binary-mode tenants.
var ResolveBinaryLocation = activeService.ResolveBinaryLocation

// ResolveYardRoomColor returns the tenant's Schulhof room color, nil when none
// is configured. Only binary mode needs it — see the doc comment on the
// services/active original.
var ResolveYardRoomColor = activeService.ResolveYardRoomColor

// YardLocationLabel is the binary-mode label for a student on the schoolyard.
const YardLocationLabel = activeService.YardLocationLabel

// LoadStudentLocationSnapshot batches all data needed to resolve student locations.
// In binary-mode tenants it skips the visit/group queries as a perf win — those
// fields become irrelevant because the resolver won't read them anyway.
func LoadStudentLocationSnapshot(ctx context.Context, svc activeService.Service, studentIDs []int64) (*StudentLocationSnapshot, error) {
	uniqueIDs := sliceutil.Unique(studentIDs)
	mode := svc.GetPresenceMode(ctx)
	snapshot := newEmptyLocationSnapshot(mode)

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
		snapshot.YardRoomColor = activeService.ResolveYardRoomColor(ctx, svc)
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
	groupIDs := extractActiveGroupIDs(snapshot.Visits)
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

// newEmptyLocationSnapshot creates a new snapshot with initialized empty maps.
// mode defaults to "detailed" when empty so old test fixtures (which construct
// a bare snapshot without setting Mode) keep the existing behavior.
func newEmptyLocationSnapshot(mode string) *StudentLocationSnapshot {
	if mode == "" {
		mode = PresenceModeDetailed
	}
	return &StudentLocationSnapshot{
		Mode:        mode,
		Attendances: make(map[int64]*activeService.AttendanceStatus),
		Visits:      make(map[int64]*activeModels.Visit),
		Groups:      make(map[int64]*activeModels.Group),
	}
}

// filterCheckedInStudents returns IDs of students with checked_in status
func filterCheckedInStudents(attendances map[int64]*activeService.AttendanceStatus) []int64 {
	result := make([]int64, 0, len(attendances))
	for studentID, status := range attendances {
		if status != nil && status.Status == "checked_in" {
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
	return slices.Collect(maps.Keys(groupIDSet))
}

// coalesce returns m if non-nil, otherwise fallback.
func coalesce[K comparable, V any](m, fallback map[K]V) map[K]V {
	if m != nil {
		return m
	}
	return fallback
}
