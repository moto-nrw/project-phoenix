package domain

import (
	"encoding/json"
	"errors"
	"time"
)

var (
	// ErrOperatorPasskeyNotFound reports a lookup that matched no active
	// credential, or a use or revocation that found no active credential of
	// that operator.
	ErrOperatorPasskeyNotFound = errors.New("operator passkey not found")
	// ErrOperatorPasskeySessionNotFound reports a consumption that found no
	// unconsumed, unexpired ceremony with that ID and purpose.
	ErrOperatorPasskeySessionNotFound = errors.New("operator passkey session not found")
	// ErrOperatorPasskeyJSONInvalid rejects credential state the WebAuthn
	// library could not read back.
	ErrOperatorPasskeyJSONInvalid = errors.New("credential_json must be valid JSON")
)

const (
	OperatorPasskeySessionPurposeRegistration = "registration"
	OperatorPasskeySessionPurposeLogin        = "login"
)

// OperatorPasskeyCredential is one WebAuthn credential an operator
// registered. Only unrevoked credentials verify assertions.
type OperatorPasskeyCredential struct {
	ID             int64
	OperatorID     int64
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
func (c *OperatorPasskeyCredential) Validate() error {
	switch {
	case c.OperatorID <= 0:
		return errors.New("operator_id is required")
	case len(c.UserHandle) == 0:
		return errors.New("user_handle is required")
	case len(c.CredentialID) == 0:
		return errors.New("credential_id is required")
	case !json.Valid(c.CredentialJSON):
		return ErrOperatorPasskeyJSONInvalid
	}
	return nil
}

// OperatorPasskeySession is the server-side state of one operator passkey
// ceremony. It completes at most once, for its own purpose, before it
// expires.
type OperatorPasskeySession struct {
	ID             string
	OperatorID     *int64
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
func (s *OperatorPasskeySession) Validate() error {
	switch {
	case s.ID == "":
		return errors.New("id is required")
	case s.Purpose != OperatorPasskeySessionPurposeRegistration && s.Purpose != OperatorPasskeySessionPurposeLogin:
		return errors.New("unsupported operator passkey session purpose")
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
