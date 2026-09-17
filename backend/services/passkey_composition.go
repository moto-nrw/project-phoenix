package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/services/auth"
)

// Identity & Access owns the school-portal passkey credentials and ceremony
// sessions (#2724). accountPasskeyRecords serves the retained passkey
// service's consumer-owned port over the public module: only the owner's
// not-found outcomes become a missing row or an unrevoked credential, every
// other failure stays an error, and the stored identity and timestamps are
// written back into the caller's value.
type accountPasskeyRecords struct {
	records identityaccess.AccountPasskeyRecords
}

func newAccountPasskeyRecords(records identityaccess.AccountPasskeyRecords) auth.PasskeyRecords {
	return accountPasskeyRecords{records: records}
}

func (r accountPasskeyRecords) CreateCredential(ctx context.Context, credential *auth.PasskeyCredential) error {
	if credential == nil {
		return fmt.Errorf("passkey credential cannot be nil")
	}
	stored, err := r.records.CreateAccountPasskey(ctx, identityaccess.AccountPasskeyCredential{
		AccountID: credential.AccountID, UserHandle: credential.UserHandle, CredentialID: credential.CredentialID,
		CredentialJSON: credential.CredentialJSON, Name: credential.Name, LastUsedAt: credential.LastUsedAt, RevokedAt: credential.RevokedAt,
	})
	if err != nil {
		return accountDatabaseError("create passkey", err)
	}
	*credential = *accountPasskeyCredentialModel(stored)
	return nil
}

func (r accountPasskeyRecords) ListActiveCredentials(ctx context.Context, accountID int64) ([]*auth.PasskeyCredential, error) {
	credentials, err := r.records.ListActiveAccountPasskeys(ctx, accountID)
	if err != nil {
		return nil, accountDatabaseError("list active passkeys", err)
	}
	result := make([]*auth.PasskeyCredential, 0, len(credentials))
	for _, credential := range credentials {
		result = append(result, accountPasskeyCredentialModel(credential))
	}
	return result, nil
}

func (r accountPasskeyRecords) FindActiveCredential(ctx context.Context, credentialID, userHandle []byte) (*auth.PasskeyCredential, error) {
	credential, err := r.records.FindActiveAccountPasskey(ctx, credentialID, userHandle)
	if errors.Is(err, identityaccess.ErrAccountPasskeyNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, accountDatabaseError("find active passkey", err)
	}
	return accountPasskeyCredentialModel(credential), nil
}

func (r accountPasskeyRecords) RecordCredentialUse(ctx context.Context, id int64, credentialJSON []byte, usedAt time.Time) error {
	if err := r.records.RecordAccountPasskeyUse(ctx, id, credentialJSON, usedAt); err != nil {
		return accountDatabaseError("update passkey after use", err)
	}
	return nil
}

func (r accountPasskeyRecords) RevokeCredential(ctx context.Context, accountID, id int64, revokedAt time.Time) (bool, error) {
	err := r.records.RevokeAccountPasskey(ctx, accountID, id, revokedAt)
	if errors.Is(err, identityaccess.ErrAccountPasskeyNotFound) {
		return false, nil
	}
	if err != nil {
		return false, accountDatabaseError("revoke passkey", err)
	}
	return true, nil
}

func (r accountPasskeyRecords) CreateSession(ctx context.Context, session *auth.PasskeySession) error {
	if session == nil {
		return fmt.Errorf("passkey session cannot be nil")
	}
	stored, err := r.records.CreateAccountPasskeySession(ctx, identityaccess.AccountPasskeySession{
		ID: session.ID, AccountID: session.AccountID, TenantID: session.TenantID, Purpose: session.Purpose, RPID: session.RPID,
		ExpectedOrigin: session.ExpectedOrigin, SessionJSON: session.SessionJSON, ExpiresAt: session.ExpiresAt, ConsumedAt: session.ConsumedAt,
	})
	if err != nil {
		return accountDatabaseError("create passkey session", err)
	}
	*session = *accountPasskeySessionModel(stored)
	return nil
}

func (r accountPasskeyRecords) ConsumeSession(ctx context.Context, id, purpose string, consumedAt time.Time) (*auth.PasskeySession, error) {
	session, err := r.records.ConsumeAccountPasskeySession(ctx, id, purpose, consumedAt)
	if errors.Is(err, identityaccess.ErrAccountPasskeySessionNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, accountDatabaseError("consume passkey session", err)
	}
	return accountPasskeySessionModel(session), nil
}

func accountPasskeyCredentialModel(src identityaccess.AccountPasskeyCredential) *auth.PasskeyCredential {
	credential := &auth.PasskeyCredential{
		AccountID: src.AccountID, UserHandle: src.UserHandle, CredentialID: src.CredentialID, CredentialJSON: src.CredentialJSON,
		Name: src.Name, LastUsedAt: src.LastUsedAt, RevokedAt: src.RevokedAt,
	}
	credential.ID, credential.CreatedAt, credential.UpdatedAt = src.ID, src.CreatedAt, src.UpdatedAt
	return credential
}

func accountPasskeySessionModel(src identityaccess.AccountPasskeySession) *auth.PasskeySession {
	session := &auth.PasskeySession{
		AccountID: src.AccountID, TenantID: src.TenantID, Purpose: src.Purpose, RPID: src.RPID, ExpectedOrigin: src.ExpectedOrigin,
		SessionJSON: src.SessionJSON, ExpiresAt: src.ExpiresAt, ConsumedAt: src.ConsumedAt,
	}
	session.ID, session.CreatedAt, session.UpdatedAt = src.ID, src.CreatedAt, src.UpdatedAt
	return session
}

func accountDatabaseError(op string, err error) error {
	return fmt.Errorf("database error during %s: %w", op, err)
}
