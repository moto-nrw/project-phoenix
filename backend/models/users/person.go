package users

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// Person represents a physical person in the system
type Person struct {
	base.Model `bun:"schema:users,table:persons"`
	base.TenantModel
	FirstName string         `bun:"first_name,notnull" json:"first_name"`
	LastName  string         `bun:"last_name,notnull" json:"last_name"`
	Birthday  *timezone.Date `bun:"birthday,type:date" json:"birthday,omitempty"`
	TagID     *string        `bun:"tag_id" json:"tag_id,omitempty"`
	AccountID *int64         `bun:"account_id" json:"account_id,omitempty"`
	DeletedAt *time.Time     `bun:"deleted_at,soft_delete,nullzero" json:"-"`

	// Relations not stored in the database
	Account  *PersonAccount  `bun:"-" json:"account,omitempty"`
	RFIDCard *PersonRFIDCard `bun:"-" json:"rfid_card,omitempty"`
}

// Validate ensures person data is valid
// ErrPersonRowMissing says a person the caller named has no row. It lives here
// because the two sides that need it — the People Directory composition seam
// that observes the owner's not-found, and the retained person service that
// maps it onto its own sentinel — may not import each other (#3349).
var ErrPersonRowMissing = errors.New("person row not found")

func (p *Person) Validate() error {
	if p.FirstName == "" {
		return errors.New("first name is required")
	}

	if p.LastName == "" {
		return errors.New("last name is required")
	}

	// Trim spaces from names
	p.FirstName = strings.TrimSpace(p.FirstName)
	p.LastName = strings.TrimSpace(p.LastName)

	// Note: Removed the requirement for TagID or AccountID
	// Students can be created without either identifier
	// The check is kept in the database constraint but made optional in the model

	return nil
}

// GetFullName returns the complete name of the person
func (p *Person) GetFullName() string {
	return p.FirstName + " " + p.LastName
}

// SetAccount links this person to an account
func (p *Person) SetAccount(account *PersonAccount) {
	p.Account = account
	if account != nil {
		p.AccountID = &account.ID
	} else {
		p.AccountID = nil
	}
}

// SetRFIDCard links this person to an RFID card
func (p *Person) SetRFIDCard(card *PersonRFIDCard) {
	p.RFIDCard = card
	if card != nil {
		p.TagID = &card.ID
	} else {
		p.TagID = nil
	}
}

// HasRFIDCard checks if the person has an RFID card assigned
func (p *Person) HasRFIDCard() bool {
	return p.TagID != nil && *p.TagID != ""
}

// HasAccount checks if the person has an account assigned
func (p *Person) HasAccount() bool {
	return p.AccountID != nil && *p.AccountID > 0
}
