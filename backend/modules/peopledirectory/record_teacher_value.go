package peopledirectory

import (
	"time"
)

type Teacher struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	TenantID       int64      `json:"tenant_id"`
	StaffID        int64      `json:"staff_id"`
	Specialization string     `json:"specialization,omitempty"`
	Role           string     `json:"role,omitempty"`
	Qualifications string     `json:"qualifications,omitempty"`
	DeletedAt      *time.Time `json:"-"`

	// Relations not stored in the database
	Staff *Staff `json:"staff,omitempty"`
}
