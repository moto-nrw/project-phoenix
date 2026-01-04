package users

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const teacherTableName = "users.teachers"

// Teacher represents a pedagogical specialist in the system
type Teacher struct {
	base.Model     `bun:"schema:users,table:teachers"`
	StaffID        int64  `bun:"staff_id,notnull,unique" json:"staff_id"`
	Specialization string `bun:"specialization,nullzero" json:"specialization,omitempty"`
	Role           string `bun:"role" json:"role,omitempty"`
	Qualifications string `bun:"qualifications" json:"qualifications,omitempty"`

	// Relations not stored in the database
	Staff *Staff `bun:"-" json:"staff,omitempty"`
	// Groups will be managed through the education.GroupTeacher model
}

func (t *Teacher) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(teacherTableName)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(teacherTableName)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(teacherTableName)
	}
	return nil
}

// TableName returns the database table name
func (t *Teacher) TableName() string {
	return teacherTableName
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

// GetID returns the entity's ID
func (m *Teacher) GetID() interface{} {
	return m.ID
}

// GetCreatedAt returns the creation timestamp
func (m *Teacher) GetCreatedAt() time.Time {
	return m.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (m *Teacher) GetUpdatedAt() time.Time {
	return m.UpdatedAt
}
