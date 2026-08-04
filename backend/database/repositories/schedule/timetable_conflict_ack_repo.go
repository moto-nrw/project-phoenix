package schedule

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

const (
	tableScheduleConflictAcks     = "schedule.timetable_conflict_acks"
	tableExprConflictAcksAsAck    = `schedule.timetable_conflict_acks AS "conflict_ack"`
	conflictAcksAccountFilterExpr = `"conflict_ack".account_id = ?`
)

// TimetableConflictAckRepository implements schedule.TimetableConflictAckRepository.
type TimetableConflictAckRepository struct {
	*base.Repository[*schedule.TimetableConflictAck]
	db *bun.DB
}

// NewTimetableConflictAckRepository creates the per-user conflict
// acknowledgement repository (#2139).
func NewTimetableConflictAckRepository(db *bun.DB) schedule.TimetableConflictAckRepository {
	repo := base.NewRepository[*schedule.TimetableConflictAck](db, tableScheduleConflictAcks, "TimetableConflictAck")
	repo.TenantScoped = true
	return &TimetableConflictAckRepository{Repository: repo, db: db}
}

// List applies QueryOptions, tenant-scoped (satisfies base.Repository[T]).
func (r *TimetableConflictAckRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*schedule.TimetableConflictAck, error) {
	return r.ListWithOptions(ctx, options)
}

// ListFingerprintsByAccount returns every fingerprint the account has
// acknowledged in the current tenant, ordered for deterministic responses.
func (r *TimetableConflictAckRepository) ListFingerprintsByAccount(ctx context.Context, accountID int64) ([]string, error) {
	fingerprints := make([]string, 0)
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(tableExprConflictAcksAsAck).
		ColumnExpr(`"conflict_ack".fingerprint`).
		Where(conflictAcksAccountFilterExpr, accountID).
		OrderExpr(`"conflict_ack".fingerprint ASC`)

	query = base.WithTenantFilter(ctx, query, "conflict_ack")

	if err := query.Scan(ctx, &fingerprints); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list conflict ack fingerprints", Err: err}
	}
	return fingerprints, nil
}

// Acknowledge idempotently records the fingerprint for the account. The
// ON CONFLICT DO NOTHING rides on the (tenant_id, account_id, fingerprint)
// unique constraint, so double-clicks and concurrent tabs cannot fail.
//
// Every actual insert then prunes the account's oldest rows beyond
// schedule.MaxConflictAcksPerAccount — fingerprints are opaque and cannot be
// verified against a live conflict, so this cap is what keeps a
// SchedulesRead holder from accumulating unbounded rows (#2151 review).
// Idempotent re-acks skip the prune: they cannot grow the set.
func (r *TimetableConflictAckRepository) Acknowledge(ctx context.Context, accountID int64, fingerprint string) error {
	ack := &schedule.TimetableConflictAck{
		AccountID:   accountID,
		Fingerprint: fingerprint,
	}
	if err := ack.Validate(); err != nil {
		return err
	}
	base.EnsureTenantID(ctx, ack)
	res, err := base.GetDB(ctx, r.db).NewInsert().
		Model(ack).
		ModelTableExpr(tableScheduleConflictAcks).
		On("CONFLICT (tenant_id, account_id, fingerprint) DO NOTHING").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "acknowledge conflict", Err: err}
	}
	if rows, err := res.RowsAffected(); err != nil || rows == 0 {
		return nil // duplicate ack — the set did not grow, nothing to prune
	}
	return r.pruneOldest(ctx, accountID)
}

// pruneOldest deletes the account's acknowledgements beyond the newest
// MaxConflictAcksPerAccount, ordered by creation time (id as tiebreaker).
// Dropping oldest-first is lossless in practice: old acks reference old
// conflicts, and a changed conflict stops matching its ack anyway.
func (r *TimetableConflictAckRepository) pruneOldest(ctx context.Context, accountID int64) error {
	keep := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(tableExprConflictAcksAsAck).
		ColumnExpr(`"conflict_ack".id`).
		Where(conflictAcksAccountFilterExpr, accountID).
		OrderExpr(`"conflict_ack".created_at DESC, "conflict_ack".id DESC`).
		Limit(schedule.MaxConflictAcksPerAccount)
	keep = base.WithTenantFilter(ctx, keep, "conflict_ack")

	prune := base.GetDB(ctx, r.db).NewDelete().
		Model((*schedule.TimetableConflictAck)(nil)).
		ModelTableExpr(tableExprConflictAcksAsAck).
		Where(conflictAcksAccountFilterExpr, accountID).
		Where(`"conflict_ack".id NOT IN (?)`, keep)
	prune = base.WithTenantFilter(ctx, prune, "conflict_ack")

	if _, err := prune.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "prune conflict acks", Err: err}
	}
	return nil
}

// Unacknowledge removes the fingerprint for the account. Unknown fingerprints
// are a no-op — the user's goal state ("not acknowledged") is already true.
func (r *TimetableConflictAckRepository) Unacknowledge(ctx context.Context, accountID int64, fingerprint string) error {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*schedule.TimetableConflictAck)(nil)).
		ModelTableExpr(tableExprConflictAcksAsAck).
		Where(conflictAcksAccountFilterExpr, accountID).
		Where(`"conflict_ack".fingerprint = ?`, fingerprint)

	query = base.WithTenantFilter(ctx, query, "conflict_ack")

	if _, err := query.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "unacknowledge conflict", Err: err}
	}
	return nil
}
