package users

import (
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// Person represents a physical person in the system
type Person struct {
	base.Model
	FirstName string  `bun:"first_name,notnull" json:"first_name"`
	LastName  string  `bun:"last_name,notnull" json:"last_name"`
	TagID     *string `bun:"tag_id" json:"tag_id,omitempty"`
	AccountID *int64  `bun:"account_id" json:"account_id,omitempty"`

	// Relations not stored in the database
	Account *auth.Account `bun:"-" json:"account,omitempty"`
	RFIDCard *RFIDCard   `bun:"-" json:"rfid_card,omitempty"`
}

// TableName returns the database table name
func (p *Person) TableName() string {
	return "users.persons"
}

// Validate ensures person data is valid
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

	// Ensure at least one identifier is set (TagID or AccountID)
	if p.TagID == nil && p.AccountID == nil {
		return errors.New("at least one identifier (TagID or AccountID) must be set")
	}

	return nil
}

// GetFullName returns the complete name of the person
func (p *Person) GetFullName() string {
	return p.FirstName + " " + p.LastName
}

// SetAccount links this person to an account
func (p *Person) SetAccount(account *auth.Account) {
	p.Account = account
	if account != nil {
		p.AccountID = &account.ID
	} else {
		p.AccountID = nil
	}
}

// SetRFIDCard links this person to an RFID card
func (p *Person) SetRFIDCard(card *RFIDCard) {
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