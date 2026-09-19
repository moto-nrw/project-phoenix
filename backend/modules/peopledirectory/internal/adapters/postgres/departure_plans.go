package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/uptrace/bun"
)

func (s *StudentStore) ListDepartureModes(ctx context.Context, ids []int64) (map[int64]map[string][]string, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return nil, domain.OperationStats{}, errors.New("people directory: departure modes require a tenant")
	}
	var rows []struct {
		ID    int64               `bun:"id"`
		Modes map[string][]string `bun:"allowed_departure_modes,type:jsonb"`
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewSelect().
		TableExpr(`users.students AS "student"`).
		ColumnExpr(`"student".id, "student".allowed_departure_modes`).
		Where(`"student".tenant_id = ?`, tenantID).
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
