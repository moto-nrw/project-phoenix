package platform

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/uptrace/bun"
)

const (
	operatorPasskeyCredentialTable      = "platform.operator_passkey_credentials"
	operatorPasskeyCredentialTableAlias = `platform.operator_passkey_credentials AS "operator_passkey_credential"`
	operatorPasskeySessionTable         = "platform.operator_passkey_sessions"
	operatorPasskeySessionTableAlias    = `platform.operator_passkey_sessions AS "operator_passkey_session"`
)

type OperatorPasskeyCredentialRepository struct {
	*base.Repository[*platform.OperatorPasskeyCredential]
	db *bun.DB
}

func NewOperatorPasskeyCredentialRepository(db *bun.DB) platform.OperatorPasskeyCredentialRepository {
	return &OperatorPasskeyCredentialRepository{
		Repository: base.NewRepository[*platform.OperatorPasskeyCredential](db, operatorPasskeyCredentialTable, "OperatorPasskeyCredential"),
		db:         db,
	}
}

func (r *OperatorPasskeyCredentialRepository) FindActiveByOperatorID(ctx context.Context, operatorID int64) ([]*platform.OperatorPasskeyCredential, error) {
	var credentials []*platform.OperatorPasskeyCredential
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&credentials).
		ModelTableExpr(operatorPasskeyCredentialTableAlias).
		Where("operator_id = ?", operatorID).
		Where("revoked_at IS NULL").
		Order("created_at ASC").
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find active operator passkeys by operator id", Err: err}
	}
	return credentials, nil
}

func (r *OperatorPasskeyCredentialRepository) FindActiveByCredentialIDAndUserHandle(ctx context.Context, credentialID, userHandle []byte) (*platform.OperatorPasskeyCredential, error) {
	credential := new(platform.OperatorPasskeyCredential)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(credential).
		ModelTableExpr(operatorPasskeyCredentialTableAlias).
		Where("credential_id = ?", credentialID).
		Where("user_handle = ?", userHandle).
		Where("revoked_at IS NULL").
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find active operator passkey by credential id and user handle", Err: err}
	}
	return credential, nil
}

func (r *OperatorPasskeyCredentialRepository) UpdateAfterUse(ctx context.Context, id int64, credentialJSON []byte, usedAt time.Time) error {
	credential := &platform.OperatorPasskeyCredential{
		Model:          modelBase.Model{ID: id},
		CredentialJSON: credentialJSON,
		LastUsedAt:     &usedAt,
	}
	updated, err := r.UpdateColumnsIfNull(ctx, credential, "revoked_at", "credential_json", "last_used_at")
	if err != nil {
		return base.UpdateOperationError(err, "update operator passkey after use")
	}
	return base.AssertRowsAffectedCount(updated, 1, "update operator passkey after use")
}

func (r *OperatorPasskeyCredentialRepository) Revoke(ctx context.Context, operatorID, id int64, revokedAt time.Time) error {
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*platform.OperatorPasskeyCredential)(nil)).
		ModelTableExpr(operatorPasskeyCredentialTable).
		Set("revoked_at = ?", revokedAt).
		Where(platformWhereID, id).
		Where("operator_id = ?", operatorID).
		Where("revoked_at IS NULL").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "revoke operator passkey", Err: err}
	}
	return base.AssertRowsAffected(res, 1, "revoke operator passkey")
}

type OperatorPasskeySessionRepository struct {
	*base.Repository[*platform.OperatorPasskeySession]
	db *bun.DB
}

func NewOperatorPasskeySessionRepository(db *bun.DB) platform.OperatorPasskeySessionRepository {
	return &OperatorPasskeySessionRepository{
		Repository: base.NewRepository[*platform.OperatorPasskeySession](db, operatorPasskeySessionTable, "OperatorPasskeySession"),
		db:         db,
	}
}

func (r *OperatorPasskeySessionRepository) Consume(ctx context.Context, id, purpose string, now time.Time) (*platform.OperatorPasskeySession, error) {
	session := new(platform.OperatorPasskeySession)
	err := base.GetDB(ctx, r.db).NewUpdate().
		Model(session).
		ModelTableExpr(operatorPasskeySessionTable).
		Set("consumed_at = ?", now).
		Where("id = ?", id).
		Where("purpose = ?", purpose).
		Where("consumed_at IS NULL").
		Where("expires_at > ?", now).
		Returning("*").
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "consume operator passkey session", Err: err}
	}
	return session, nil
}

func (r *OperatorPasskeySessionRepository) DeleteExpired(ctx context.Context, now time.Time) (int, error) {
	deleted, err := r.DeleteBefore(ctx, "expires_at", now, "delete expired operator passkey sessions")
	return int(deleted), err
}
