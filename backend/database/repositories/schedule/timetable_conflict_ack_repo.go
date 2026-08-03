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
func (r *TimetableConflictAckRepository) Acknowledge(ctx context.Context, accountID int64, fingerprint string) error {
	ack := &schedule.TimetableConflictAck{
		AccountID:   accountID,
		Fingerprint: fingerprint,
	}
	if err := ack.Validate(); err != nil {
		return err
	}
	base.EnsureTenantID(ctx, ack)
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(ack).
		ModelTableExpr(tableScheduleConflictAcks).
		On("CONFLICT (tenant_id, account_id, fingerprint) DO NOTHING").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "acknowledge conflict", Err: err}
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
