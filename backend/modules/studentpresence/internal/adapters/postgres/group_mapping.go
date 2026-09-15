package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Store) AddGroupToCombination(ctx context.Context, combinedID, groupID int64) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	started := time.Now()
	result, err := db.NewRaw(`INSERT INTO active.group_mappings (tenant_id,active_combined_group_id,active_group_id)
 VALUES (?,?,?) ON CONFLICT (tenant_id,active_combined_group_id,active_group_id) DO NOTHING`, tenantID, combinedID, groupID).Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("add group to combination: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	return stats, err
}
func (s *Store) RemoveGroupFromCombination(ctx context.Context, combinedID, groupID int64) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	started := time.Now()
	result, err := db.NewDelete().Table("active.group_mappings").Where("tenant_id = ?", tenantID).
		Where("active_combined_group_id = ? AND active_group_id = ?", combinedID, groupID).Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("remove group from combination: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	return stats, err
}

func (s *Store) ListGroupMappings(ctx context.Context, filter ports.GroupMappingFilter) ([]ports.GroupMapping, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	rows := []ports.GroupMapping{}
	query := db.NewSelect().Table("active.group_mappings").Column("id", "tenant_id", "created_at", "updated_at", "active_combined_group_id", "active_group_id").Where("tenant_id = ?", tenantID).Order("id")
	if filter.CombinedGroupID != nil {
		query = query.Where("active_combined_group_id = ?", *filter.CombinedGroupID)
	}
	if filter.ActiveGroupID != nil {
		query = query.Where("active_group_id = ?", *filter.ActiveGroupID)
	}
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats := ports.Stats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("list group mappings: %w", err)
	}
	return rows, stats, nil
}

func (s *Store) RecordGroupMapping(ctx context.Context, combinedID, groupID int64) (ports.GroupMapping, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.GroupMapping{}, ports.Stats{}, err
	}
	var row ports.GroupMapping
	started := time.Now()
	err = db.NewRaw(`INSERT INTO active.group_mappings (tenant_id,active_combined_group_id,active_group_id)
 VALUES (?,?,?) RETURNING id,tenant_id,created_at,updated_at,active_combined_group_id,active_group_id`, tenantID, combinedID, groupID).Scan(ctx, &row)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return ports.GroupMapping{}, stats, fmt.Errorf("record group mapping: %w", err)
	}
	stats.Rows = 1
	return row, stats, nil
}
func (s *Store) DeleteGroupMapping(ctx context.Context, id int64) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	started := time.Now()
	result, err := db.NewDelete().Table("active.group_mappings").Where("tenant_id = ? AND id = ?", tenantID, id).Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("delete group mapping: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	if err != nil {
		return stats, err
	}
	if stats.Rows == 0 {
		return stats, fmt.Errorf("delete group mapping: %w", sql.ErrNoRows)
	}
	return stats, nil
}
