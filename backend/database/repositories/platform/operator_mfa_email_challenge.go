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
	operatorMFAEmailChallengeTable      = "platform.operator_mfa_email_challenges"
	operatorMFAEmailChallengeTableAlias = `platform.operator_mfa_email_challenges AS "operator_mfa_email_challenge"`
)

type OperatorMFAEmailChallengeRepository struct {
	*base.Repository[*platform.OperatorMFAEmailChallenge]
	db *bun.DB
}

func NewOperatorMFAEmailChallengeRepository(db *bun.DB) platform.OperatorMFAEmailChallengeRepository {
	return &OperatorMFAEmailChallengeRepository{
		Repository: base.NewRepository[*platform.OperatorMFAEmailChallenge](db, operatorMFAEmailChallengeTable, "OperatorMfaEmailChallenge"),
		db:         db,
	}
}

func (r *OperatorMFAEmailChallengeRepository) Create(ctx context.Context, challenge *platform.OperatorMFAEmailChallenge) error {
	if challenge == nil {
		return fmt.Errorf("operator mfa email challenge cannot be nil")
	}
	if challenge.OperatorID == 0 {
		return fmt.Errorf("operator_id is required")
	}
	if challenge.CodeHash == "" {
		return fmt.Errorf("code_hash is required")
	}
	if challenge.ExpiresAt.IsZero() {
		return fmt.Errorf("expires_at is required")
	}
	return r.Repository.Create(ctx, challenge)
}

func (r *OperatorMFAEmailChallengeRepository) FindActiveByOperatorID(ctx context.Context, operatorID int64) (*platform.OperatorMFAEmailChallenge, error) {
	challenge := new(platform.OperatorMFAEmailChallenge)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(challenge).
		ModelTableExpr(operatorMFAEmailChallengeTableAlias).
		Where("operator_id = ?", operatorID).
		Where("consumed_at IS NULL").
		Where("expires_at > ?", time.Now()).
		Order("expires_at DESC").
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find active operator mfa email challenge", Err: base.TranslateNotFound(err)}
	}
	return challenge, nil
}

func (r *OperatorMFAEmailChallengeRepository) MarkConsumed(ctx context.Context, id int64, consumedAt time.Time) error {
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*platform.OperatorMFAEmailChallenge)(nil)).
		ModelTableExpr(operatorMFAEmailChallengeTable).
		Set("consumed_at = ?", consumedAt).
		Where(platformWhereID, id).
		Where("consumed_at IS NULL").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "mark operator mfa email challenge consumed", Err: base.TranslateNotFound(err)}
	}
	return base.AssertRowsAffected(res, 1, "mark operator mfa email challenge consumed")
}

func (r *OperatorMFAEmailChallengeRepository) MarkActive(ctx context.Context, id int64) error {
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*platform.OperatorMFAEmailChallenge)(nil)).
		ModelTableExpr(operatorMFAEmailChallengeTable).
		Set("consumed_at = NULL").
		Where(platformWhereID, id).
		Where("consumed_at IS NOT NULL").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "mark operator mfa email challenge active", Err: base.TranslateNotFound(err)}
	}
	return base.AssertRowsAffected(res, 1, "mark operator mfa email challenge active")
}

func (r *OperatorMFAEmailChallengeRepository) CountRecentByOperatorID(ctx context.Context, operatorID int64, since time.Time) (int, error) {
	count, err := base.GetDB(ctx, r.db).NewSelect().
		Model((*platform.OperatorMFAEmailChallenge)(nil)).
		ModelTableExpr(operatorMFAEmailChallengeTableAlias).
		Where("operator_id = ?", operatorID).
		Where("created_at >= ?", since).
		Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "count recent operator mfa email challenges", Err: base.TranslateNotFound(err)}
	}
	return count, nil
}

func (r *OperatorMFAEmailChallengeRepository) DeleteExpired(ctx context.Context) (int, error) {
	deleted, err := r.DeleteBefore(ctx, "expires_at", time.Now(), "delete expired operator mfa email challenges")
	return int(deleted), err
}
