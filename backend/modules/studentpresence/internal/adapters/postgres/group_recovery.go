package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/uptrace/bun"
)

func (s *Store) LockOpenSupervisors(ctx context.Context, groupID int64) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	var ids []int64
	started := time.Now()
	err = db.NewSelect().TableExpr(`active.group_supervisors AS "group_supervisor"`).
		ColumnExpr(`"group_supervisor".id`).
		Where(`"group_supervisor".tenant_id = ?`, tenantID).
		Where(`"group_supervisor".group_id = ?`, groupID).
		Where(`"group_supervisor".end_date IS NULL`).
		OrderExpr(`"group_supervisor".id ASC`).For("UPDATE").Scan(ctx, &ids)
	stats := ports.Stats{Queries: 1, Rows: int64(len(ids)), StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("lock open supervisors: %w", err)
	}
	return stats, nil
}

func (s *Store) LockSupervisors(ctx context.Context, ids []int64) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	if len(ids) == 0 {
		return ports.Stats{}, nil
	}
	var locked []int64
	started := time.Now()
	err = db.NewSelect().TableExpr(`active.group_supervisors AS "group_supervisor"`).
		ColumnExpr(`"group_supervisor".id`).
		Where(`"group_supervisor".tenant_id = ?`, tenantID).
		Where(`"group_supervisor".id IN (?)`, bun.List(ids)).
		OrderExpr(`"group_supervisor".id ASC`).For("UPDATE").Scan(ctx, &locked)
	stats := ports.Stats{Queries: 1, Rows: int64(len(locked)), StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("lock supervisors: %w", err)
	}
	return stats, nil
}

// RestoreGroup reopens exactly one ended active group. The tenant filter keeps
// a foreign snapshot ID from matching, so the caller sees a row mismatch.
func (s *Store) RestoreGroup(ctx context.Context, groupID int64, now time.Time) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().Table("active.groups").Set("end_time = NULL").Set("last_activity = ?", now).
		Where("tenant_id = ?", tenantID).Where("id = ? AND end_time IS NOT NULL", groupID).Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("restore active group: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("restore active group: %w", err)
	}
	if stats.Rows != 1 {
		return stats, fmt.Errorf("restore active group: snapshot mismatch for active group: expected 1 rows, updated %d", stats.Rows)
	}
	return stats, nil
}

func (s *Store) RestoreSupervisors(ctx context.Context, ids []int64) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	if len(ids) == 0 {
		return ports.Stats{}, nil
	}
	started := time.Now()
	result, err := db.NewUpdate().Table("active.group_supervisors").Set("end_date = NULL").
		Where("tenant_id = ?", tenantID).Where("id IN (?) AND end_date IS NOT NULL", bun.List(ids)).Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("restore supervisors: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("restore supervisors: %w", err)
	}
	if stats.Rows != int64(len(ids)) {
		return stats, fmt.Errorf("restore supervisors: snapshot mismatch for supervisors: expected %d rows, updated %d", len(ids), stats.Rows)
	}
	return stats, nil
}
