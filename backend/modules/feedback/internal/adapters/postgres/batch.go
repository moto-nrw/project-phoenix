package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/feedback/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
		ids := make([]int64, len(batch))
		started := time.Now()
		err := db.NewSelect().
			ColumnExpr("nextval(pg_get_serial_sequence('feedback.entries', 'id'))").
			TableExpr("generate_series(1, ?) AS input_position", len(batch)).
			OrderExpr("input_position").
			Scan(ctx, &ids)
		stats.Queries++
		stats.StatementDuration += time.Since(started)
		if err != nil {
			return nil, stats, fmt.Errorf("feedback postgres: reserve entry IDs: %w", err)
		}
		for i, entry := range batch {
			values[i] = row{ID: ids[i], TenantID: tenantID, Value: entry.Value, Day: entry.Day, Time: calendar.NormalizeWallClock(entry.Time), StudentID: entry.StudentID, IsMensaFeedback: entry.IsMensaFeedback}
		}
		returned := make([]row, 0, len(values))
		started = time.Now()
		err = db.NewInsert().Model(&values).ModelTableExpr(`feedback.entries`).Returning("*").Scan(ctx, &returned)
		stats.Queries++
		stats.StatementDuration += time.Since(started)
		if err != nil {
			return nil, stats, fmt.Errorf("feedback postgres: insert entry: %w", err)
		}
		stats.Rows += int64(len(values))
		ordered, err := orderBatchResults(values, returned)
		if err != nil {
			return nil, stats, err
		}
		result = append(result, ordered...)
	}
	return result, stats, nil
}

func orderBatchResults(values, returned []row) ([]domain.Entry, error) {
	returnedByID := make(map[int64]row, len(returned))
	for _, value := range returned {
		if _, exists := returnedByID[value.ID]; exists {
			return nil, fmt.Errorf("feedback postgres: duplicate returned entry ID %d", value.ID)
		}
		returnedByID[value.ID] = value
	}
	result := make([]domain.Entry, 0, len(values))
	for _, value := range values {
		returned, ok := returnedByID[value.ID]
		if !ok {
			return nil, fmt.Errorf("feedback postgres: missing returned entry ID %d", value.ID)
		}
		result = append(result, toDomain(returned))
	}
	return result, nil
}
