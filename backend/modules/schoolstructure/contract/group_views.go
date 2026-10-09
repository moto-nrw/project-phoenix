package contract

import "time"

// GroupRoom is the Facilities room a group is assigned to, as the
// room-enriched group reads carry it (#2665). facilities.rooms belongs to
// that owner; School Structure only keeps this projection.
type GroupRoom struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Name      string    `json:"name"`
	Building  string    `json:"building,omitempty"`
	Floor     *int      `json:"floor,omitempty"`
	Capacity  *int      `json:"capacity,omitempty"`
	Category  *string   `json:"category,omitempty"`
	Color     *string   `json:"color,omitempty"`
}

// GroupListQuery is the bounded read shape for the group overview. It exposes
// only the filters and ordering that overview callers use.
type GroupListQuery struct {
	Name         string
	NameContains string
	RoomID       *int64
	Limit        int
	Offset       int
	SortByName   bool
	Descending   bool
}
