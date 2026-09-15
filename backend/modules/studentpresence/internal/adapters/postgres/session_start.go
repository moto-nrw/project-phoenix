package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Store) LockSessionStart(ctx context.Context, activityID int64) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	started := time.Now()
	_, err = db.ExecContext(ctx, "SELECT pg_advisory_xact_lock(?, ?)", tenantID, activityID)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("lock session start: %w", err)
	}
	return stats, nil
}
