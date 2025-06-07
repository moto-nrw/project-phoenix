package users

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
	"golang.org/x/crypto/bcrypt"
)

// Staff represents a staff member in the system
type Staff struct {
	base.Model     `bun:"schema:users,table:staff"`
	PersonID       int64      `bun:"person_id,notnull,unique" json:"person_id"`
	StaffNotes     string     `bun:"staff_notes" json:"staff_notes,omitempty"`
	PINHash        *string    `bun:"pin_hash" json:"-"`                           // Never expose PIN hash in JSON
	PINAttempts    int        `bun:"pin_attempts,default:0" json:"pin_attempts"`
	PINLockedUntil *time.Time `bun:"pin_locked_until" json:"pin_locked_until,omitempty"`

	// Relations
	Person *Person `bun:"rel:belongs-to,join:person_id=id" json:"person,omitempty"`
}

func (s *Staff) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(`users.staff AS "staff"`)
	}
	if q, ok := query.(*bun.InsertQuery); ok {
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

// HasPIN returns true if the staff member has a PIN set
func (s *Staff) HasPIN() bool {
	return s.PINHash != nil && *s.PINHash != ""
}

// IsLocked returns true if the account is currently locked due to failed PIN attempts
func (s *Staff) IsLocked() bool {
	if s.PINLockedUntil == nil {
		return false
	}
	return time.Now().Before(*s.PINLockedUntil)
}

// LockAccount locks the account for the specified duration
func (s *Staff) LockAccount(duration time.Duration) {
	lockUntil := time.Now().Add(duration)
	s.PINLockedUntil = &lockUntil
}

// UnlockAccount removes the account lock
func (s *Staff) UnlockAccount() {
	s.PINLockedUntil = nil
	s.PINAttempts = 0
}

// IncrementPINAttempts increments the failed PIN attempts counter
func (s *Staff) IncrementPINAttempts() {
	s.PINAttempts++
}

// HashPIN hashes a PIN using bcrypt
func HashPIN(pin string) (string, error) {
	hashedPIN, err := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPIN), nil
}

// VerifyPIN verifies a PIN against a hash
func VerifyPIN(pin, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(pin))
	return err == nil
}
