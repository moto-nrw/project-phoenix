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
	base.TenantModel `bun:"schema:users,table:students"`
	PersonID         int64      `bun:"person_id,notnull" json:"person_id"`
	SchoolClass      string     `bun:"school_class,notnull" json:"school_class"`
	GuardianName     *string    `bun:"guardian_name" json:"guardian_name,omitempty"`       // Optional: Legacy field, use guardian_profiles instead
	GuardianContact  *string    `bun:"guardian_contact" json:"guardian_contact,omitempty"` // Optional: Legacy field, use guardian_profiles instead
	GuardianEmail    *string    `bun:"guardian_email" json:"guardian_email,omitempty"`
	GuardianPhone    *string    `bun:"guardian_phone" json:"guardian_phone,omitempty"`
	GroupID          *int64     `bun:"group_id" json:"group_id,omitempty"`
	ExtraInfo        *string    `bun:"extra_info" json:"extra_info,omitempty"`
	SupervisorNotes  *string    `bun:"supervisor_notes" json:"supervisor_notes,omitempty"`
	HealthInfo       *string    `bun:"health_info" json:"health_info,omitempty"`
	PickupStatus     *string    `bun:"pickup_status" json:"pickup_status,omitempty"`
	Bus              *bool      `bun:"bus" json:"bus,omitempty"`               // Administrative permission flag (Buskind)
	Sick             *bool      `bun:"sick" json:"sick,omitempty"`             // true = currently sick
	SickSince        *time.Time `bun:"sick_since" json:"sick_since,omitempty"` // When sickness was reported

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

	s.SchoolClass = strings.TrimSpace(s.SchoolClass)

	// Normalize optional legacy guardian fields
	trimPtrString(s.GuardianName)
	trimPtrStringOrNil(&s.GuardianContact)

	// Validate optional contact fields
	if err := validatePtrEmail(s.GuardianEmail, "guardian email"); err != nil {
		return err
	}
	if err := validatePtrPhone(s.GuardianPhone, "guardian phone"); err != nil {
		return err
	}

	return nil
}

// trimPtrString trims whitespace from a non-nil string pointer
func trimPtrString(s *string) {
	if s != nil && *s != "" {
		*s = strings.TrimSpace(*s)
	}
}

// trimPtrStringOrNil trims whitespace and sets to nil if empty
func trimPtrStringOrNil(sp **string) {
	if *sp == nil || **sp == "" {
		return
	}
	trimmed := strings.TrimSpace(**sp)
	if trimmed == "" {
		*sp = nil
	} else {
		**sp = trimmed
	}
}

// validatePtrEmail validates an optional email pointer
func validatePtrEmail(email *string, fieldName string) error {
	if email == nil || *email == "" {
		return nil
	}
	*email = strings.TrimSpace(*email)
	emailPattern := regexp.MustCompile(`^[A-Za-z0-9._%-]+@[A-Za-z0-9.-]+[.][A-Za-z]+$`)
	if !emailPattern.MatchString(*email) {
		return errors.New("invalid " + fieldName + " format")
	}
	return nil
}

// validatePtrPhone validates an optional phone pointer
func validatePtrPhone(phone *string, fieldName string) error {
	if phone == nil || *phone == "" {
		return nil
	}
	*phone = strings.TrimSpace(*phone)
	phonePattern := regexp.MustCompile(`^(\+[0-9]{1,3}\s?)?[0-9\s-]{7,15}$`)
	if !phonePattern.MatchString(*phone) {
		return errors.New("invalid " + fieldName + " format")
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
