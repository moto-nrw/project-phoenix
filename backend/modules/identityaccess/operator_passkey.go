package identityaccess

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	ErrOperatorPasskeyNotFound = errors.New("operator passkey not found")
	// ErrOperatorPasskeySessionNotFound reports a consumption that found no
	// unconsumed, unexpired ceremony with that ID and purpose. The loser of
	// two concurrent completions receives it.
	ErrOperatorPasskeySessionNotFound = errors.New("operator passkey session not found")
)

// The ceremony purposes an operator passkey session is bound to.
const (
	OperatorPasskeySessionPurposeRegistration = "registration"
	OperatorPasskeySessionPurposeLogin        = "login"
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

// OperatorPasskeyQuery resolves active, that is unrevoked, operator
// passkeys.
type OperatorPasskeyQuery interface {
	// ListActiveOperatorPasskeys orders by registration, oldest first.
	ListActiveOperatorPasskeys(ctx context.Context, operatorID int64) ([]OperatorPasskeyCredential, error)
	FindActiveOperatorPasskey(ctx context.Context, credentialID, userHandle []byte) (OperatorPasskeyCredential, error)
}

// OperatorPasskeyCommand changes operator passkey records. The creates
// validate the record and return it with its identity and timestamps.
type OperatorPasskeyCommand interface {
	CreateOperatorPasskey(ctx context.Context, credential OperatorPasskeyCredential) (OperatorPasskeyCredential, error)
	// RecordOperatorPasskeyUse stores the credential state after a verified
	// assertion on an active credential.
	RecordOperatorPasskeyUse(ctx context.Context, id int64, credentialJSON json.RawMessage, usedAt time.Time) error
	// RevokeOperatorPasskey revokes an active credential of that operator.
	RevokeOperatorPasskey(ctx context.Context, operatorID, id int64, revokedAt time.Time) error
	CreateOperatorPasskeySession(ctx context.Context, session OperatorPasskeySession) (OperatorPasskeySession, error)
	// ConsumeOperatorPasskeySession completes a ceremony exactly once and
	// returns its state.
	ConsumeOperatorPasskeySession(ctx context.Context, id, purpose string, consumedAt time.Time) (OperatorPasskeySession, error)
}

// OperatorPasskeyRecords is the capability the operator passkey flows
// consume.
type OperatorPasskeyRecords interface {
	OperatorPasskeyQuery
	OperatorPasskeyCommand
}

func (m *Module) ListActiveOperatorPasskeys(ctx context.Context, operatorID int64) ([]OperatorPasskeyCredential, error) {
	credentials, err := m.engine.ListActiveOperatorPasskeys(ctx, operatorID)
	if err != nil {
		return nil, fmt.Errorf("identity access: list active operator passkeys: %w", err)
	}
	return credentials, nil
}

func (m *Module) FindActiveOperatorPasskey(ctx context.Context, credentialID, userHandle []byte) (OperatorPasskeyCredential, error) {
	credential, err := m.engine.FindActiveOperatorPasskey(ctx, credentialID, userHandle)
	if err != nil {
		return OperatorPasskeyCredential{}, fmt.Errorf("identity access: find active operator passkey: %w", err)
	}
	return credential, nil
}

func (m *Module) CreateOperatorPasskey(ctx context.Context, credential OperatorPasskeyCredential) (OperatorPasskeyCredential, error) {
	stored, err := m.engine.CreateOperatorPasskey(ctx, credential)
	if err != nil {
		return OperatorPasskeyCredential{}, fmt.Errorf("identity access: create operator passkey: %w", err)
	}
	return stored, nil
}

func (m *Module) RecordOperatorPasskeyUse(ctx context.Context, id int64, credentialJSON json.RawMessage, usedAt time.Time) error {
	if err := m.engine.RecordOperatorPasskeyUse(ctx, id, credentialJSON, usedAt); err != nil {
		return fmt.Errorf("identity access: record operator passkey use: %w", err)
	}
	return nil
}

func (m *Module) RevokeOperatorPasskey(ctx context.Context, operatorID, id int64, revokedAt time.Time) error {
	if err := m.engine.RevokeOperatorPasskey(ctx, operatorID, id, revokedAt); err != nil {
		return fmt.Errorf("identity access: revoke operator passkey: %w", err)
	}
	return nil
}

func (m *Module) CreateOperatorPasskeySession(ctx context.Context, session OperatorPasskeySession) (OperatorPasskeySession, error) {
	stored, err := m.engine.CreateOperatorPasskeySession(ctx, session)
	if err != nil {
		return OperatorPasskeySession{}, fmt.Errorf("identity access: create operator passkey session: %w", err)
	}
	return stored, nil
}

func (m *Module) ConsumeOperatorPasskeySession(ctx context.Context, id, purpose string, consumedAt time.Time) (OperatorPasskeySession, error) {
	session, err := m.engine.ConsumeOperatorPasskeySession(ctx, id, purpose, consumedAt)
	if err != nil {
		return OperatorPasskeySession{}, fmt.Errorf("identity access: consume operator passkey session: %w", err)
	}
	return session, nil
}
