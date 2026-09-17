package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/services/platform"
)

// Identity & Access owns the operator passkey credentials and ceremony
// sessions (#2724). operatorPasskeyRecords serves the retained operator
// passkey service's consumer-owned port over the public module: only the
// owner's not-found outcomes become a missing row or an unrevoked
// credential, every other failure stays an error, and the stored identity
// and timestamps are written back into the caller's value.
type operatorPasskeyRecords struct {
	records identityaccess.OperatorPasskeyRecords
}

func newOperatorPasskeyRecords(records identityaccess.OperatorPasskeyRecords) platform.OperatorPasskeyRecords {
	return operatorPasskeyRecords{records: records}
}

func (r operatorPasskeyRecords) CreateCredential(ctx context.Context, credential *platformModels.OperatorPasskeyCredential) error {
	if credential == nil {
		return fmt.Errorf("operator passkey credential cannot be nil")
	}
	stored, err := r.records.CreateOperatorPasskey(ctx, identityaccess.OperatorPasskeyCredential{
		OperatorID: credential.OperatorID, UserHandle: credential.UserHandle, CredentialID: credential.CredentialID,
		CredentialJSON: credential.CredentialJSON, Name: credential.Name, LastUsedAt: credential.LastUsedAt, RevokedAt: credential.RevokedAt,
	})
	if err != nil {
		return operatorDatabaseError("create operator passkey", err)
	}
	*credential = *operatorPasskeyCredentialModel(stored)
	return nil
}

func (r operatorPasskeyRecords) ListActiveCredentials(ctx context.Context, operatorID int64) ([]*platformModels.OperatorPasskeyCredential, error) {
	credentials, err := r.records.ListActiveOperatorPasskeys(ctx, operatorID)
	if err != nil {
		return nil, operatorDatabaseError("list active operator passkeys", err)
	}
	result := make([]*platformModels.OperatorPasskeyCredential, 0, len(credentials))
	for _, credential := range credentials {
		result = append(result, operatorPasskeyCredentialModel(credential))
	}
	return result, nil
}

func (r operatorPasskeyRecords) FindActiveCredential(ctx context.Context, credentialID, userHandle []byte) (*platformModels.OperatorPasskeyCredential, error) {
	credential, err := r.records.FindActiveOperatorPasskey(ctx, credentialID, userHandle)
	if errors.Is(err, identityaccess.ErrOperatorPasskeyNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, operatorDatabaseError("find active operator passkey", err)
	}
	return operatorPasskeyCredentialModel(credential), nil
}

func (r operatorPasskeyRecords) RecordCredentialUse(ctx context.Context, id int64, credentialJSON []byte, usedAt time.Time) error {
	if err := r.records.RecordOperatorPasskeyUse(ctx, id, credentialJSON, usedAt); err != nil {
		return operatorDatabaseError("update operator passkey after use", err)
	}
	return nil
}

func (r operatorPasskeyRecords) RevokeCredential(ctx context.Context, operatorID, id int64, revokedAt time.Time) (bool, error) {
	err := r.records.RevokeOperatorPasskey(ctx, operatorID, id, revokedAt)
	if errors.Is(err, identityaccess.ErrOperatorPasskeyNotFound) {
		return false, nil
	}
	if err != nil {
		return false, operatorDatabaseError("revoke operator passkey", err)
	}
	return true, nil
}

func (r operatorPasskeyRecords) CreateSession(ctx context.Context, session *platformModels.OperatorPasskeySession) error {
	if session == nil {
		return fmt.Errorf("operator passkey session cannot be nil")
	}
	stored, err := r.records.CreateOperatorPasskeySession(ctx, identityaccess.OperatorPasskeySession{
		ID: session.ID, OperatorID: session.OperatorID, Purpose: session.Purpose, RPID: session.RPID,
		ExpectedOrigin: session.ExpectedOrigin, SessionJSON: session.SessionJSON, ExpiresAt: session.ExpiresAt, ConsumedAt: session.ConsumedAt,
	})
	if err != nil {
		return operatorDatabaseError("create operator passkey session", err)
	}
	*session = *operatorPasskeySessionModel(stored)
	return nil
}

func (r operatorPasskeyRecords) ConsumeSession(ctx context.Context, id, purpose string, consumedAt time.Time) (*platformModels.OperatorPasskeySession, error) {
	session, err := r.records.ConsumeOperatorPasskeySession(ctx, id, purpose, consumedAt)
	if errors.Is(err, identityaccess.ErrOperatorPasskeySessionNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, operatorDatabaseError("consume operator passkey session", err)
	}
	return operatorPasskeySessionModel(session), nil
}

func operatorPasskeyCredentialModel(src identityaccess.OperatorPasskeyCredential) *platformModels.OperatorPasskeyCredential {
	credential := &platformModels.OperatorPasskeyCredential{
		OperatorID: src.OperatorID, UserHandle: src.UserHandle, CredentialID: src.CredentialID, CredentialJSON: src.CredentialJSON,
		Name: src.Name, LastUsedAt: src.LastUsedAt, RevokedAt: src.RevokedAt,
	}
	credential.ID, credential.CreatedAt, credential.UpdatedAt = src.ID, src.CreatedAt, src.UpdatedAt
	return credential
}

func operatorPasskeySessionModel(src identityaccess.OperatorPasskeySession) *platformModels.OperatorPasskeySession {
	session := &platformModels.OperatorPasskeySession{
		OperatorID: src.OperatorID, Purpose: src.Purpose, RPID: src.RPID, ExpectedOrigin: src.ExpectedOrigin,
		SessionJSON: src.SessionJSON, ExpiresAt: src.ExpiresAt, ConsumedAt: src.ConsumedAt,
	}
	session.ID, session.CreatedAt, session.UpdatedAt = src.ID, src.CreatedAt, src.UpdatedAt
	return session
}
