package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Store) UnclaimedGroups(ctx context.Context, date ports.Date) ([]ports.UnclaimedGroup, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	started := time.Now()
	rows := make([]ports.UnclaimedGroup, 0)
	err = db.NewRaw(`SELECT g.id, g.tenant_id, g.created_at, g.updated_at, g.start_time,
		g.end_time, g.last_activity, g.timeout_minutes, g.group_id, g.device_id, g.room_id
		FROM active.groups g WHERE g.tenant_id = ? AND g.end_time IS NULL
		AND NOT EXISTS (SELECT 1 FROM active.group_supervisors s WHERE s.tenant_id = g.tenant_id
		AND s.group_id = g.id AND s.start_date <= ? AND (s.end_date IS NULL OR s.end_date > ?))
		ORDER BY g.start_time DESC`, tenantID, date, date).Scan(ctx, &rows)
	stats := ports.Stats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("find unclaimed groups: %w", err)
	}
	return rows, stats, nil
}

func (s *Store) ClaimGroup(ctx context.Context, claim ports.GroupClaim) (row ports.ClaimedSupervision, stats ports.Stats, err error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return row, stats, err
	}
	started := time.Now()
	defer func() { stats.StatementDuration = time.Since(started) }()
	var endTime *time.Time
	stats.Queries++
	err = db.NewRaw(`SELECT end_time FROM active.groups WHERE tenant_id = ? AND id = ? FOR UPDATE`, tenantID, claim.GroupID).Scan(ctx, &endTime)
	if errors.Is(err, sql.ErrNoRows) {
		return row, stats, ports.ErrGroupNotFound
	}
	if err != nil {
		return row, stats, fmt.Errorf("claim group: lock lifecycle: %w", err)
	}
	if endTime != nil {
		return row, stats, ports.ErrGroupEnded
	}
	var duplicate bool
	stats.Queries++
	err = db.NewRaw(`SELECT EXISTS (SELECT 1 FROM active.group_supervisors
		WHERE tenant_id = ? AND group_id = ? AND staff_id = ? AND start_date <= ?
		AND (end_date IS NULL OR end_date > ?))`, tenantID, claim.GroupID, claim.StaffID, claim.Date, claim.Date).Scan(ctx, &duplicate)
	if err != nil {
		return row, stats, fmt.Errorf("claim group: read supervisors: %w", err)
	}
	if duplicate {
		return row, stats, ports.ErrAlreadySupervising
	}
	stats.Queries++
	err = db.NewRaw(`INSERT INTO active.group_supervisors (tenant_id, group_id, staff_id, role, start_date)
		VALUES (?, ?, ?, ?, ?) RETURNING id, tenant_id, group_id, staff_id, role, start_date, created_at, updated_at`,
		tenantID, claim.GroupID, claim.StaffID, claim.Role, claim.Date).Scan(ctx, &row)
	if err != nil {
		return row, stats, fmt.Errorf("claim group: insert supervision: %w", err)
	}
	stats.Rows = 1
	return row, stats, nil
}
