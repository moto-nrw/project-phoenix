package studentdirectoryview

import (
	"context"
	"fmt"
	"time"

	calendar "github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/uptrace/bun"
)

func (s *Projection) ListCareEnds(
	ctx context.Context,
	ids []int64,
) (map[int64]string, OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, OperationStats{}, err
	}
	var rows []struct {
		ID            int64          `bun:"id"`
		EnrolledUntil *calendar.Date `bun:"enrolled_until"`
	}
	query := withStudentTenant(db.NewSelect().
		TableExpr(studentSource).
		ColumnExpr(`"student".id, "student".enrolled_until`).
		Where(`"student".id IN (?)`, bun.List(ids)).
		Where(`"student".enrolled_until IS NOT NULL`), tenantID)

	stats := OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("people directory postgres: list student care ends: %w", err)
	}
	stats.Rows = int64(len(rows))
	bounds := make(map[int64]string, len(rows))
	for _, row := range rows {
		if row.EnrolledUntil != nil {
			bounds[row.ID] = row.EnrolledUntil.String()
		}
	}
	return bounds, stats, nil
}
