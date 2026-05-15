package auth

import (
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/userpass"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Account represents an authentication account
type Account struct {
	base.Model     `bun:"schema:auth,table:accounts"`
	Email          string     `bun:"email,notnull" json:"email"`
	Username       *string    `bun:"username,unique" json:"username,omitempty"`
	Avatar         string     `bun:"avatar" json:"avatar,omitempty"`
	Active         bool       `bun:"active,notnull,default:true" json:"active"`
	PasswordHash   *string    `bun:"password_hash" json:"-"`
	IsPasswordOTP  bool       `bun:"is_password_otp,default:false" json:"is_password_otp"`
	LastLogin      *time.Time `bun:"last_login" json:"last_login,omitempty"`
	PINHash        *string    `bun:"pin_hash" json:"-"`
	PINAttempts    int        `bun:"pin_attempts,default:0" json:"-"`
	PINLockedUntil *time.Time `bun:"pin_locked_until" json:"-"`
	MFAAttempts    int        `bun:"mfa_attempts,default:0" json:"-"`
	MFALockedUntil *time.Time `bun:"mfa_locked_until" json:"-"`
	// MFAAdminOverride is an admin-controlled per-account toggle that
	// overrides the tenant-level mfa_mode setting:
	//   - "none"      → fall back to tenant + role rules
	//   - "force_off" → 2FA disabled for this user (e.g. lost mailbox access)
	//   - "force_on"  → 2FA required regardless of tenant mode
	// Writes flow through MFAService.SetMFAOverride so audit + trusted-device
	// revocation stay in one place.
	MFAAdminOverride string `bun:"mfa_admin_override,notnull,default:'none'" json:"mfa_admin_override"`

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
	// INSERT queries should not use aliases
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`auth.accounts AS "account"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`auth.accounts AS "account"`)
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

// PIN-related methods

// HashPIN hashes a PIN using Argon2id
func (a *Account) HashPIN(pin string) error {
	hashedPIN, err := userpass.HashPassword(pin, nil)
	if err != nil {
		return err
	}
	a.PINHash = &hashedPIN
	return nil
}

// VerifyPIN verifies a PIN against the stored hash
func (a *Account) VerifyPIN(pin string) bool {
	if a.PINHash == nil {
		return false
	}
	isValid, err := userpass.VerifyPassword(pin, *a.PINHash)
	if err != nil {
		return false
	}
	return isValid
}

// HasPIN checks if the account has a PIN set
func (a *Account) HasPIN() bool {
	return a.PINHash != nil && *a.PINHash != ""
}

// IsPINLocked checks if the account is temporarily locked due to failed PIN attempts
func (a *Account) IsPINLocked() bool {
	if a.PINLockedUntil == nil {
		return false
	}
	return time.Now().Before(*a.PINLockedUntil)
}

// IncrementPINAttempts increments the failed PIN attempt counter and locks if needed
func (a *Account) IncrementPINAttempts() {
	a.PINAttempts++

	// Lock account for 15 minutes after 5 failed attempts
	if a.PINAttempts >= 5 {
		lockUntil := time.Now().Add(15 * time.Minute)
		a.PINLockedUntil = &lockUntil
	}
}

// ResetPINAttempts resets the failed PIN attempt counter
func (a *Account) ResetPINAttempts() {
	a.PINAttempts = 0
	a.PINLockedUntil = nil
}

// ClearPIN removes the PIN from the account
func (a *Account) ClearPIN() {
	a.PINHash = nil
	a.ResetPINAttempts()
}

// MFA-related lockout helpers (mirror PIN-Lockout pattern)

// IsMFALocked reports whether the account is currently in the MFA-failure
// cooldown window.
func (a *Account) IsMFALocked() bool {
	if a.MFALockedUntil == nil {
		return false
	}
	return time.Now().Before(*a.MFALockedUntil)
}

// IncrementMFAAttempts records a failed MFA verification and applies the
// 15-minute lockout once the threshold of 5 failures is hit.
func (a *Account) IncrementMFAAttempts() {
	a.MFAAttempts++
	if a.MFAAttempts >= 5 {
		lockUntil := time.Now().Add(15 * time.Minute)
		a.MFALockedUntil = &lockUntil
	}
}

// ResetMFAAttempts clears any in-flight MFA-failure counter and lockout.
func (a *Account) ResetMFAAttempts() {
	a.MFAAttempts = 0
	a.MFALockedUntil = nil
}
