package auth

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Token represents an authentication token in the system
type Token struct {
	base.Model `bun:"schema:auth,table:tokens"`
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

// GetID returns the entity's ID
func (m *Token) GetID() interface{} {
	return m.ID
}

// GetCreatedAt returns the creation timestamp
func (m *Token) GetCreatedAt() time.Time {
	return m.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (m *Token) GetUpdatedAt() time.Time {
	return m.UpdatedAt
}

// BeforeAppendModel lets us modify query before it's executed
func (t *Token) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr("auth.tokens")
	}
	if q, ok := query.(*bun.InsertQuery); ok {
		q.ModelTableExpr("auth.tokens")
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr("auth.tokens")
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr("auth.tokens")
	}
	return nil
}
