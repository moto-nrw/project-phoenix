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
