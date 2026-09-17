package domain

import (
	"errors"
	"net"
	"time"
)

var (
	ErrOperatorMFACredentialNotFound = errors.New("operator mfa credential not found")
	ErrOperatorMFAChallengeNotFound  = errors.New("operator mfa challenge not found")
	// ErrOperatorMFAChallengeStateChanged reports an activation or
	// consumption that found no row in the expected state: a concurrent
	// verification consumed the code first, or the row is gone.
	ErrOperatorMFAChallengeStateChanged = errors.New("operator mfa challenge was already consumed or activated")
	// ErrOperatorTrustedDeviceNotFound reports a lookup that matched no
	// active trusted device, or a revocation of a device that is already
	// revoked or gone.
	ErrOperatorTrustedDeviceNotFound = errors.New("operator trusted device not found")
)

// OperatorMFAMethodEmail is the only enrollment method operators have.
const OperatorMFAMethodEmail = "email"

// OperatorMFACredential records that an operator enrolled in MFA. Operator
// MFA is mandatory; the row only tells login which gate to apply.
type OperatorMFACredential struct {
	ID         int64
	OperatorID int64
	Method     string
	EnrolledAt time.Time
	LastUsedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Validate rejects a credential that cannot be stored.
func (c *OperatorMFACredential) Validate() error {
	switch {
	case c.OperatorID <= 0:
		return errors.New("operator_id is required")
	case c.Method != OperatorMFAMethodEmail:
		return errors.New("unsupported MFA method")
	}
	return nil
}

// OperatorMFAChallenge is one hashed six-digit code e-mailed to an operator.
// A challenge is stored consumed until the mail was accepted and activated
// afterwards, so an undelivered code is never redeemable.
type OperatorMFAChallenge struct {
	ID         int64
	OperatorID int64
	CodeHash   string
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	IPAddress  net.IP
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Validate rejects a challenge that cannot be stored.
func (c *OperatorMFAChallenge) Validate() error {
	switch {
	case c.OperatorID <= 0:
		return errors.New("operator_id is required")
	case c.CodeHash == "":
		return errors.New("code_hash is required")
	case c.ExpiresAt.IsZero():
		return errors.New("expires_at is required")
	}
	return nil
}

// OperatorTrustedDevice is the server-side record behind an operator's
// signed remember-device cookie.
type OperatorTrustedDevice struct {
	ID         int64
	OperatorID int64
	TokenHash  string
	UserAgent  *string
	IPAddress  net.IP
	ExpiresAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Validate rejects a trusted device that cannot be stored.
func (d *OperatorTrustedDevice) Validate() error {
	switch {
	case d.OperatorID <= 0:
		return errors.New("operator_id is required")
	case d.TokenHash == "":
		return errors.New("token_hash is required")
	case d.ExpiresAt.IsZero():
		return errors.New("expires_at is required")
	}
	return nil
}
