package auth

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Token represents an authentication token in the system
type Token struct {
	base.Model
	AccountID  int64     `bun:"account_id,notnull" json:"account_id"`
	Token      string    `bun:"token,notnull" json:"token"`
	Expiry     time.Time `bun:"expiry,notnull" json:"expiry"`
	Mobile     bool      `bun:"mobile,notnull,default:false" json:"mobile"`
	Identifier *string   `bun:"identifier" json:"identifier,omitempty"`

	// Relations
	Account *Account `bun:"rel:belongs-to,join:account_id=id" json:"account,omitempty"`
}

// TableName returns the database table name
func (t *Token) TableName() string {
	return "auth.tokens"
}

// Validate ensures token data is valid
func (t *Token) Validate() error {
	if t.AccountID <= 0 {
		return errors.New("account ID is required")
	}

	if t.Token == "" {
		return errors.New("token value is required")
	}

	// Check if token has expired
	if t.Expiry.Before(time.Now()) {
		return errors.New("token has already expired")
	}

	return nil
}

// IsExpired checks if the token has expired
func (t *Token) IsExpired() bool {
	return t.Expiry.Before(time.Now())
}

// SetExpiry sets the token expiry time to a specified duration from now
func (t *Token) SetExpiry(duration time.Duration) {
	t.Expiry = time.Now().Add(duration)
}
