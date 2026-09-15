package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Store) ListCombinedGroups(ctx context.Context, filter ports.CombinedGroupFilter) ([]ports.CombinedGroup, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	rows := []ports.CombinedGroup{}
	q := db.NewSelect().Table("active.combined_groups").Column("id", "tenant_id", "created_at", "updated_at", "start_time", "end_time").Where("tenant_id = ?", tenantID).Order("id")
	if filter.ID != nil {
		q = q.Where("id = ?", *filter.ID)
	}
	if filter.Active != nil {
		if *filter.Active {
			q = q.Where("(end_time IS NULL OR end_time > NOW())")
		} else {
			q = q.Where("end_time IS NOT NULL AND end_time <= NOW()")
		}
	}
	if filter.Limit > 0 {
		q = q.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		q = q.Offset(filter.Offset)
	}
	if filter.OpenOnly {
		q = q.Where("end_time IS NULL")
	}
	if filter.From != nil {
		q = q.Where("(end_time IS NULL OR end_time >= ?)", *filter.From)
	}
	if filter.Until != nil {
		q = q.Where("start_time <= ?", *filter.Until)
	}
	started := time.Now()
	err = q.Scan(ctx, &rows)
	stats := ports.Stats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("list combined groups: %w", err)
	}
	return rows, stats, nil
}
func (s *Store) EndCombination(ctx context.Context, id int64, at time.Time) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().Table("active.combined_groups").Set("end_time = ?", at).Where("tenant_id = ? AND id = ? AND end_time IS NULL", tenantID, id).Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("end combination: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	if err != nil {
		return stats, err
	}
	if stats.Rows == 0 {
		return stats, fmt.Errorf("end combination: %w", sql.ErrNoRows)
	}
	return stats, nil
}

func (s *Store) GetCombinedGroup(ctx context.Context, id int64) (ports.CombinedGroup, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.CombinedGroup{}, ports.Stats{}, err
	}
	var row ports.CombinedGroup
	started := time.Now()
	err = db.NewSelect().Table("active.combined_groups").Column("id", "tenant_id", "created_at", "updated_at", "start_time", "end_time").Where("tenant_id = ? AND id = ?", tenantID, id).Scan(ctx, &row)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if errors.Is(err, sql.ErrNoRows) {
		return ports.CombinedGroup{}, stats, ports.ErrCombinedGroupNotFound
	}
	if err != nil {
		return ports.CombinedGroup{}, stats, fmt.Errorf("get combined group: %w", err)
	}
	stats.Rows = 1
	return row, stats, nil
}
func (s *Store) RecordCombination(ctx context.Context, start time.Time, end *time.Time) (ports.CombinedGroup, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.CombinedGroup{}, ports.Stats{}, err
	}
	var row ports.CombinedGroup
	started := time.Now()
	err = db.NewRaw("INSERT INTO active.combined_groups (tenant_id,start_time,end_time) VALUES (?,?,?) RETURNING id,tenant_id,created_at,updated_at,start_time,end_time", tenantID, start, end).Scan(ctx, &row)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return ports.CombinedGroup{}, stats, fmt.Errorf("record combination: %w", err)
	}
	stats.Rows = 1
	return row, stats, nil
}
func (s *Store) ReviseCombination(ctx context.Context, id int64, start time.Time, end *time.Time) (ports.CombinedGroup, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.CombinedGroup{}, ports.Stats{}, err
	}
	var row ports.CombinedGroup
	started := time.Now()
	err = db.NewRaw("UPDATE active.combined_groups SET start_time = ?,end_time = ?,updated_at = NOW() WHERE tenant_id = ? AND id = ? RETURNING id,tenant_id,created_at,updated_at,start_time,end_time", start, end, tenantID, id).Scan(ctx, &row)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return ports.CombinedGroup{}, stats, fmt.Errorf("revise combination: %w", err)
	}
	stats.Rows = 1
	return row, stats, nil
}
func (s *Store) DeleteCombination(ctx context.Context, id int64) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	started := time.Now()
	result, err := db.NewDelete().Table("active.combined_groups").Where("tenant_id = ? AND id = ?", tenantID, id).Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("delete combination: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	return stats, err
}
