package enrollment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/uptrace/bun"
)

const lateInviteTableExpr = `enrollment.late_invites AS "late_invite"`

type LateInviteRepository struct {
	db *bun.DB
}

func NewLateInviteRepository(db *bun.DB) enrollment.LateInviteRepository {
	return &LateInviteRepository{db: db}
}

func (r *LateInviteRepository) Create(ctx context.Context, invite *enrollment.LateInvite) error {
	base.EnsureTenantID(ctx, invite)
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(invite).
		ModelTableExpr(lateInviteTableExpr).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create enrollment late invite: %w", err)
	}
	return nil
}

func (r *LateInviteRepository) FindUsableByTokenHash(ctx context.Context, tokenHash string, phaseID int64, now time.Time) (*enrollment.LateInvite, error) {
	return r.findUsableByTokenHash(ctx, tokenHash, phaseID, now, false)
}

func (r *LateInviteRepository) FindUsableByTokenHashForUpdate(ctx context.Context, tokenHash string, phaseID int64, now time.Time) (*enrollment.LateInvite, error) {
	return r.findUsableByTokenHash(ctx, tokenHash, phaseID, now, true)
}

func (r *LateInviteRepository) findUsableByTokenHash(ctx context.Context, tokenHash string, phaseID int64, now time.Time, lock bool) (*enrollment.LateInvite, error) {
	invite := new(enrollment.LateInvite)
	q := base.GetDB(ctx, r.db).NewSelect().
		Model(invite).
		ModelTableExpr(lateInviteTableExpr).
		Where(`"late_invite".token_hash = ?`, tokenHash).
		Where(`"late_invite".phase_id = ?`, phaseID).
		Where(`"late_invite".used_at IS NULL`).
		Where(`"late_invite".expires_at > ?`, now)
	if lock {
		q = q.For("UPDATE")
	}
	if err := q.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("late invite not found")
		}
		return nil, fmt.Errorf("failed to find enrollment late invite: %w", err)
	}
	return invite, nil
}

func (r *LateInviteRepository) MarkUsed(ctx context.Context, inviteID, requestID int64, usedAt time.Time) error {
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*enrollment.LateInvite)(nil)).
		ModelTableExpr(lateInviteTableExpr).
		Set(`used_at = ?`, usedAt).
		Set(`used_request_id = ?`, requestID).
		Where(`"late_invite".id = ?`, inviteID).
		Where(`"late_invite".used_at IS NULL`).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to mark enrollment late invite used: %w", err)
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return fmt.Errorf("late invite was already used")
	}
	return nil
}

// DeleteByUsedRequestID removes late-invite PII linked to a request before the
// request deletion can clear used_request_id through ON DELETE SET NULL.
func (r *LateInviteRepository) DeleteByUsedRequestID(ctx context.Context, requestID int64) (int64, error) {
	res, err := base.GetDB(ctx, r.db).NewDelete().
		Model((*enrollment.LateInvite)(nil)).
		ModelTableExpr(lateInviteTableExpr).
		Where(`"late_invite".used_request_id = ?`, requestID).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to delete enrollment late invites by used request: %w", err)
	}
	affected, _ := res.RowsAffected()
	return affected, nil
}
