package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Store) RecordSupervision(ctx context.Context, input ports.GroupSupervision) (ports.GroupSupervision, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.GroupSupervision{}, ports.Stats{}, err
	}
	started := time.Now()
	if input.CreatedAt.IsZero() {
		input.CreatedAt = started
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = started
	}
	var result ports.GroupSupervision
	err = db.NewRaw(`INSERT INTO active.group_supervisors
 (tenant_id, group_id, staff_id, role, start_date, end_date, created_at, updated_at)
 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
 RETURNING id, tenant_id, group_id, staff_id, role, start_date, end_date, created_at, updated_at`,
		tenantID, input.GroupID, input.StaffID, input.Role, input.StartDate, input.EndDate, input.CreatedAt, input.UpdatedAt).Scan(ctx, &result)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return ports.GroupSupervision{}, stats, fmt.Errorf("record supervision: %w", err)
	}
	stats.Rows = 1
	return result, stats, nil
}

func (s *Store) ReviseSupervision(ctx context.Context, input ports.GroupSupervision) (ports.GroupSupervision, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.GroupSupervision{}, ports.Stats{}, err
	}
	started := time.Now()
	var result ports.GroupSupervision
	err = db.NewRaw(`UPDATE active.group_supervisors
 SET group_id = ?, staff_id = ?, role = ?, start_date = ?, end_date = ?, updated_at = ?
 WHERE tenant_id = ? AND id = ?
 RETURNING id, tenant_id, group_id, staff_id, role, start_date, end_date, created_at, updated_at`,
		input.GroupID, input.StaffID, input.Role, input.StartDate, input.EndDate, started, tenantID, input.ID).Scan(ctx, &result)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ports.GroupSupervision{}, stats, ports.ErrSupervisionNotFound
		}
		return ports.GroupSupervision{}, stats, fmt.Errorf("revise supervision: %w", err)
	}
	stats.Rows = 1
	return result, stats, nil
}

func (s *Store) RemoveSupervision(ctx context.Context, id int64) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	started := time.Now()
	result, err := db.NewDelete().Table("active.group_supervisors").Where("tenant_id = ? AND id = ?", tenantID, id).Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("remove supervision: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	return stats, err
}

func (s *Store) SetSupervisionEnd(ctx context.Context, id int64, date ports.Date, at time.Time) (int64, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, ports.Stats{}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().Table("active.group_supervisors").
		Set("end_date = ?", date).Set("updated_at = ?", at).
		Where("tenant_id = ? AND id = ?", tenantID, id).Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return 0, stats, fmt.Errorf("set supervision end: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	return stats.Rows, stats, err
}

func (s *Store) EndOpenGroupSupervisions(ctx context.Context, groupID, staffID int64, date ports.Date) (int, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, ports.Stats{}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().Table("active.group_supervisors").Set("end_date = now()").
		Where("tenant_id = ? AND group_id = ? AND staff_id = ?", tenantID, groupID, staffID).
		Where("start_date <= ? AND end_date IS NULL", date).Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return 0, stats, fmt.Errorf("end open group supervisions: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	return int(stats.Rows), stats, err
}
