package auth

import (
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/userpass"
	"github.com/moto-nrw/project-phoenix/models/base"
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
	// CalendarFeedToken stores the SHA-256 HASH of the secret capability token in
	// the parent's iCalendar subscription URL — never the raw token, so a DB read
	// exposes no replayable /api/calendar-feed/{token} URL. The service hashes on
	// write and before lookup. Unset until the parent first requests their feed.
	CalendarFeedToken *string `bun:"calendar_feed_token" json:"-"`
	// The per-account MFA admin override no longer lives on this row.
	// Tenant-scoped overrides + the operator's account-wide emergency
	// override are stored in auth.mfa_overrides — see
	// models.auth.MFAOverride. Callers go through MFAService for both
	// reads and writes; the resolver consults the override table on
	// every IsRequired call.

	// Relations not stored in the database
	Roles       []*Role       `bun:"-" json:"roles,omitempty"`
	Permissions []*Permission `bun:"-" json:"permissions,omitempty"`
}

// AuthorizationRoleNames returns the names of loaded roles. It exposes data,
// not an authorization decision, so policy packages need not import ORM
// models.
func (a *Account) AuthorizationRoleNames() []string {
	if a == nil {
		return nil
	}
	names := make([]string, 0, len(a.Roles))
	for _, role := range a.Roles {
		if role != nil {
			names = append(names, role.Name)
		}
	}
	return names
}

// Validate ensures account data is valid
func (a *Account) Validate() error {
	return validateAccountEmail(&a.Email)
}

// validateAccountEmail is the shared required/format/lowercase email rule
// of Account and AccountParent. It normalizes the address in place.
func validateAccountEmail(email *string) error {
	if *email == "" {
		return errors.New("email is required")
	}

	// Validate email format
	if _, err := mail.ParseAddress(*email); err != nil {
		return errors.New("invalid email format")
	}

	// Convert email to lowercase for consistency
	*email = strings.ToLower(*email)

	return nil
}

// IsActive returns whether the account is active
func (a *Account) IsActive() bool {
	return a.Active
}

// SetLastLogin updates the last login timestamp
func (a *Account) SetLastLogin(time time.Time) {
	a.LastLogin = &time
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

// PIN- and MFA-failure lockout policy no longer lives on the model
// (issue #586, Rule 12). The decision (is the account locked?) and the
// counter mutations are owned by the service layer with atomic repository
// methods so concurrent failures can't share an attempt budget:
//   - PIN:  services/users person service + services/auth Service.IsPINLocked /
//           RecordFailedPINAttempt / ResetPINLockout
//           (database/repositories/auth AccountRepository.IncrementPINAttempts,
//            ResetPINAttempts, ClearPIN)
//   - MFA:  services/auth mfaService.isMFALocked / handleFailedAttempt
//           (AccountRepository.IncrementMFAAttempts, ResetMFAAttempts)
// The account row holds only the pin_attempts / pin_locked_until /
// mfa_attempts / mfa_locked_until facts.
