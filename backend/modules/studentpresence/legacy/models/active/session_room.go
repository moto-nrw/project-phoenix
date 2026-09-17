package active

import "time"

// SessionRoom is room information projected onto a presence session.
// Facilities owns the writable room record.
type SessionRoom struct {
	ID         int64     `json:"id"`
	TenantID   int64     `json:"tenant_id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	Name       string    `json:"name"`
	Building   string    `json:"building,omitempty"`
	Floor      *int      `json:"floor,omitempty"`
	Capacity   *int      `json:"capacity,omitempty"`
	Category   *string   `json:"category,omitempty"`
	Color      *string   `json:"color,omitempty"`
	IsSystem   bool      `json:"is_system"`
	IsOpenRoom bool      `json:"is_open_room"`
}

func (r *SessionRoom) SetTenantID(id int64) { r.TenantID = id }
func (r *SessionRoom) GetTenantID() int64   { return r.TenantID }
