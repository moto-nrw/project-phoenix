package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

func (s *studentDirectoryReads) ListDepartureModes(ctx context.Context, ids []int64) (map[int64]map[string][]string, directoryStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, directoryStats{}, err
	}
	if scope.TenantID <= 0 {
		return nil, directoryStats{}, errors.New("people directory: departure modes require a tenant")
	}
	var rows []struct {
		ID    int64               `bun:"id"`
		Modes map[string][]string `bun:"allowed_departure_modes,type:jsonb"`
	}
	stats := directoryStats{Queries: 1}
	started := time.Now()
	err = s.studentRows(db, scope).
		ColumnExpr(`"student".id, "student".allowed_departure_modes`).
		Where(`"student".tenant_id = ?`, scope.TenantID).
		Where(`"student".id IN (?)`, bun.List(ids)).
		Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("list student departure modes: %w", err)
	}
	stats.Rows = int64(len(rows))
	result := make(map[int64]map[string][]string, len(rows))
	for _, row := range rows {
		result[row.ID] = row.Modes
	}
	return result, stats, nil
}
