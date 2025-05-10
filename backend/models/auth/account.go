package auth

import (
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Account represents an authentication account
type Account struct {
	base.Model    `bun:"schema:auth,table:accounts,alias:account"`
	Email         string     `bun:"email,notnull" json:"email"`
	Username      *string    `bun:"username,unique" json:"username,omitempty"`
	Active        bool       `bun:"active,notnull,default:true" json:"active"`
	PasswordHash  *string    `bun:"password_hash" json:"-"`
	IsPasswordOTP bool       `bun:"is_password_otp,default:false" json:"is_password_otp"`
	LastLogin     *time.Time `bun:"last_login" json:"last_login,omitempty"`

	// Relations not stored in the database
	Roles       []*Role       `bun:"-" json:"roles,omitempty"`
	Permissions []*Permission `bun:"-" json:"permissions,omitempty"`
}

// TableName returns the database table name
func (a *Account) TableName() string {
	return "auth.accounts"
}

// BeforeAppendModel lets us modify query before it's executed
func (a *Account) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.TableExpr("auth.accounts AS account")
	}
	if q, ok := query.(*bun.InsertQuery); ok {
		q.TableExpr("auth.accounts")
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.TableExpr("auth.accounts")
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.TableExpr("auth.accounts")
	}
	return nil
}

// Validate ensures account data is valid
func (a *Account) Validate() error {
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

// HasRole checks if the account has the specified role
func (a *Account) HasRole(roleName string) bool {
	if a.Roles == nil {
		return false
	}

	for _, role := range a.Roles {
		if strings.EqualFold(role.Name, roleName) {
			return true
		}
	}

	return false
}

// HasPermission checks if the account has the specified permission
func (a *Account) HasPermission(permission string) bool {
	if a.Permissions == nil {
		return false
	}

	for _, p := range a.Permissions {
		if strings.EqualFold(p.Name, permission) {
			return true
		}
	}

	return false
}

// IsActive returns whether the account is active
func (a *Account) IsActive() bool {
	return a.Active
}

// SetLastLogin updates the last login timestamp
func (a *Account) SetLastLogin(time time.Time) {
	a.LastLogin = &time
}

// GetID returns the entity's ID
func (a *Account) GetID() interface{} {
	return a.ID
}

// GetCreatedAt returns the creation timestamp
func (a *Account) GetCreatedAt() time.Time {
	return a.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (a *Account) GetUpdatedAt() time.Time {
	return a.UpdatedAt
}
