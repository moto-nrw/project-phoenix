package domain

import (
	"errors"
	"strings"
	"time"
)

const (
	maxOperatorEmailLength       = 255
	maxOperatorDisplayNameLength = 100
)

// Operator is the platform-wide login identity of a moto operator. It has
// no tenant; the platform portal authenticates it directly.
type Operator struct {
	ID             int64
	Email          string
	DisplayName    string
	PasswordHash   string
	Active         bool
	LastLogin      *time.Time
	MFAAttempts    int
	MFALockedUntil *time.Time
	DeactivatedAt  *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Validate normalizes the address and display name in place and rejects an
// operator that cannot be stored.
func (o *Operator) Validate() error {
	o.Email = strings.TrimSpace(strings.ToLower(o.Email))
	o.DisplayName = strings.TrimSpace(o.DisplayName)
	switch {
	case o.Email == "":
		return errors.New("email is required")
	case len(o.Email) > maxOperatorEmailLength:
		return errors.New("email must not exceed 255 characters")
	case !strings.Contains(o.Email, "@"):
		return ErrOperatorEmailInvalid
	case o.DisplayName == "":
		return ErrOperatorDisplayNameRequired
	case len(o.DisplayName) > maxOperatorDisplayNameLength:
		return ErrOperatorDisplayNameTooLong
	}
	return nil
}

// OperatorMFAAttempts is the counter snapshot after one failed verification.
type OperatorMFAAttempts struct {
	Attempts    int
	LockedUntil *time.Time
}

// OperatorSession is one persisted, revocable operator refresh session. A
// family groups the generations one login produced through rotation.
//
// Token is the raw value the refresh JWT carries. Only the minting or
// presenting request knows it; the store keeps TokenHash, its hex SHA-256,
// and ReplacementTokenHash likewise names the successor by hash (#4902).
// Listings and revocations return Token empty.
type OperatorSession struct {
	ID                   int64
	OperatorID           int64
	Token                string
	TokenHash            string
	Expiry               time.Time
	FamilyID             string
	Generation           int
	RotatedAt            *time.Time
	ReplacementTokenHash *string
	RecoveryProofHash    []byte
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// Validate rejects a session that cannot be stored.
func (s *OperatorSession) Validate() error {
	switch {
	case s.OperatorID <= 0:
		return errors.New("operator ID is required")
	case s.Token == "":
		return errors.New("token value is required")
	case s.Expiry.IsZero():
		return errors.New("expiry is required")
	case s.FamilyID == "":
		return errors.New("family ID is required")
	case (s.RotatedAt == nil) != (s.ReplacementTokenHash == nil):
		return errors.New("rotation handoff must include both timestamp and replacement token")
	case s.RotatedAt == nil && len(s.RecoveryProofHash) != 0:
		return errors.New("recovery proof hash requires a rotation handoff")
	}
	return nil
}
