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

// YardLocationLabel is the binary-mode label for a student on the schoolyard.
// Named so the yard-color stamping below matches on a constant instead of a
// second copy of the string literal.
const YardLocationLabel = "Schulhof"

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

	// YardRoomColor is the tenant's configured Schulhof room color, used to
	// tint the binary-mode "Schulhof" state (#2405). Only binary mode needs
	// it: in detailed mode the yard is an ordinary room visit whose color
	// already travels with the active group's room. Nil means "no color
	// configured" — the frontend then renders the orange Schulhof default.
	YardRoomColor *string
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

	status := s.Attendances[studentID]
	if s.Mode == PresenceModeBinary {
		if status == nil {
			return StudentLocationInfo{Location: "Abwesend"}
		}
		info := ResolveBinaryLocation(status, hasFullAccess)
		return withYardRoomColor(info, s.YardRoomColor)
	}
	return s.resolveDetailedLocation(studentID, status, hasFullAccess)
}

// resolveDetailedLocation treats an open visit as the authoritative live
// location even when the attendance projection is missing or stale.
func (s *StudentLocationSnapshot) resolveDetailedLocation(
	studentID int64, status *AttendanceStatus, hasFullAccess bool,
) StudentLocationInfo {
	visit := s.Visits[studentID]
	if status == nil && visit == nil {
		return StudentLocationInfo{Location: "Abwesend"}
	}

	if visit == nil {
		return detailedAttendanceLocation(status, hasFullAccess)
	}
	if !hasFullAccess {
		return StudentLocationInfo{Location: "Anwesend"}
	}
	return s.resolveVisitLocation(visit)
}

func detailedAttendanceLocation(status *AttendanceStatus, hasFullAccess bool) StudentLocationInfo {
	if status.Status == "checked_out" {
		if hasFullAccess && status.CheckOutTime != nil {
			return StudentLocationInfo{Location: "Abwesend", Since: status.CheckOutTime}
		}
		return StudentLocationInfo{Location: "Abwesend"}
	}
	if status.Status != "checked_in" {
		return StudentLocationInfo{Location: "Abwesend"}
	}
	if !hasFullAccess {
		return StudentLocationInfo{Location: "Anwesend"}
	}
	return StudentLocationInfo{Location: "Unterwegs"}
}

func (s *StudentLocationSnapshot) resolveVisitLocation(visit *activeModels.Visit) StudentLocationInfo {
	if visit.ActiveGroupID <= 0 {
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
		info := StudentLocationInfo{Location: YardLocationLabel}
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

// withYardRoomColor stamps the Schulhof room color onto a binary-mode
// location, but only when the resolved label actually IS the yard. Every
// other binary label (Anwesend / Abwesend) has no room behind it, so
// attaching a room color there would tint an unrelated badge.
func withYardRoomColor(info StudentLocationInfo, yardColor *string) StudentLocationInfo {
	if yardColor == nil || info.Location != YardLocationLabel {
		return info
	}
	info.RoomColor = yardColor
	return info
}
