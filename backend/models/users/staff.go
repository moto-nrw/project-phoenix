package users

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Staff represents a staff member in the system
type Staff struct {
	base.Model
	PersonID   int64  `bun:"person_id,notnull,unique" json:"person_id"`
	StaffNotes string `bun:"staff_notes" json:"staff_notes,omitempty"`

	// Relations not stored in the database
	Person *Person `bun:"-" json:"person,omitempty"`
}

// TableName returns the database table name
func (s *Staff) TableName() string {
	return "users.staff"
}

// Validate ensures staff data is valid
func (s *Staff) Validate() error {
	if s.PersonID <= 0 {
		return errors.New("person ID is required")
	}

	return nil
}

// SetPerson links this staff member to a person
func (s *Staff) SetPerson(person *Person) {
	s.Person = person
	if person != nil {
		s.PersonID = person.ID
	}
}

// GetFullName returns the full name of the staff member from the linked person
func (s *Staff) GetFullName() string {
	if s.Person != nil {
		return s.Person.GetFullName()
	}
	return ""
}

// AddNotes adds notes about this staff member
func (s *Staff) AddNotes(notes string) {
	if s.StaffNotes == "" {
		s.StaffNotes = notes
	} else {
		s.StaffNotes += "\n" + notes
	}
}
