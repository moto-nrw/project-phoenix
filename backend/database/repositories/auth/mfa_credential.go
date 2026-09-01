package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const (
	mfaCredentialTable      = "auth.mfa_credentials"
	mfaCredentialTableAlias = `auth.mfa_credentials AS "mfa_credential"`
)

// MFACredentialRepository implements auth.MFACredentialRepository.
type MFACredentialRepository struct {
	*base.Repository[*auth.MFACredential]
	db *bun.DB
}

// NewMFACredentialRepository creates a new repository for MFA credential records.
func NewMFACredentialRepository(db *bun.DB) auth.MFACredentialRepository {
	return &MFACredentialRepository{
		// EntityName "MfaCredential" (not "MFACredential"): the generic derives
		// the SQL alias by snake-casing, and it must match bun's model alias
		// "mfa_credential" for WherePK-based methods like UpdateColumns.
		Repository: base.NewRepository[*auth.MFACredential](db, mfaCredentialTable, "MfaCredential"),
		db:         db,
	}
}

// Update validates and persists modifications to an existing credential.
func (r *MFACredentialRepository) Update(ctx context.Context, credential *auth.MFACredential) error {
	if credential == nil {
		return fmt.Errorf("mfa credential cannot be nil")
	}
	if err := credential.Validate(); err != nil {
		return err
	}
	_, err := base.GetDB(ctx, r.db).NewUpdate().
		Model(credential).
		ModelTableExpr(mfaCredentialTable).
		Where(whereID, credential.ID).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "update mfa credential", Err: base.TranslateNotFound(err)}
	}
	return nil
}

// FindByAccountID returns the (single) MFA credential for an account or nil if none exists.
func (r *MFACredentialRepository) FindByAccountID(ctx context.Context, accountID int64) (*auth.MFACredential, error) {
	credential := new(auth.MFACredential)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(credential).
		ModelTableExpr(mfaCredentialTableAlias).
		Where("account_id = ?", accountID).
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find mfa credential by account id", Err: base.TranslateNotFound(err)}
	}
	return credential, nil
}

// UpdateLastUsedAt stamps the credential after a successful verification.
func (r *MFACredentialRepository) UpdateLastUsedAt(ctx context.Context, id int64, when time.Time) error {
	credential := &auth.MFACredential{Model: modelBase.Model{ID: id}, LastUsedAt: &when}
	_, err := r.UpdateColumns(ctx, credential, "last_used_at")
	return err
}

// DeleteByAccountID removes the credential for an account (used by user opt-out + admin override).
func (r *MFACredentialRepository) DeleteByAccountID(ctx context.Context, accountID int64) error {
	_, err := base.GetDB(ctx, r.db).NewDelete().
		Model((*auth.MFACredential)(nil)).
		ModelTableExpr(mfaCredentialTable).
		Where("account_id = ?", accountID).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "delete mfa credential by account id", Err: base.TranslateNotFound(err)}
	}
	return nil
}

// List retrieves credentials matching the provided filters.
func (r *MFACredentialRepository) List(ctx context.Context, filters map[string]interface{}) ([]*auth.MFACredential, error) {
	var credentials []*auth.MFACredential
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&credentials).
		ModelTableExpr(mfaCredentialTableAlias)
	for field, value := range filters {
		if value != nil {
			query = query.Where("? = ?", bun.Ident(field), value)
		}
	}
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list mfa credentials", Err: base.TranslateNotFound(err)}
	}
	return credentials, nil
}
