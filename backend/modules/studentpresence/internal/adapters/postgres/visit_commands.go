package postgres

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/uptrace/bun"
)

func (s *Store) CloseGroupVisits(ctx context.Context, ids []int64) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	if len(ids) == 0 {
		return ports.Stats{}, nil
	}
	started := time.Now()
	// The app's check-in clock may be ahead of the DB clock. Preserve the
	// existing zero-duration clamp so one skewed interval cannot abort a batch.
	result, err := db.NewUpdate().Table("active.visits").
		Set("exit_time = GREATEST(now(), entry_time)").Where("tenant_id = ?", tenantID).
		Where("active_group_id IN (?)", bun.List(ids)).Where("exit_time IS NULL").Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, err
	}
	stats.Rows, err = result.RowsAffected()
	return stats, err
}

func (s *Store) TransferOpenVisits(ctx context.Context, from, to int64) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().Table("active.visits").
		Set("active_group_id = ?", to).Set("updated_at = ?", time.Now()).
		Where("tenant_id = ?", tenantID).Where("active_group_id = ?", from).
		Where("exit_time IS NULL").Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, err
	}
	stats.Rows, err = result.RowsAffected()
	return stats, err
}

func (s *Store) TransferRecentDeviceVisits(ctx context.Context, to, deviceID int64) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().Table("active.visits").Set("active_group_id = ?", to).
		Where("tenant_id = ?", tenantID).
		Where(`active_group_id IN (
			SELECT id FROM active.groups WHERE tenant_id = ? AND device_id = ?
			AND end_time IS NOT NULL AND end_time > NOW() - INTERVAL '1 hour'
		) AND exit_time IS NULL`, tenantID, deviceID).Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, err
	}
	stats.Rows, err = result.RowsAffected()
	return stats, err
}

func (s *Store) DeleteCompletedVisitsBefore(ctx context.Context, studentID int64, cutoff time.Time) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	started := time.Now()
	result, err := db.NewDelete().Table("active.visits").Where("tenant_id = ?", tenantID).
		Where("student_id = ?", studentID).Where("created_at < ?", cutoff).
		Where("exit_time IS NOT NULL").Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, err
	}
	stats.Rows, err = result.RowsAffected()
	return stats, err
}
