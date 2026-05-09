package platform

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/uptrace/bun"
)

const (
	operatorMFARecoveryCodeTable      = "platform.operator_mfa_recovery_codes"
	operatorMFARecoveryCodeTableAlias = `platform.operator_mfa_recovery_codes AS "operator_mfa_recovery_code"`
)

type OperatorMFARecoveryCodeRepository struct {
	*base.Repository[*platform.OperatorMFARecoveryCode]
	db *bun.DB
}

func NewOperatorMFARecoveryCodeRepository(db *bun.DB) platform.OperatorMFARecoveryCodeRepository {
	return &OperatorMFARecoveryCodeRepository{
		Repository: base.NewRepository[*platform.OperatorMFARecoveryCode](db, operatorMFARecoveryCodeTable, "OperatorMFARecoveryCode"),
		db:         db,
	}
}

func (r *OperatorMFARecoveryCodeRepository) BulkCreate(ctx context.Context, codes []*platform.OperatorMFARecoveryCode) error {
	if len(codes) == 0 {
		return nil
	}
	for _, code := range codes {
		if code == nil {
			return fmt.Errorf("operator mfa recovery code entry cannot be nil")
		}
		if code.OperatorID == 0 {
			return fmt.Errorf("operator_id is required on every recovery code")
		}
		if code.CodeHash == "" {
			return fmt.Errorf("code_hash is required on every recovery code")
		}
	}
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(&codes).
		ModelTableExpr(operatorMFARecoveryCodeTable).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "bulk create operator mfa recovery codes", Err: err}
	}
	return nil
}

func (r *OperatorMFARecoveryCodeRepository) FindUnusedByOperatorID(ctx context.Context, operatorID int64) ([]*platform.OperatorMFARecoveryCode, error) {
	var codes []*platform.OperatorMFARecoveryCode
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&codes).
		ModelTableExpr(operatorMFARecoveryCodeTableAlias).
		Where("operator_id = ?", operatorID).
		Where("used_at IS NULL").
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find unused operator mfa recovery codes", Err: err}
	}
	return codes, nil
}

func (r *OperatorMFARecoveryCodeRepository) MarkUsed(ctx context.Context, id int64, usedAt time.Time) error {
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*platform.OperatorMFARecoveryCode)(nil)).
		ModelTableExpr(operatorMFARecoveryCodeTable).
		Set("used_at = ?", usedAt).
		Where(platformWhereID, id).
		Where("used_at IS NULL").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "mark operator mfa recovery code used", Err: err}
	}
	return base.AssertRowsAffected(res, 1, "mark operator mfa recovery code used")
}

func (r *OperatorMFARecoveryCodeRepository) DeleteByOperatorID(ctx context.Context, operatorID int64) error {
	_, err := base.GetDB(ctx, r.db).NewDelete().
		Model((*platform.OperatorMFARecoveryCode)(nil)).
		ModelTableExpr(operatorMFARecoveryCodeTable).
		Where("operator_id = ?", operatorID).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "delete operator mfa recovery codes by operator id", Err: err}
	}
	return nil
}

func (r *OperatorMFARecoveryCodeRepository) CountUnused(ctx context.Context, operatorID int64) (int, error) {
	count, err := base.GetDB(ctx, r.db).NewSelect().
		Model((*platform.OperatorMFARecoveryCode)(nil)).
		ModelTableExpr(operatorMFARecoveryCodeTableAlias).
		Where("operator_id = ?", operatorID).
		Where("used_at IS NULL").
		Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "count unused operator mfa recovery codes", Err: err}
	}
	return count, nil
}
