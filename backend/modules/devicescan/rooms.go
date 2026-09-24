package devicescan

import "context"

type RoomAvailability interface {
	AvailableRooms(context.Context, int) ([]AvailableRoom, error)
}

// AvailableRoom is the room-selection wire, including occupancy. Minimum
// capacity filtering is applied by the Facilities query before projection.
type AvailableRoom struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	Building   string  `json:"building,omitempty"`
	Floor      *int    `json:"floor,omitempty"`
	Capacity   *int    `json:"capacity,omitempty"`
	Category   *string `json:"category,omitempty"`
	Color      *string `json:"color,omitempty"`
	IsOccupied bool    `json:"is_occupied"`
	// IsOpenRoom marks a room the school released as a shared destination
	// (#3067): a child can choose it at the device it leaves, and the stay is
	// booked without a device or supervision at the destination.
	IsOpenRoom bool `json:"is_open_room"`
	// IsSchulhof marks the canonical Schulhof, so the kiosk finds it by this
	// flag instead of by its name. Its own scan flow keeps its block rules.
	IsSchulhof bool `json:"is_schulhof"`
}
