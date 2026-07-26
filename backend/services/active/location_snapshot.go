package active

import (
	"fmt"
	"time"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
)

// StudentLocationInfo contains resolved location data including timestamps.
//
// RoomColor carries the room's configured hex code (e.g. "#A3D977") when the
// student is currently checked into a real room and that room has a color
// override set. Frontend uses it to differentiate room badges instead of
// rendering every "in some room" student blue. Nil for non-room statuses
// (Schulhof / Unterwegs / Zuhause) and for rooms without a custom color.
type StudentLocationInfo struct {
	Location  string
	Since     *time.Time // When the student entered this location (nil if not in a room)
	RoomColor *string    // Hex color of the current room, nil when no room or no override set
}

// Mode constants for StudentLocationSnapshot.Mode.
const (
	PresenceModeDetailed = "detailed"
	PresenceModeBinary   = "binary"
)

// StudentLocationSnapshot caches attendance, visit, and group data for a set of students.
// Callers can reuse the snapshot to resolve location strings without triggering N+1 queries.
//
// Mode is the tenant's resolved presence mode. In "binary" mode the snapshot
// loader skips visit and group queries entirely — those maps will be empty —
// and the resolver returns simple Anwesend/Abwesend/Schulhof labels driven
// solely by the attendance row.
type StudentLocationSnapshot struct {
	Mode        string
	Attendances map[int64]*AttendanceStatus
	Visits      map[int64]*activeModels.Visit
	Groups      map[int64]*activeModels.Group
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

	// Binary mode short-circuits after the attendance check. Visit and group
	// data aren't written by binary-mode tenants, so there's nothing else to
	// consult. Yard state is surfaced as "Schulhof" — the only non-binary
	// label the resolver produces in this mode, and only when a yard timestamp
	// is actually set on the attendance row.
	if s.Mode == PresenceModeBinary {
		return ResolveBinaryLocation(status, hasFullAccess)
	}

	// If checked out, return "Abwesend" with checkout time (for hasFullAccess users)
	if status.Status == "checked_out" {
		if hasFullAccess && status.CheckOutTime != nil {
			return StudentLocationInfo{Location: "Abwesend", Since: status.CheckOutTime}
		}
		return StudentLocationInfo{Location: "Abwesend"}
	}

	// If not checked in at all, return "Abwesend" without time.
	// "on_yard" does not appear in detailed mode (yard state is binary-mode
	// only), so we treat anything other than "checked_in" as absent here.
	if status.Status != "checked_in" {
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
			Location:  fmt.Sprintf("Anwesend - %s", group.Room.Name),
			Since:     &visit.EntryTime,
			RoomColor: group.Room.Color,
		}
	}

	return StudentLocationInfo{Location: "Unterwegs"}
}

// ResolveBinaryLocation maps an attendance row's derived status to a simple
// label for binary-mode tenants. The Since field carries the most relevant
// timestamp for the current state (check-in, yard transition, or check-out),
// gated on hasFullAccess for parity with detailed-mode privacy semantics.
//
// Exported so the detail-endpoint helper (api/students/response_helpers.go)
// can short-circuit identically — the per-student path used to fall through
// to "Unterwegs" for binary-mode tenants.
func ResolveBinaryLocation(status *AttendanceStatus, hasFullAccess bool) StudentLocationInfo {
	switch status.Status {
	case "on_yard":
		info := StudentLocationInfo{Location: "Schulhof"}
		if hasFullAccess {
			info.Since = status.YardSince
		}
		return info
	case "checked_in":
		info := StudentLocationInfo{Location: "Anwesend"}
		if hasFullAccess {
			info.Since = status.CheckInTime
		}
		return info
	case "checked_out":
		info := StudentLocationInfo{Location: "Abwesend"}
		if hasFullAccess {
			info.Since = status.CheckOutTime
		}
		return info
	default:
		return StudentLocationInfo{Location: "Abwesend"}
	}
}
