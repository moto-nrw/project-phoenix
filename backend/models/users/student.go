package users

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Student represents a student in the system
type Student struct {
	base.Model      `bun:"schema:users,table:students"`
	PersonID        int64   `bun:"person_id,notnull" json:"person_id"`
	SchoolClass     string  `bun:"school_class,notnull" json:"school_class"`
	GuardianName    *string `bun:"guardian_name" json:"guardian_name,omitempty"`          // Optional: Legacy field, use guardian_profiles instead
	GuardianContact *string `bun:"guardian_contact" json:"guardian_contact,omitempty"`    // Optional: Legacy field, use guardian_profiles instead
	GuardianEmail   *string `bun:"guardian_email" json:"guardian_email,omitempty"`
	GuardianPhone   *string `bun:"guardian_phone" json:"guardian_phone,omitempty"`
	GroupID         *int64  `bun:"group_id" json:"group_id,omitempty"`
	ExtraInfo       *string `bun:"extra_info" json:"extra_info,omitempty"`
	SupervisorNotes *string `bun:"supervisor_notes" json:"supervisor_notes,omitempty"`
	HealthInfo      *string `bun:"health_info" json:"health_info,omitempty"`

	// Relations
	Person *Person `bun:"rel:belongs-to,join:person_id=id" json:"person,omitempty"`
	// Group relation is loaded dynamically to avoid import cycle
}

// BeforeAppendModel sets the correct table expression
// Note: Table aliases (AS "student") are only applied for SELECT, UPDATE, and DELETE queries.
//
//	For INSERT queries, aliases should NOT be used, as they can cause issues with some database drivers.
func (s *Student) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(`users.students AS "student"`)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`users.students AS "student"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`users.students AS "student"`)
	}
	return nil
}

// TableName returns the database table name
func (s *Student) TableName() string {
	return "users.students"
}

// Validate ensures student data is valid
func (s *Student) Validate() error {
	if s.PersonID <= 0 {
		return errors.New("person ID is required")
	}

	if s.SchoolClass == "" {
		return errors.New("school class is required")
	}

	// Trim spaces from school class
	s.SchoolClass = strings.TrimSpace(s.SchoolClass)

	// Guardian fields are now optional (legacy fields, use guardian_profiles instead)
	// Trim spaces from guardian name if provided
	if s.GuardianName != nil && *s.GuardianName != "" {
		trimmed := strings.TrimSpace(*s.GuardianName)
		s.GuardianName = &trimmed
	}

	// Trim spaces from guardian contact if provided
	if s.GuardianContact != nil && *s.GuardianContact != "" {
		trimmed := strings.TrimSpace(*s.GuardianContact)
		if trimmed == "" {
			s.GuardianContact = nil
		} else {
			s.GuardianContact = &trimmed
		}
	}

	// Validate guardian email if provided
	if s.GuardianEmail != nil && *s.GuardianEmail != "" {
		*s.GuardianEmail = strings.TrimSpace(*s.GuardianEmail)
		emailPattern := regexp.MustCompile(`^[A-Za-z0-9._%-]+@[A-Za-z0-9.-]+[.][A-Za-z]+$`)
		if !emailPattern.MatchString(*s.GuardianEmail) {
			return errors.New("invalid guardian email format")
		}
	}

	// Validate guardian phone if provided
	if s.GuardianPhone != nil && *s.GuardianPhone != "" {
		*s.GuardianPhone = strings.TrimSpace(*s.GuardianPhone)
		phonePattern := regexp.MustCompile(`^(\+[0-9]{1,3}\s?)?[0-9\s-]{7,15}$`)
		if !phonePattern.MatchString(*s.GuardianPhone) {
			return errors.New("invalid guardian phone format")
		}
	}

	return nil
}

// SetPerson links this student to a person
func (s *Student) SetPerson(person *Person) {
	s.Person = person
	if person != nil {
		s.PersonID = person.ID
	}
}

// GetID returns the entity's ID
func (m *Student) GetID() interface{} {
	return m.ID
}

// GetCreatedAt returns the creation timestamp
func (m *Student) GetCreatedAt() time.Time {
	return m.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (m *Student) GetUpdatedAt() time.Time {
	return m.UpdatedAt
}

// StudentWithGroupInfo represents a student with their group information
type StudentWithGroupInfo struct {
	*Student
	GroupName string `json:"group_name"`
}
