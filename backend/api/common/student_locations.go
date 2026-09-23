package common

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/sliceutil"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// The snapshot type and its location resolver live in modules/studentpresence, where
// the underlying attendance/visit/group data originates. These aliases keep
// the historical api/common call sites working unchanged; the batch loader
// below stays here as the shared api-layer read helper.

// StudentLocationInfo contains resolved location data including timestamps.
type StudentLocationInfo = studentpresence.StudentLocationInfo

// StudentLocationSnapshot caches attendance, visit, and group data for a set of students.
type StudentLocationSnapshot = studentpresence.StudentLocationSnapshot

// Presence mode labels for StudentLocationSnapshot.Mode.
const (
	PresenceModeDetailed = studentpresence.PresenceModeDetailed
	PresenceModeBinary   = studentpresence.PresenceModeBinary
)

// ResolveBinaryLocation maps an attendance row's derived status to a simple
// label for binary-mode tenants.
var ResolveBinaryLocation = studentpresence.ResolveBinaryLocation

// ResolveYardRoomColor returns the tenant's Schulhof room color, nil when none
// is configured. Only binary mode needs it — see the doc comment on the
// modules/studentpresence original.
var ResolveYardRoomColor = studentpresence.ResolveYardRoomColor

// YardLocationLabel is the binary-mode label for a student on the schoolyard.
const YardLocationLabel = studentpresence.YardLocationLabel

// AbsentLocationLabel is the resolved location of a child without an open check-in.
const AbsentLocationLabel = studentpresence.AbsentLocationLabel

// AtSchoolLocationLabel is the location of an expected child before the first check-in (#3260).
const AtSchoolLocationLabel = studentpresence.AtSchoolLocationLabel

// StudentLocationReader is the presence surface the shared location snapshot
// reads (#3352): the tenant's presence mode, today's attendance rows, the open
// visits and the sessions those visits belong to. Inbound adapters bind it
// from the public Student Presence contract instead of the whole capability.
type StudentLocationReader interface {
	studentpresence.PresenceModes
	GetStudentsAttendanceStatuses(ctx context.Context, studentIDs []int64) (map[int64]*studentpresence.DailyAttendanceStatus, error)
	GetStudentsCurrentVisits(ctx context.Context, studentIDs []int64) (map[int64]*studentpresence.Visit, error)
	GetActiveGroupsByIDs(ctx context.Context, groupIDs []int64) (map[int64]*studentpresence.SessionDetail, error)
}

// The public capability satisfies the port; a contract change surfaces here.
var _ StudentLocationReader = studentpresence.Presence(nil)

// LoadStudentLocationSnapshot batches all data needed to resolve student locations.
// In binary-mode tenants it skips the visit/group queries as a perf win — those
// fields become irrelevant because the resolver won't read them anyway.
func LoadStudentLocationSnapshot(ctx context.Context, svc StudentLocationReader, studentIDs []int64) (*StudentLocationSnapshot, error) {
	uniqueIDs := sliceutil.Unique(studentIDs)
	mode, err := svc.GetPresenceMode(ctx)
	if err != nil {
		return nil, err
	}
	snapshot := studentpresence.NewStudentLocationSnapshot(mode)

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
		snapshot.YardRoomColor = studentpresence.ResolveYardRoomColor(ctx, svc)
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
func filterCheckedInStudents(attendances map[int64]*studentpresence.DailyAttendanceStatus) []int64 {
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
