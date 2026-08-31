package auth

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const (
	passkeyCredentialTable      = "auth.passkey_credentials"
	passkeyCredentialTableAlias = `auth.passkey_credentials AS "passkey_credential"`
	passkeySessionTable         = "auth.passkey_sessions"
	passkeySessionTableAlias    = `auth.passkey_sessions AS "passkey_session"`
)

type PasskeyCredentialRepository struct {
	*base.Repository[*auth.PasskeyCredential]
	db *bun.DB
}

func NewPasskeyCredentialRepository(db *bun.DB) auth.PasskeyCredentialRepository {
	return &PasskeyCredentialRepository{
		Repository: base.NewRepository[*auth.PasskeyCredential](db, passkeyCredentialTable, "PasskeyCredential"),
		db:         db,
	}
}

func (r *PasskeyCredentialRepository) FindActiveByAccountID(ctx context.Context, accountID int64) ([]*auth.PasskeyCredential, error) {
	var credentials []*auth.PasskeyCredential
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&credentials).
		ModelTableExpr(passkeyCredentialTableAlias).
		Where("account_id = ?", accountID).
		Where("revoked_at IS NULL").
		Order("created_at ASC").
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find active passkeys by account id", Err: base.TranslateNotFound(err)}
	}
	return credentials, nil
}

func (r *PasskeyCredentialRepository) FindActiveByCredentialIDAndUserHandle(ctx context.Context, credentialID, userHandle []byte) (*auth.PasskeyCredential, error) {
	credential := new(auth.PasskeyCredential)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(credential).
		ModelTableExpr(passkeyCredentialTableAlias).
		Where("credential_id = ?", credentialID).
		Where("user_handle = ?", userHandle).
		Where("revoked_at IS NULL").
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find active passkey by credential id and user handle", Err: base.TranslateNotFound(err)}
	}
	return credential, nil
}

func (r *PasskeyCredentialRepository) UpdateAfterUse(ctx context.Context, id int64, credentialJSON []byte, usedAt time.Time) error {
	credential := &auth.PasskeyCredential{
		Model:          modelBase.Model{ID: id},
		CredentialJSON: credentialJSON,
		LastUsedAt:     &usedAt,
	}
	updated, err := r.UpdateColumnsIfNull(ctx, credential, "revoked_at", "credential_json", "last_used_at")
	if err != nil {
		return base.UpdateOperationError(err, "update passkey after use")
	}
	return base.AssertRowsAffectedCount(updated, 1, "update passkey after use")
}

func (r *PasskeyCredentialRepository) Revoke(ctx context.Context, accountID, id int64, revokedAt time.Time) error {
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*auth.PasskeyCredential)(nil)).
		ModelTableExpr(passkeyCredentialTable).
		Set("revoked_at = ?", revokedAt).
		Where(whereID, id).
		Where("account_id = ?", accountID).
		Where("revoked_at IS NULL").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "revoke passkey", Err: base.TranslateNotFound(err)}
	}
	return base.AssertRowsAffected(res, 1, "revoke passkey")
}

type PasskeySessionRepository struct {
	*base.Repository[*auth.PasskeySession]
	db *bun.DB
}

func NewPasskeySessionRepository(db *bun.DB) auth.PasskeySessionRepository {
	return &PasskeySessionRepository{
		Repository: base.NewRepository[*auth.PasskeySession](db, passkeySessionTable, "PasskeySession"),
		db:         db,
	}
}

func (r *PasskeySessionRepository) Consume(ctx context.Context, id, purpose string, now time.Time) (*auth.PasskeySession, error) {
	session := new(auth.PasskeySession)
	err := base.GetDB(ctx, r.db).NewUpdate().
		Model(session).
		ModelTableExpr(passkeySessionTable).
		Set("consumed_at = ?", now).
		Where("id = ?", id).
		Where("purpose = ?", purpose).
		Where("consumed_at IS NULL").
		Where("expires_at > ?", now).
		Returning("*").
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "consume passkey session", Err: base.TranslateNotFound(err)}
	}
	return session, nil
}

func (r *PasskeySessionRepository) DeleteExpired(ctx context.Context, now time.Time) (int, error) {
	deleted, err := r.DeleteBefore(ctx, "expires_at", now, "delete expired passkey sessions")
	return int(deleted), err
}
