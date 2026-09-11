package postgres

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun"
)

const conflictAckTable = `schedule.timetable_conflict_acks`

type conflictAckRow struct {
	bun.BaseModel `bun:"table:timetable_conflict_acks,alias:conflict_ack"`
	ID            int64     `bun:"id,pk,autoincrement"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	AccountID     int64     `bun:"account_id,notnull"`
	Fingerprint   string    `bun:"fingerprint,notnull"`
}

// ListConflictAckFingerprints returns every fingerprint the account has
// acknowledged in the current tenant, ordered for deterministic responses.
func (s *Store) ListConflictAckFingerprints(ctx context.Context, accountID int64) ([]string, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	fingerprints := make([]string, 0)
	query := db.NewSelect().TableExpr(conflictAckTable+` AS "conflict_ack"`).
		ColumnExpr(`"conflict_ack".fingerprint`).
		Where(`"conflict_ack".tenant_id = ?`, tenantID).
		Where(`"conflict_ack".account_id = ?`, accountID).
		OrderExpr(`"conflict_ack".fingerprint ASC`)
	stats, err := scanAllInto(ctx, query, &fingerprints, "list conflict ack fingerprints")
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = int64(len(fingerprints))
	return fingerprints, stats, nil
}

// InsertConflictAck records the fingerprint for the account. The insert
// rides on the (tenant_id, account_id, fingerprint) unique constraint, so a
// repeated acknowledgement is reported as not inserted instead of failing.
func (s *Store) InsertConflictAck(ctx context.Context, accountID int64, fingerprint string) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	row := conflictAckRow{TenantID: tenantID, AccountID: accountID, Fingerprint: fingerprint}
	stats := domain.OperationStats{}
	rows, err := execPlannedSupervisorWrite(ctx, db.NewInsert().Model(&row).ModelTableExpr(conflictAckTable).
		On("CONFLICT (tenant_id, account_id, fingerprint) DO NOTHING"), "acknowledge conflict", &stats)
	if err != nil {
		return false, stats, err
	}
	if rows == 0 {
		stats.DuplicatePreventionConflicts++
	}
	return rows > 0, stats, nil
}

// PruneConflictAcks deletes the account's acknowledgements beyond the newest
// keep rows, ordered by creation time with the row ID as tiebreaker.
func (s *Store) PruneConflictAcks(ctx context.Context, accountID int64, keep int) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	newest := db.NewSelect().TableExpr(conflictAckTable+` AS "newest"`).
		ColumnExpr(`"newest".id`).
		Where(`"newest".tenant_id = ?`, tenantID).
		Where(`"newest".account_id = ?`, accountID).
		OrderExpr(`"newest".created_at DESC, "newest".id DESC`).
		Limit(keep)
	stats := domain.OperationStats{}
	rows, err := execPlannedSupervisorWrite(ctx, db.NewDelete().Table(conflictAckTable).
		Where("tenant_id = ?", tenantID).
		Where("account_id = ?", accountID).
		Where("id NOT IN (?)", newest), "prune conflict acks", &stats)
	return rows, stats, err
}

// DeleteConflictAck removes the fingerprint for the account. An unknown
// fingerprint affects zero rows and is not an error.
func (s *Store) DeleteConflictAck(ctx context.Context, accountID int64, fingerprint string) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execMeasuredWrite(ctx, db.NewDelete().Table(conflictAckTable).
		Where("tenant_id = ?", tenantID).
		Where("account_id = ?", accountID).
		Where("fingerprint = ?", fingerprint), "unacknowledge conflict")
}
