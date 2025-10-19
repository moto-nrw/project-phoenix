package auth

import (
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// AccountParent represents a parent/guardian authentication account
type AccountParent struct {
	base.Model   `bun:"schema:auth,table:accounts_parents"`
	Email        string     `bun:"email,notnull" json:"email"`
	Username     *string    `bun:"username,unique" json:"username,omitempty"`
	Active       bool       `bun:"active,notnull,default:true" json:"active"`
	PasswordHash *string    `bun:"password_hash" json:"-"`
	LastLogin    *time.Time `bun:"last_login" json:"last_login,omitempty"`
}

// TableName returns the database table name
func (a *AccountParent) TableName() string {
	return "auth.accounts_parents"
}

func (a *AccountParent) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(`auth.accounts_parents AS "accountparent"`)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`auth.accounts_parents AS "accountparent"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`auth.accounts_parents AS "accountparent"`)
	}
	return nil
}

// Validate ensures account parent data is valid
func (a *AccountParent) Validate() error {
	if a.Email == "" {
		return errors.New("email is required")
	}

	// Validate email format
	if _, err := mail.ParseAddress(a.Email); err != nil {
		return errors.New("invalid email format")
	}

	// Convert email to lowercase for consistency
	a.Email = strings.ToLower(a.Email)

	return nil
}

// IsActive returns whether the account is active
func (a *AccountParent) IsActive() bool {
	return a.Active
}

// SetLastLogin updates the last login timestamp
func (a *AccountParent) SetLastLogin(time time.Time) {
	a.LastLogin = &time
}

// GetID returns the entity's ID
func (a *AccountParent) GetID() interface{} {
	return a.ID
}

// GetCreatedAt returns the creation timestamp
func (a *AccountParent) GetCreatedAt() time.Time {
	return a.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (a *AccountParent) GetUpdatedAt() time.Time {
	return a.UpdatedAt
}
