package identityaccess

import (
	"encoding/json"
	"time"
)

// PasskeyRPName is the relying-party name the authenticators show for the
// passkeys of both portals.
const PasskeyRPName = "moto"

// Account passkey records (#2724): the WebAuthn credentials a school-portal
// account registered and the server-side state of that portal's ceremonies.
// A credential belongs to the account and carries no school; the ceremony
// records the school whose portal started it, and login verifies the
// account's membership in that school before it mints a session.
var (
	// ErrAccountPasskeyNotFound reports a lookup without an active
	// credential, or a use or revocation of a credential that is revoked,
	// belongs to another account or is gone.
	ErrAccountPasskeyNotFound = notFoundError("account passkey not found")
	// ErrAccountPasskeySessionNotFound reports a consumption that found no
	// unconsumed, unexpired ceremony with that ID and purpose.
	ErrAccountPasskeySessionNotFound = notFoundError("account passkey session not found")
)

// AccountPasskeyCredential is one registered WebAuthn credential of a
// school-portal account.
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

// AccountPasskeySession is the server-side state of one school-portal
// ceremony. AccountID is nil for a discoverable login, which learns the
// account only from the assertion; TenantID names the school whose portal
// started the ceremony.
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
