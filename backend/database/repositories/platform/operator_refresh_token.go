package platform

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/uptrace/bun"
)

const (
	operatorRefreshTokenTable      = "platform.operator_refresh_tokens"
	operatorRefreshTokenTableAlias = `platform.operator_refresh_tokens AS "operator_refresh_token"`
)

type OperatorRefreshTokenRepository struct {
	*base.Repository[*platform.OperatorRefreshToken]
	db *bun.DB
}

func NewOperatorRefreshTokenRepository(db *bun.DB) platform.OperatorRefreshTokenRepository {
	return &OperatorRefreshTokenRepository{
		Repository: base.NewRepository[*platform.OperatorRefreshToken](db, operatorRefreshTokenTable, "OperatorRefreshToken"),
		db:         db,
	}
}

func (r *OperatorRefreshTokenRepository) FindByTokenForUpdate(ctx context.Context, tokenValue string) (*platform.OperatorRefreshToken, error) {
	token := new(platform.OperatorRefreshToken)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(token).
		ModelTableExpr(operatorRefreshTokenTableAlias).
		Where(`"operator_refresh_token".token = ?`, tokenValue).
		For("UPDATE").
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "find operator refresh token for update", Err: err}
	}
	return token, nil
}

func (r *OperatorRefreshTokenRepository) MarkRotated(ctx context.Context, id int64, replacementToken string, recoveryProofHash []byte, rotatedAt time.Time) error {
	result, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*platform.OperatorRefreshToken)(nil)).
		ModelTableExpr(operatorRefreshTokenTableAlias).
		Set(`rotated_at = ?`, rotatedAt).
		Set(`replacement_token = ?`, replacementToken).
		Set(`recovery_proof_hash = ?`, recoveryProofHash).
		Where(`"operator_refresh_token".id = ?`, id).
		Where(`"operator_refresh_token".rotated_at IS NULL`).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "mark operator refresh token rotated", Err: err}
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return &modelBase.DatabaseError{Op: "count rotated operator refresh tokens", Err: err}
	}
	if affected != 1 {
		return &modelBase.DatabaseError{Op: "mark operator refresh token rotated", Err: errors.New("operator refresh token was already rotated or not found")}
	}
	return nil
}

// DeleteExpiredRotated removes predecessor rows only after their refresh JWTs
// expire, preserving family evidence for replay detection until then.
func (r *OperatorRefreshTokenRepository) DeleteExpiredRotated(ctx context.Context, familyID string, now time.Time) error {
	db := base.GetDB(ctx, r.db)
	candidates := db.NewSelect().
		Model((*platform.OperatorRefreshToken)(nil)).
		ModelTableExpr(operatorRefreshTokenTableAlias).
		ColumnExpr(`"operator_refresh_token".id`).
		Where(`"operator_refresh_token".family_id = ?`, familyID).
		Where(`"operator_refresh_token".rotated_at IS NOT NULL`).
		Where(`"operator_refresh_token".expiry <= ?`, now).
		For("UPDATE SKIP LOCKED")

	_, err := db.NewDelete().
		Model((*platform.OperatorRefreshToken)(nil)).
		ModelTableExpr(operatorRefreshTokenTableAlias).
		Where(`"operator_refresh_token".id IN (?)`, candidates).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "delete expired rotated operator refresh tokens", Err: err}
	}
	return nil
}

func (r *OperatorRefreshTokenRepository) Delete(ctx context.Context, id any) error {
	_, err := base.GetDB(ctx, r.db).NewDelete().
		Model((*platform.OperatorRefreshToken)(nil)).
		ModelTableExpr(operatorRefreshTokenTableAlias).
		Where(`"operator_refresh_token".id = ?`, id).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "delete operator refresh token", Err: err}
	}
	return nil
}

func (r *OperatorRefreshTokenRepository) DeleteByOperatorID(ctx context.Context, operatorID int64) (int, error) {
	res, err := base.GetDB(ctx, r.db).NewDelete().
		Model((*platform.OperatorRefreshToken)(nil)).
		ModelTableExpr(operatorRefreshTokenTableAlias).
		Where(`"operator_refresh_token".operator_id = ?`, operatorID).
		Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete operator refresh tokens by operator ID", Err: err}
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "count deleted operator refresh tokens", Err: err}
	}
	return int(affected), nil
}

func (r *OperatorRefreshTokenRepository) DeleteByFamilyID(ctx context.Context, familyID string) error {
	_, err := base.GetDB(ctx, r.db).NewDelete().
		Model((*platform.OperatorRefreshToken)(nil)).
		ModelTableExpr(operatorRefreshTokenTableAlias).
		Where(`"operator_refresh_token".family_id = ?`, familyID).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "delete operator refresh token family", Err: err}
	}
	return nil
}

func (r *OperatorRefreshTokenRepository) GetLatestTokenInFamily(ctx context.Context, familyID string) (*platform.OperatorRefreshToken, error) {
	token := new(platform.OperatorRefreshToken)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(token).
		ModelTableExpr(operatorRefreshTokenTableAlias).
		Where(`"operator_refresh_token".family_id = ?`, familyID).
		OrderExpr(`"operator_refresh_token".generation DESC`).
		Limit(1).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "get latest operator refresh token in family", Err: err}
	}
	return token, nil
}

func (r *OperatorRefreshTokenRepository) DeleteExpired(ctx context.Context, now time.Time) (int, error) {
	deleted, err := r.DeleteBefore(ctx, "expiry", now, "delete expired operator refresh tokens")
	return int(deleted), err
}
