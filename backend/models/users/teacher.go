package users

import (
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Teacher represents a pedagogical specialist in the system
type Teacher struct {
	base.Model
	StaffID        int64  `bun:"staff_id,notnull,unique" json:"staff_id"`
	Specialization string `bun:"specialization,notnull" json:"specialization"`
	Role           string `bun:"role" json:"role,omitempty"`
	Qualifications string `bun:"qualifications" json:"qualifications,omitempty"`

	// Relations not stored in the database
	Staff *Staff `bun:"-" json:"staff,omitempty"`
	// Groups will be managed through the education.GroupTeacher model
}

// TableName returns the database table name
func (t *Teacher) TableName() string {
	return "users.teachers"
}

// Validate ensures teacher data is valid
func (t *Teacher) Validate() error {
	if t.StaffID <= 0 {
		return errors.New("staff ID is required")
	}

	if t.Specialization == "" {
		return errors.New("specialization is required")
	}

	// Trim spaces from specialization
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
