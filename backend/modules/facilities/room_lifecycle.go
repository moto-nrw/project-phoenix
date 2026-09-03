package facilities

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	SchulhofRoomName = "Schulhof"
	WCRoomName       = "WC"
	WCRoomAliasName  = "Toilette"

	RoomColorUniqueConstraintName   = "uniq_facilities_rooms_tenant_color"
	RoomWCAliasUniqueConstraintName = "uniq_facilities_rooms_tenant_wc_alias"

	SchulhofActivityName        = "Schulhof Freispiel"
	SchulhofCategoryName        = "Schulhof"
	SchulhofCategoryDescription = "Outdoor playground activities"
	SchulhofColor               = "#7ED321"
	SchulhofRoomCapacity        = 300
	SchulhofMaxParticipants     = 300
	WCActivityName              = "WC"
	WCCategoryName              = "WC"
	WCCategoryDescription       = "Bathroom/toilet break"
	WCColor                     = "#60A5FA"
	WCRoomCapacity              = 20
	WCMaxParticipants           = 20
)

func IsWCRoomName(name string) bool { return name == WCRoomName || name == WCRoomAliasName }

func IsSystemRoomName(name string) bool { return name == SchulhofRoomName || IsWCRoomName(name) }

var roomColorPattern = regexp.MustCompile(`^#([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$`)

var reservedRoomColors = map[string]struct{}{
	"#83CD2D": {}, "#5080D8": {}, "#6B7280": {}, "#F78C10": {},
	"#D946EF": {}, "#78716C": {}, "#DC2626": {}, "#7C3AED": {},
	"#0891B2": {}, "#365D83": {}, "#FF3130": {}, "#EAB308": {},
	"#4F46E5": {},
}

func IsReservedRoomColor(color string) bool {
	normalized := strings.TrimSpace(color)
	if normalized == "" {
		return false
	}
	if !strings.HasPrefix(normalized, "#") {
		normalized = "#" + normalized
	}
	if len(normalized) == 4 {
		normalized = string([]byte{'#', normalized[1], normalized[1], normalized[2], normalized[2], normalized[3], normalized[3]})
	}
	_, found := reservedRoomColors[strings.ToUpper(normalized)]
	return found
}

// CreateRoom contains the caller-controlled fields for a new room. TenantID
// comes from the tenant runtime and is never accepted from the caller.
type CreateRoom struct {
	Name     string
	Building string
	Floor    *int
	Capacity *int
	Category *string
	Color    *string
	IsSystem bool
}

type UpdateRoom struct {
	ID       int64
	Name     string
	Building string
	Floor    *int
	Capacity *int
	Category *string
	Color    *string
}

// RoomFilter is the closed set of filters supported by the owner. Pointers
// distinguish an omitted filter from a zero-valued floor or capacity.
type RoomFilter struct {
	Name             *string
	NameContains     *string
	Building         *string
	BuildingContains *string
	Floor            *int
	Category         *string
	MinimumCapacity  *int
	MaximumCapacity  *int
	Search           *string
}

type InvalidRoomError struct{ Reason string }

func (e *InvalidRoomError) Error() string { return e.Reason }
func (e *InvalidRoomError) Unwrap() error { return ErrInvalidRoom }

func normalizeAndValidateCreateRoom(input *CreateRoom) error {
	return normalizeAndValidateRoom(&input.Name, input.Capacity, &input.Color)
}

func normalizeAndValidateRoom(name *string, capacity *int, color **string) error {
	*name = strings.TrimSpace(*name)
	if *name == "" {
		return &InvalidRoomError{Reason: "room name is required"}
	}
	if capacity != nil && *capacity <= 0 {
		return &InvalidRoomError{Reason: "capacity must be greater than zero"}
	}
	if *color == nil || strings.TrimSpace(**color) == "" {
		return nil
	}

	normalized := strings.TrimSpace(**color)
	if !strings.HasPrefix(normalized, "#") {
		normalized = "#" + normalized
	}
	if !roomColorPattern.MatchString(normalized) {
		return &InvalidRoomError{Reason: fmt.Sprintf("invalid room color %q", **color)}
	}
	if len(normalized) == 4 {
		normalized = string([]byte{'#', normalized[1], normalized[1], normalized[2], normalized[2], normalized[3], normalized[3]})
	}
	normalized = strings.ToUpper(normalized)
	if _, reserved := reservedRoomColors[normalized]; reserved {
		return ErrRoomColorReserved
	}
	*color = &normalized
	return nil
}
