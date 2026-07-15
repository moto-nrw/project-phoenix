package users

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Teacher represents a pedagogical specialist in the system
type Teacher struct {
	base.Model `bun:"schema:users,table:teachers"`
	base.TenantModel
	StaffID        int64      `bun:"staff_id,notnull" json:"staff_id"`
	Specialization string     `bun:"specialization,nullzero" json:"specialization,omitempty"`
	Role           string     `bun:"role" json:"role,omitempty"`
	Qualifications string     `bun:"qualifications" json:"qualifications,omitempty"`
	DeletedAt      *time.Time `bun:"deleted_at,soft_delete,nullzero" json:"-"`

	// Relations not stored in the database
	Staff *Staff `bun:"-" json:"staff,omitempty"`
	// Groups will be managed through the education.GroupTeacher model
}

// Validate ensures teacher data is valid
func (t *Teacher) Validate() error {
	if t.StaffID <= 0 {
		return errors.New("staff ID is required")
	}

	// Normalize specialization whitespace; empty string will be stored as NULL via nullzero tag
	t.Specialization = strings.TrimSpace(t.Specialization)

	// Trim spaces from role if provided
	if t.Role != "" {
		t.Role = strings.TrimSpace(t.Role)
	}

	return nil
}

// SetStaff links this teacher to a staff member
func (t *Teacher) SetStaff(staff *Staff) {
	t.Staff = staff
	if staff != nil {
		t.StaffID = staff.ID
	}
}

// GetFullName returns the full name of the teacher from the linked staff and person
func (t *Teacher) GetFullName() string {
	if t.Staff != nil && t.Staff.Person != nil {
		return t.Staff.Person.GetFullName()
	}
	return ""
}

// GetTitle returns the teacher's title based on role and specialization
func (t *Teacher) GetTitle() string {
	if t.Role != "" {
		return t.Role
	}
	return t.Specialization
}

// HasQualifications checks if the teacher has specified qualifications
func (t *Teacher) HasQualifications() bool {
	return t.Qualifications != ""
}
