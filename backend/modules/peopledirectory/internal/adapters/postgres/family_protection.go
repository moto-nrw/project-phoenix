package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/uptrace/bun"
)

// CurrentFamilyProtection selects the latest immutable event by ID, matching
// the ledger's existing ordering even when event timestamps tie.
func (s *StudentStore) CurrentFamilyProtection(ctx context.Context, ids []int64) (map[int64]bool, domain.OperationStats, error) {
	result := make(map[int64]bool, len(ids))
	if len(ids) == 0 {
		return result, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return nil, domain.OperationStats{}, errors.New("people directory: family protection requires a tenant")
	}
	var rows []struct {
		StudentID int64 `bun:"student_id"`
		Enabled   bool  `bun:"enabled"`
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewSelect().
		TableExpr(`users.student_family_protection_events AS "event"`).
		ColumnExpr(`"event".student_id, "event".enabled`).
		Where(`"event".tenant_id = ?`, tenantID).
		Where(`"event".student_id IN (?)`, bun.List(ids)).
		DistinctOn(`"event".student_id`).
		OrderExpr(`"event".student_id, "event".id DESC`).
		Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("list current family protection: %w", err)
	}
	stats.Rows = int64(len(rows))
	for _, row := range rows {
		result[row.StudentID] = row.Enabled
	}
	return result, stats, nil
}

type familyProtectionRow struct {
	bun.BaseModel  `bun:"table:student_family_protection_events,alias:event"`
	TenantID       int64  `bun:"tenant_id,notnull"`
	StudentID      int64  `bun:"student_id,notnull"`
	Enabled        bool   `bun:"enabled,notnull"`
	Reason         string `bun:"reason,notnull"`
	ActorAccountID int64  `bun:"actor_account_id,notnull"`
}

// AppendFamilyProtection writes one immutable ledger row. created_at uses
// clock_timestamp() so a protection and a sharing event written in the same
// transaction keep their real order.
func (s *StudentStore) AppendFamilyProtection(ctx context.Context, change domain.FamilyProtectionChange) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return domain.OperationStats{}, errors.New("people directory: family protection requires a tenant")
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewInsert().
		Model(&familyProtectionRow{
			TenantID: tenantID, StudentID: change.StudentID, Enabled: change.Enabled,
			Reason: change.Reason, ActorAccountID: change.ActorAccountID,
		}).
		ModelTableExpr("users.student_family_protection_events").
		Value("created_at", "clock_timestamp()").
		Value("updated_at", "clock_timestamp()").
		Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("append family protection: %w", err)
	}
	if affected, err := result.RowsAffected(); err == nil {
		stats.Rows = affected
	}
	return stats, nil
}

// LockLifecycle takes the student row FOR UPDATE and reports its lifecycle
// status, so a caller can refuse a graduate under the same lock it writes with.
func (s *StudentStore) LockLifecycle(ctx context.Context, id int64) (string, bool, domain.OperationStats, error) {
	_, tenantID, err := s.database(ctx)
	if err != nil {
		return "", false, domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return "", false, domain.OperationStats{}, errors.New("people directory: tenant is required to lock a student")
	}
	record, found, stats, err := s.FindRecord(ctx, id, "UPDATE")
	return record.Status, found, stats, err
}
