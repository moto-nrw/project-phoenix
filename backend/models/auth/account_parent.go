package auth

import (
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// AccountParent represents a parent/guardian authentication account
type AccountParent struct {
	base.Model `bun:"schema:auth,table:accounts_parents"`
	base.TenantModel
	Email        string     `bun:"email,notnull" json:"email"`
	Username     *string    `bun:"username" json:"username,omitempty"`
	Active       bool       `bun:"active,notnull,default:true" json:"active"`
	PasswordHash *string    `bun:"password_hash" json:"-"`
	LastLogin    *time.Time `bun:"last_login" json:"last_login,omitempty"`
}

// Validate ensures account parent data is valid
func (a *AccountParent) Validate() error {
	return validateAccountEmail(&a.Email)
}

// IsActive returns whether the account is active
func (a *AccountParent) IsActive() bool {
	return a.Active
}

// SetLastLogin updates the last login timestamp
func (a *AccountParent) SetLastLogin(time time.Time) {
	a.LastLogin = &time
}
