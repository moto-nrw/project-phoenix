package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrOperatorNotFound        = errors.New("operator not found")
	ErrOperatorSessionNotFound = errors.New("operator session not found")
	// ErrOperatorSessionRotated reports a rotation hand-off that found no
	// un-rotated row: the session was already rotated or does not exist.
	ErrOperatorSessionRotated = errors.New("operator session was already rotated or not found")
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
		return errors.New("invalid email format")
	case o.DisplayName == "":
		return errors.New("display name is required")
	case len(o.DisplayName) > maxOperatorDisplayNameLength:
		return errors.New("display name must not exceed 100 characters")
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
type OperatorSession struct {
	ID                int64
	OperatorID        int64
	Token             string
	Expiry            time.Time
	FamilyID          string
	Generation        int
	RotatedAt         *time.Time
	ReplacementToken  *string
	RecoveryProofHash []byte
	CreatedAt         time.Time
	UpdatedAt         time.Time
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
	case (s.RotatedAt == nil) != (s.ReplacementToken == nil):
		return errors.New("rotation handoff must include both timestamp and replacement token")
	case s.RotatedAt == nil && len(s.RecoveryProofHash) != 0:
		return errors.New("recovery proof hash requires a rotation handoff")
	}
	return nil
}
