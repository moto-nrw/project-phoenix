package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/feedback/internal/domain"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// CreateBatch keeps all chunks in the caller's transaction. PostgreSQL may
// consume sequence values for rows after a failing FK (E3, #5789).
func (s *Store) CreateBatch(ctx context.Context, entries []domain.Entry) ([]domain.Entry, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	result := make([]domain.Entry, 0, len(entries))
	var stats domain.OperationStats
	for offset := 0; offset < len(entries); offset += 128 {
		batch := entries[offset:min(offset+128, len(entries))]
		values := make([]row, len(batch))
		for i, entry := range batch {
			values[i] = row{TenantID: tenantID, Value: entry.Value, Day: entry.Day, Time: calendar.NormalizeWallClock(entry.Time), StudentID: entry.StudentID, IsMensaFeedback: entry.IsMensaFeedback}
		}
		started := time.Now()
		err := db.NewInsert().Model(&values).ModelTableExpr(`feedback.entries`).Returning("*").Scan(ctx)
		stats.Queries++
		stats.StatementDuration += time.Since(started)
		if err != nil {
			return nil, stats, fmt.Errorf("feedback postgres: insert entry: %w", err)
		}
		stats.Rows += int64(len(values))
		for _, value := range values {
			result = append(result, toDomain(value))
		}
	}
	return result, stats, nil
}
