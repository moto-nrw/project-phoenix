package users

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
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
	GuardianName    string  `bun:"guardian_name,notnull" json:"guardian_name"`
	GuardianContact string  `bun:"guardian_contact,notnull" json:"guardian_contact"`
	GuardianEmail   *string `bun:"guardian_email" json:"guardian_email,omitempty"`
	GuardianPhone   *string `bun:"guardian_phone" json:"guardian_phone,omitempty"`
	GroupID         *int64  `bun:"group_id" json:"group_id,omitempty"`

	// Relations not stored in the database
	Person *Person `bun:"-" json:"person,omitempty"`
	// Group relation is loaded dynamically to avoid import cycle
}

// BeforeAppendModel is removed to avoid conflicts with repository ModelTableExpr

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

	if s.GuardianName == "" {
		return errors.New("guardian name is required")
	}

	// Trim spaces from guardian name
	s.GuardianName = strings.TrimSpace(s.GuardianName)

	if s.GuardianContact == "" {
		return errors.New("guardian contact is required")
	}

	// Trim spaces from guardian contact
	s.GuardianContact = strings.TrimSpace(s.GuardianContact)

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

	// Ensure only one loc ation is active at a time
	locationCount := 0
	if s.Bus {
		locationCount++
	}
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
	if s.Bus {
		return "Bus"
	}
	if s.InHouse {
		return "In House"
	}
	if s.WC {
		return "WC"
	}
	if s.SchoolYard {
		return "School Yard"
	}
	return "Unknown"
}

// SetLocation sets the student's location, ensuring only one is active
func (s *Student) SetLocation(location string) error {
	// Reset all locations
	s.Bus = false
	s.InHouse = false
	s.WC = false
	s.SchoolYard = false

	// Set the specified location
	switch strings.ToLower(location) {
	case "bus":
		s.Bus = true
	case "in house", "house":
		s.InHouse = true
	case "wc", "bathroom":
		s.WC = true
	case "school yard", "yard":
		s.SchoolYard = true
	case "unknown", "none", "":
		// All locations remain false
	default:
		return errors.New("invalid location: must be Bus, In House, WC, School Yard, or empty")
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
