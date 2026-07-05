package users

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/auth"
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
	Account  *auth.Account `bun:"rel:belongs-to,join:account_id=id" json:"account,omitempty"`
	RFIDCard *RFIDCard     `bun:"rel:belongs-to,join:tag_id=id" json:"rfid_card,omitempty"`
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
