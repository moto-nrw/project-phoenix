package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

func (s *studentDirectoryReads) ListCareEnds(
	ctx context.Context,
	ids []int64,
) (map[int64]string, directoryStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, directoryStats{}, err
	}
	var rows []struct {
		ID            int64          `bun:"id"`
		EnrolledUntil *calendar.Date `bun:"enrolled_until"`
	}
	query := withStudentTenant(s.studentRows(db, scope).
		ColumnExpr(`"student".id, "student".enrolled_until`).
		Where(`"student".id IN (?)`, bun.List(ids)).
		Where(`"student".enrolled_until IS NOT NULL`), scope)

	stats := directoryStats{Queries: 1}
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
