package identityaccess

import (
	"encoding/json"
	"time"
)

// Operator passkey records (#2724): the WebAuthn credentials an operator
// registered and the server-side state of the operator portal's passkey
// ceremonies. The rows are platform-wide like the operator itself. Every
// operation joins the caller's transaction when one is active.
var (
	// ErrOperatorPasskeyNotFound reports a lookup without an active
	// credential, or a use or revocation of a credential that is revoked,
	// belongs to another operator or is gone.
	ErrOperatorPasskeyNotFound = notFoundError("operator passkey not found")
	// ErrOperatorPasskeySessionNotFound reports a consumption that found no
	// unconsumed, unexpired ceremony with that ID and purpose. The loser of
	// two concurrent completions receives it.
	ErrOperatorPasskeySessionNotFound = notFoundError("operator passkey session not found")
)

// The ceremony purposes a passkey session is bound to, in both portals.
const (
	PasskeySessionPurposeRegistration = "registration"
	PasskeySessionPurposeLogin        = "login"
)

// OperatorPasskeyCredential is one registered WebAuthn credential.
// CredentialJSON is the serialized credential the WebAuthn library verifies
// assertions against.
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

// OperatorPasskeySession is the server-side state of one registration or
// login ceremony. OperatorID is nil for a discoverable login, which learns
// the operator only from the assertion.
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
