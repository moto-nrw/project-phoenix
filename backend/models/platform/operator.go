package platform

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// tablePlatformOperators is the schema-qualified table name
const tablePlatformOperators = "platform.operators"

// Operator represents a platform operator (moto DevOps team member)
type Operator struct {
	base.Model   `bun:"schema:platform,table:operators"`
	Email        string     `bun:"email,notnull,unique" json:"email"`
	DisplayName  string     `bun:"display_name,notnull" json:"display_name"`
	PasswordHash string     `bun:"password_hash,notnull" json:"-"`
	Active       bool       `bun:"active,notnull,default:true" json:"active"`
	LastLogin    *time.Time `bun:"last_login" json:"last_login,omitempty"`

	// MFA-Lockout fields (operator MFA is hardcoded mandatory; lockout mirrors auth.accounts)
	MFAAttempts    int        `bun:"mfa_attempts,default:0" json:"-"`
	MFALockedUntil *time.Time `bun:"mfa_locked_until" json:"-"`
}

// IsMFALocked reports whether the operator is currently in the MFA-failure
// cooldown window.
func (o *Operator) IsMFALocked() bool {
	if o.MFALockedUntil == nil {
		return false
	}
	return time.Now().Before(*o.MFALockedUntil)
}

// IncrementMFAAttempts records a failed MFA verification and applies the
// 15-minute lockout once the threshold of 5 failures is hit.
func (o *Operator) IncrementMFAAttempts() {
	o.MFAAttempts++
	if o.MFAAttempts >= 5 {
		lockUntil := time.Now().Add(15 * time.Minute)
		o.MFALockedUntil = &lockUntil
	}
}

// ResetMFAAttempts clears any in-flight MFA-failure counter and lockout.
func (o *Operator) ResetMFAAttempts() {
	o.MFAAttempts = 0
	o.MFALockedUntil = nil
}

func (o *Operator) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tablePlatformOperators)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tablePlatformOperators)
	}
	return nil
}

// TableName returns the database table name
func (o *Operator) TableName() string {
	return tablePlatformOperators
}

// Validate ensures operator data is valid
func (o *Operator) Validate() error {
	o.Email = strings.TrimSpace(strings.ToLower(o.Email))
	o.DisplayName = strings.TrimSpace(o.DisplayName)

	if o.Email == "" {
		return errors.New("email is required")
	}
	if len(o.Email) > 255 {
		return errors.New("email must not exceed 255 characters")
	}
	if !strings.Contains(o.Email, "@") {
		return errors.New("invalid email format")
	}
	if o.DisplayName == "" {
		return errors.New("display name is required")
	}
	if len(o.DisplayName) > 100 {
		return errors.New("display name must not exceed 100 characters")
	}
	return nil
}

// GetID returns the entity's ID
func (o *Operator) GetID() any {
	return o.ID
}

// GetCreatedAt returns the creation timestamp
func (o *Operator) GetCreatedAt() time.Time {
	return o.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (o *Operator) GetUpdatedAt() time.Time {
	return o.UpdatedAt
}
