package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Store) RecordGroupActivity(ctx context.Context, id int64, at time.Time) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().Table("active.groups").Set("last_activity = ?", at).Set("updated_at = ?", started).
		Where("tenant_id = ? AND id = ? AND end_time IS NULL", tenantID, id).Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("record group activity: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	if err != nil {
		return stats, err
	}
	if stats.Rows == 0 {
		return stats, fmt.Errorf("active group with id %d not found or already ended: %w", id, ports.ErrGroupNotOpen)
	}
	return stats, nil
}

func (s *Store) DeleteGroup(ctx context.Context, id int64) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	started := time.Now()
	result, err := db.NewDelete().Table("active.groups").Where("tenant_id = ?", tenantID).Where("id = ?", id).Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("delete group: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	return stats, err
}

func (s *Store) ReviseGroup(ctx context.Context, group ports.LiveGroup) (ports.LiveGroup, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.LiveGroup{}, ports.Stats{}, err
	}
	started := time.Now()
	var row liveGroupRow
	err = db.NewRaw(`UPDATE active.groups SET start_time = ?, end_time = ?, last_activity = ?,
 timeout_minutes = NULLIF(?, 0), group_id = ?, device_id = ?, room_id = ?, updated_at = ?
 WHERE id = ? AND tenant_id = ?
 RETURNING id, tenant_id, created_at, updated_at, start_time, end_time, last_activity,
 timeout_minutes, group_id, device_id, room_id`,
		group.StartTime, group.EndTime, group.LastActivity, group.TimeoutMinutes, group.ActivityGroupID,
		group.DeviceID, group.RoomID, started, group.ID, tenantID).Scan(ctx, &row)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return ports.LiveGroup{}, stats, fmt.Errorf("revise group: %w", err)
	}
	stats.Rows = 1
	return row.record(), stats, nil
}

func (s *Store) RecordGroup(ctx context.Context, group ports.LiveGroup) (ports.LiveGroup, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.LiveGroup{}, ports.Stats{}, err
	}
	started := time.Now()
	if group.CreatedAt.IsZero() {
		group.CreatedAt = started
	}
	if group.UpdatedAt.IsZero() {
		group.UpdatedAt = started
	}
	var row liveGroupRow
	err = db.NewRaw(`INSERT INTO active.groups
 (tenant_id, created_at, updated_at, start_time, end_time, last_activity, timeout_minutes, group_id, device_id, room_id)
 VALUES (?, ?, ?, ?, ?, ?, NULLIF(?, 0), ?, ?, ?)
 RETURNING id, tenant_id, created_at, updated_at, start_time, end_time, last_activity,
 timeout_minutes, group_id, device_id, room_id`,
		tenantID, group.CreatedAt, group.UpdatedAt, group.StartTime, group.EndTime, group.LastActivity,
		group.TimeoutMinutes, group.ActivityGroupID, group.DeviceID, group.RoomID).Scan(ctx, &row)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return ports.LiveGroup{}, stats, fmt.Errorf("record group: %w", err)
	}
	stats.Rows = 1
	return row.record(), stats, nil
}

func (s *Store) EndSupervisionOn(ctx context.Context, id int64, date ports.Date) (int, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, ports.Stats{}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().Table("active.group_supervisors").Set("end_date = ?", date).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Where("start_date <= ?", date).Where("(end_date IS NULL OR end_date > ?)", date).Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return 0, stats, fmt.Errorf("end supervision: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	return int(stats.Rows), stats, err
}

func (s *Store) EndStaffSupervisionsOn(ctx context.Context, id int64, date ports.Date) (int, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, ports.Stats{}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().Table("active.group_supervisors").Set("end_date = ?", date).
		Where("tenant_id = ? AND staff_id = ?", tenantID, id).
		Where("start_date <= ?", date).Where("(end_date IS NULL OR end_date > ?)", date).Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return 0, stats, fmt.Errorf("end supervision: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	return int(stats.Rows), stats, err
}
