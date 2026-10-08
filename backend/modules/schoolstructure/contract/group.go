package contract

import "time"

// Group is the structure view of one education group (Klasse/Gruppe).
type Group struct {
	ID        int64      `json:"id"`
	TenantID  int64      `json:"tenant_id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Name      string     `json:"name"`
	RoomID    *int64     `json:"room_id,omitempty"`
	Room      *GroupRoom `json:"room,omitempty"`
}
