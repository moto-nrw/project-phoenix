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
	mfaRecoveryCodeTable      = "auth.mfa_recovery_codes"
	mfaRecoveryCodeTableAlias = `auth.mfa_recovery_codes AS "mfa_recovery_code"`
)

// MFARecoveryCodeRepository implements auth.MFARecoveryCodeRepository.
type MFARecoveryCodeRepository struct {
	*base.Repository[*auth.MFARecoveryCode]
	db *bun.DB
}

// NewMFARecoveryCodeRepository creates a new repository for Argon2id-hashed recovery codes.
func NewMFARecoveryCodeRepository(db *bun.DB) auth.MFARecoveryCodeRepository {
	return &MFARecoveryCodeRepository{
		Repository: base.NewRepository[*auth.MFARecoveryCode](db, mfaRecoveryCodeTable, "MFARecoveryCode"),
		db:         db,
	}
}

// BulkCreate inserts a batch of recovery codes in a single statement.
// All codes must belong to the same account.
func (r *MFARecoveryCodeRepository) BulkCreate(ctx context.Context, codes []*auth.MFARecoveryCode) error {
	if len(codes) == 0 {
		return nil
	}
	for _, code := range codes {
		if code == nil {
			return fmt.Errorf("mfa recovery code entry cannot be nil")
		}
		if code.AccountID == 0 {
			return fmt.Errorf("account_id is required on every recovery code")
		}
		if code.CodeHash == "" {
			return fmt.Errorf("code_hash is required on every recovery code")
		}
	}
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(&codes).
		ModelTableExpr(mfaRecoveryCodeTable).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "bulk create mfa recovery codes", Err: err}
	}
	return nil
}

// FindUnusedByAccountID returns all recovery codes that have not yet been redeemed.
// The service walks this list and Argon2id-verifies each hash against the user-supplied code.
func (r *MFARecoveryCodeRepository) FindUnusedByAccountID(ctx context.Context, accountID int64) ([]*auth.MFARecoveryCode, error) {
	var codes []*auth.MFARecoveryCode
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&codes).
		ModelTableExpr(mfaRecoveryCodeTableAlias).
		Where("account_id = ?", accountID).
		Where("used_at IS NULL").
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find unused mfa recovery codes", Err: err}
	}
	return codes, nil
}

// MarkUsed flips the used_at timestamp on a single recovery code (single-use).
func (r *MFARecoveryCodeRepository) MarkUsed(ctx context.Context, id int64, usedAt time.Time) error {
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*auth.MFARecoveryCode)(nil)).
		ModelTableExpr(mfaRecoveryCodeTable).
		Set("used_at = ?", usedAt).
		Where(whereID, id).
		Where("used_at IS NULL").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "mark mfa recovery code used", Err: err}
	}
	return base.AssertRowsAffected(res, 1, "mark mfa recovery code used")
}

// DeleteByAccountID removes every recovery code for an account.
// Used both when regenerating (the service replaces the whole batch) and when
// disabling MFA entirely.
func (r *MFARecoveryCodeRepository) DeleteByAccountID(ctx context.Context, accountID int64) error {
	_, err := base.GetDB(ctx, r.db).NewDelete().
		Model((*auth.MFARecoveryCode)(nil)).
		ModelTableExpr(mfaRecoveryCodeTable).
		Where("account_id = ?", accountID).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "delete mfa recovery codes by account id", Err: err}
	}
	return nil
}

// CountUnused returns the number of unredeemed codes (used by the settings UI to
// nudge the user to regenerate when the pool is running out).
func (r *MFARecoveryCodeRepository) CountUnused(ctx context.Context, accountID int64) (int, error) {
	count, err := base.GetDB(ctx, r.db).NewSelect().
		Model((*auth.MFARecoveryCode)(nil)).
		ModelTableExpr(mfaRecoveryCodeTableAlias).
		Where("account_id = ?", accountID).
		Where("used_at IS NULL").
		Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "count unused mfa recovery codes", Err: err}
	}
	return count, nil
}
