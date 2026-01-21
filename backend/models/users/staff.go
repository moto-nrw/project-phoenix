package users

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Staff represents a staff member in the system
type Staff struct {
	base.TenantModel `bun:"schema:users,table:staff"`
	PersonID         int64   `bun:"person_id,notnull,unique" json:"person_id"`
	StaffNotes       string  `bun:"staff_notes" json:"staff_notes,omitempty"`
	BetterAuthUserID *string `bun:"betterauth_user_id" json:"betterauth_user_id,omitempty"`

	// Relations
	Person *Person `bun:"rel:belongs-to,join:person_id=id" json:"person,omitempty"`
}

func (s *Staff) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(`users.staff AS "staff"`)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`users.staff AS "staff"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`users.staff AS "staff"`)
	}
	return nil
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

// GetID returns the entity's ID
func (m *Staff) GetID() interface{} {
	return m.ID
}

// GetCreatedAt returns the creation timestamp
func (m *Staff) GetCreatedAt() time.Time {
	return m.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (m *Staff) GetUpdatedAt() time.Time {
	return m.UpdatedAt
}

// PIN-related functionality has been moved to auth.Account model
// This simplifies the architecture by centralizing all authentication data
