package users

import (
	"time"
)

// Teacher represents a pedagogical specialist in the system
type Teacher struct {
	Model `bun:"schema:users,table:teachers"`
	TenantModel
	StaffID        int64      `bun:"staff_id,notnull" json:"staff_id"`
	Specialization string     `bun:"specialization,nullzero" json:"specialization,omitempty"`
	Role           string     `bun:"role" json:"role,omitempty"`
	Qualifications string     `bun:"qualifications" json:"qualifications,omitempty"`
	DeletedAt      *time.Time `bun:"deleted_at,soft_delete,nullzero" json:"-"`

	// Relations not stored in the database
	Staff *Staff `bun:"-" json:"staff,omitempty"`
	// Groups will be managed through the education.GroupTeacher model
}
