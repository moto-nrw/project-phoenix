package domain

import (
	"encoding/json"
	"errors"
	"time"
)

var (
	// ErrAccountPasskeyNotFound reports a lookup that matched no active
	// credential, or a use or revocation that found no active credential of
	// that account.
	ErrAccountPasskeyNotFound = errors.New("account passkey not found")
	// ErrAccountPasskeySessionNotFound reports a consumption that found no
	// unconsumed, unexpired ceremony with that ID and purpose.
	ErrAccountPasskeySessionNotFound = errors.New("account passkey session not found")
)

// AccountPasskeyCredential is one WebAuthn credential a school-portal
// account registered. Credentials belong to the account, not to a school;
// only unrevoked credentials verify assertions.
type AccountPasskeyCredential struct {
	ID             int64
	AccountID      int64
	UserHandle     []byte
	CredentialID   []byte
	CredentialJSON json.RawMessage
	Name           string
	LastUsedAt     *time.Time
	RevokedAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Validate rejects a credential that cannot be stored.
func (c *AccountPasskeyCredential) Validate() error {
	switch {
	case c.AccountID <= 0:
		return errors.New("account_id is required")
	case len(c.UserHandle) == 0:
		return errors.New("user_handle is required")
	case len(c.CredentialID) == 0:
		return errors.New("credential_id is required")
	case !json.Valid(c.CredentialJSON):
		return ErrPasskeyJSONInvalid
	}
	return nil
}

// AccountPasskeySession is the server-side state of one school-portal
// ceremony. TenantID names the school whose portal started it; AccountID is
// nil for a discoverable login, which learns the account from the assertion.
type AccountPasskeySession struct {
	ID             string
	AccountID      *int64
	TenantID       *int64
	Purpose        string
	RPID           string
	ExpectedOrigin string
	SessionJSON    json.RawMessage
	ExpiresAt      time.Time
	ConsumedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Validate rejects a session that cannot be stored.
func (s *AccountPasskeySession) Validate() error {
	switch {
	case s.ID == "":
		return errors.New("id is required")
	case s.Purpose != PasskeySessionPurposeRegistration && s.Purpose != PasskeySessionPurposeLogin:
		return errors.New("unsupported passkey session purpose")
	case s.RPID == "":
		return errors.New("rp_id is required")
	case s.ExpectedOrigin == "":
		return errors.New("expected_origin is required")
	case !json.Valid(s.SessionJSON):
		return errors.New("session_json must be valid JSON")
	case s.ExpiresAt.IsZero():
		return errors.New("expires_at is required")
	}
	return nil
}
