package users

import "time"

// PersonRFIDCard preserves card metadata in person responses without an ORM
// relationship to the card owner's table.
type PersonRFIDCard struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	TenantID  int64     `json:"tenant_id"`
	Active    bool      `json:"active"`
}
