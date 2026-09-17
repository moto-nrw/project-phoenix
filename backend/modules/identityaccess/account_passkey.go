package identityaccess

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Account passkey records (#2724): the WebAuthn credentials a school-portal
// account registered and the server-side state of that portal's ceremonies.
// A credential belongs to the account and carries no school; the ceremony
// records the school whose portal started it, and login verifies the
// account's membership in that school before it mints a session.
var (
	// ErrAccountPasskeyNotFound reports a lookup without an active
	// credential, or a use or revocation of a credential that is revoked,
	// belongs to another account or is gone.
	ErrAccountPasskeyNotFound = errors.New("account passkey not found")
	// ErrAccountPasskeySessionNotFound reports a consumption that found no
	// unconsumed, unexpired ceremony with that ID and purpose.
	ErrAccountPasskeySessionNotFound = errors.New("account passkey session not found")
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

// AccountPasskeyQuery resolves active, that is unrevoked, account passkeys.
type AccountPasskeyQuery interface {
	// ListActiveAccountPasskeys orders by registration, oldest first.
	ListActiveAccountPasskeys(ctx context.Context, accountID int64) ([]AccountPasskeyCredential, error)
	FindActiveAccountPasskey(ctx context.Context, credentialID, userHandle []byte) (AccountPasskeyCredential, error)
}

// AccountPasskeyCommand changes account passkey records. The creates
// validate the record and return it with its identity and timestamps.
type AccountPasskeyCommand interface {
	CreateAccountPasskey(ctx context.Context, credential AccountPasskeyCredential) (AccountPasskeyCredential, error)
	// RecordAccountPasskeyUse stores the credential state after a verified
	// assertion on an active credential.
	RecordAccountPasskeyUse(ctx context.Context, id int64, credentialJSON json.RawMessage, usedAt time.Time) error
	// RevokeAccountPasskey revokes an active credential of that account.
	RevokeAccountPasskey(ctx context.Context, accountID, id int64, revokedAt time.Time) error
	CreateAccountPasskeySession(ctx context.Context, session AccountPasskeySession) (AccountPasskeySession, error)
	// ConsumeAccountPasskeySession completes a ceremony exactly once and
	// returns its state.
	ConsumeAccountPasskeySession(ctx context.Context, id, purpose string, consumedAt time.Time) (AccountPasskeySession, error)
}

// AccountPasskeyRecords is the capability the school-portal passkey flows
// consume.
type AccountPasskeyRecords interface {
	AccountPasskeyQuery
	AccountPasskeyCommand
}

func (m *Module) ListActiveAccountPasskeys(ctx context.Context, accountID int64) ([]AccountPasskeyCredential, error) {
	credentials, err := m.engine.ListActiveAccountPasskeys(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("identity access: list active account passkeys: %w", err)
	}
	return credentials, nil
}

func (m *Module) FindActiveAccountPasskey(ctx context.Context, credentialID, userHandle []byte) (AccountPasskeyCredential, error) {
	credential, err := m.engine.FindActiveAccountPasskey(ctx, credentialID, userHandle)
	if err != nil {
		return AccountPasskeyCredential{}, fmt.Errorf("identity access: find active account passkey: %w", err)
	}
	return credential, nil
}

func (m *Module) CreateAccountPasskey(ctx context.Context, credential AccountPasskeyCredential) (AccountPasskeyCredential, error) {
	stored, err := m.engine.CreateAccountPasskey(ctx, credential)
	if err != nil {
		return AccountPasskeyCredential{}, fmt.Errorf("identity access: create account passkey: %w", err)
	}
	return stored, nil
}

func (m *Module) RecordAccountPasskeyUse(ctx context.Context, id int64, credentialJSON json.RawMessage, usedAt time.Time) error {
	if err := m.engine.RecordAccountPasskeyUse(ctx, id, credentialJSON, usedAt); err != nil {
		return fmt.Errorf("identity access: record account passkey use: %w", err)
	}
	return nil
}

func (m *Module) RevokeAccountPasskey(ctx context.Context, accountID, id int64, revokedAt time.Time) error {
	if err := m.engine.RevokeAccountPasskey(ctx, accountID, id, revokedAt); err != nil {
		return fmt.Errorf("identity access: revoke account passkey: %w", err)
	}
	return nil
}

func (m *Module) CreateAccountPasskeySession(ctx context.Context, session AccountPasskeySession) (AccountPasskeySession, error) {
	stored, err := m.engine.CreateAccountPasskeySession(ctx, session)
	if err != nil {
		return AccountPasskeySession{}, fmt.Errorf("identity access: create account passkey session: %w", err)
	}
	return stored, nil
}

func (m *Module) ConsumeAccountPasskeySession(ctx context.Context, id, purpose string, consumedAt time.Time) (AccountPasskeySession, error) {
	session, err := m.engine.ConsumeAccountPasskeySession(ctx, id, purpose, consumedAt)
	if err != nil {
		return AccountPasskeySession{}, fmt.Errorf("identity access: consume account passkey session: %w", err)
	}
	return session, nil
}
