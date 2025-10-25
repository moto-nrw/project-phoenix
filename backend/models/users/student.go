package users

import (
	"errors"
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
	Bus             bool    `bun:"bus,notnull" json:"bus"`
	InHouse         bool    `bun:"in_house,notnull" json:"in_house"`
	WC              bool    `bun:"wc,notnull" json:"wc"`
	SchoolYard      bool    `bun:"school_yard,notnull" json:"school_yard"`
	GroupID         *int64  `bun:"group_id" json:"group_id,omitempty"`
	ExtraInfo       *string `bun:"extra_info" json:"extra_info,omitempty"`
	SupervisorNotes *string `bun:"supervisor_notes" json:"supervisor_notes,omitempty"`
	HealthInfo      *string `bun:"health_info" json:"health_info,omitempty"`

	// Relations
	Person    *Person              `bun:"rel:belongs-to,join:person_id=id" json:"person,omitempty"`
	Guardians []*StudentGuardian   `bun:"rel:has-many,join:id=student_id" json:"guardians,omitempty"`
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

	// Ensure only one location is active at a time (bus is not a location)
	locationCount := 0
	if s.InHouse {
		locationCount++
	}
	if s.WC {
		locationCount++
	}
	if s.SchoolYard {
		locationCount++
	}

	if locationCount > 1 {
		return errors.New("only one location can be active at a time")
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

// SetGroupID sets the group ID for this student
func (s *Student) SetGroupID(groupID *int64) {
	s.GroupID = groupID
}

// GetLocation returns the current location of the student
func (s *Student) GetLocation() string {
	if s.InHouse {
		return "In House"
	}
	if s.WC {
		return "WC"
	}
	if s.SchoolYard {
		return "School Yard"
	}
	// If none of the location flags are set, student is at home
	return "Home"
}

// SetLocation sets the student's location, ensuring only one is active
func (s *Student) SetLocation(location string) error {
	// Reset all location flags (but not bus, which is transportation info)
	s.InHouse = false
	s.WC = false
	s.SchoolYard = false

	// Set the specified location
	switch strings.ToLower(location) {
	case "in house", "house":
		s.InHouse = true
	case "wc", "bathroom":
		s.WC = true
	case "school yard", "yard":
		s.SchoolYard = true
	case "home", "none", "":
		// All locations remain false (student is at home)
	default:
		return errors.New("invalid location: must be In House, WC, School Yard, Home, or empty")
	}

	return nil
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
