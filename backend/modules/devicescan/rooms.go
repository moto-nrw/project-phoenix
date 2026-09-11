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
}
