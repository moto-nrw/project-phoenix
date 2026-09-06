package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

const lateInviteTableExpr = `enrollment.late_invites AS "late_invite"`

func (r *Store) InsertLateInvite(ctx context.Context, invite *enrollment.LateInvite) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	if invite.TenantID != 0 && invite.TenantID != tenantID {
		return fmt.Errorf("late invite tenant mismatch")
	}
	if _, err := r.Phase(ctx, invite.PhaseID); err != nil {
		return fmt.Errorf("find late invite phase: %w", err)
	}
	if invite.UsedRequestID != nil {
		if _, err := r.RequestByID(ctx, *invite.UsedRequestID, false); err != nil {
			return fmt.Errorf("find late invite request: %w", err)
		}
	}
	row := lateInviteStorage(invite)
	row.TenantID = tenantID
	now := time.Now()
	if row.CreatedAt.IsZero() {
		row.CreatedAt = now
	}
	row.UpdatedAt = now
	_, err = db.NewInsert().
		Model(row).
		ModelTableExpr(lateInviteTableExpr).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create enrollment late invite: %w", err)
	}
	*invite = *row.value()
	return nil
}

func (r *Store) LateInviteByUsedRequestID(ctx context.Context, requestID int64) (*enrollment.LateInvite, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}

	invite := new(lateInviteRow)
	if err := db.NewSelect().
		Model(invite).
		ModelTableExpr(lateInviteTableExpr).
		Where("tenant_id = ?", tenantID).
		Where(`"late_invite".used_request_id = ?`, requestID).
		Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, enrollment.ErrLateInviteNotFound
		}
		return nil, fmt.Errorf("failed to find enrollment late invite by used request: %w", err)
	}
	return invite.value(), nil
}

func (r *Store) UsableLateInvite(ctx context.Context, tokenHash string, phaseID int64, now time.Time, lock bool) (*enrollment.LateInvite, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}

	invite := new(lateInviteRow)
	q := db.NewSelect().
		Model(invite).
		ModelTableExpr(lateInviteTableExpr).
		Where("tenant_id = ?", tenantID).
		Where(`"late_invite".token_hash = ?`, tokenHash).
		Where(`"late_invite".phase_id = ?`, phaseID).
		Where(`"late_invite".used_at IS NULL`).
		Where(`"late_invite".expires_at > ?`, now)
	if lock {
		q = q.For("UPDATE")
	}
	if err := q.Scan(ctx); err != nil {
		// Sentinel, not a bare message: callers gate access on the difference
		// between "no usable invite" and "lookup failed" (#1663).
		if errors.Is(err, sql.ErrNoRows) {
			return nil, enrollment.ErrLateInviteNotFound
		}
		return nil, fmt.Errorf("failed to find enrollment late invite: %w", err)
	}
	return invite.value(), nil
}

func (r *Store) MarkLateInviteUsed(ctx context.Context, inviteID, requestID int64, usedAt time.Time) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}

	if _, err := r.RequestByID(ctx, requestID, false); err != nil {
		return fmt.Errorf("find late invite request: %w", err)
	}

	res, err := db.NewUpdate().
		Model((*lateInviteRow)(nil)).
		ModelTableExpr(lateInviteTableExpr).
		Where("tenant_id = ?", tenantID).
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
func (r *Store) DeleteLateInvitesByUsedRequestID(ctx context.Context, requestID int64) (int64, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return 0, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return 0, err
	}

	res, err := db.NewDelete().
		Model((*lateInviteRow)(nil)).
		ModelTableExpr(lateInviteTableExpr).
		Where("tenant_id = ?", tenantID).
		Where(`"late_invite".used_request_id = ?`, requestID).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to delete enrollment late invites by used request: %w", err)
	}
	affected, _ := res.RowsAffected()
	return affected, nil
}
