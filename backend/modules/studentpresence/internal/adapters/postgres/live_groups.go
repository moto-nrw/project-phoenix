package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/uptrace/bun"
)

func (s *Store) QueryLiveGroups(ctx context.Context, filter ports.LiveGroupFilter) ([]ports.LiveGroup, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	result := []ports.LiveGroup{}
	if (filter.IDs != nil && len(filter.IDs) == 0) || (filter.ActivityGroupIDs != nil && len(filter.ActivityGroupIDs) == 0) {
		return result, ports.Stats{}, nil
	}
	var rows []liveGroupRow
	started := time.Now()
	query := db.NewSelect().Table("active.groups").
		Column("id", "tenant_id", "created_at", "updated_at", "start_time", "last_activity", "end_time", "timeout_minutes", "group_id", "device_id", "room_id").
		Where("tenant_id = ?", tenantID)
	if filter.IDs != nil {
		query = query.Where("id IN (?)", bun.List(filter.IDs))
	}
	if filter.ActivityGroupIDs != nil {
		query = query.Where("group_id IN (?)", bun.List(filter.ActivityGroupIDs))
	}
	if filter.RoomID != nil {
		query = query.Where("room_id = ?", *filter.RoomID)
	}
	if filter.DeviceID != nil {
		query = query.Where("device_id = ?", *filter.DeviceID)
	}
	if filter.DeviceManagedOnly {
		query = query.Where("device_id IS NOT NULL")
	}
	if filter.LastActivityBefore != nil {
		query = query.Where("last_activity < ?", *filter.LastActivityBefore)
	}
	if filter.EndedOnly {
		query = query.Where("end_time IS NOT NULL")
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}
	if filter.OpenOnly {
		query = query.Where("end_time IS NULL")
	}
	if filter.From != nil {
		query = query.Where("(end_time IS NULL OR end_time >= ?)", *filter.From)
	}
	if filter.Until != nil {
		query = query.Where("start_time <= ?", *filter.Until)
	}
	err = query.Order("id").Scan(ctx, &rows)
	stats := ports.Stats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("list live groups: %w", err)
	}
	for _, row := range rows {
		result = append(result, row.record())
	}
	return result, stats, nil
}

func (s *Store) OccupiedActivityGroupIDs(ctx context.Context, ids []int64) ([]int64, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	result := []int64{}
	if len(ids) == 0 {
		return result, ports.Stats{}, nil
	}
	started := time.Now()
	err = db.NewSelect().Table("active.groups").Column("group_id").Distinct().
		Where("tenant_id = ?", tenantID).Where("end_time IS NULL").
		Where("group_id IN (?)", bun.List(ids)).Order("group_id").Scan(ctx, &result)
	stats := ports.Stats{Queries: 1, Rows: int64(len(result)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("occupied activity group IDs: %w", err)
	}
	return result, stats, nil
}

func (s *Store) QueryGroupSupervisions(ctx context.Context, filter ports.GroupSupervisionFilter) ([]ports.GroupSupervision, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	result := []ports.GroupSupervision{}
	if (filter.StaffIDs != nil && len(filter.StaffIDs) == 0) || (filter.IDs != nil && len(filter.IDs) == 0) || (filter.GroupIDs != nil && len(filter.GroupIDs) == 0) {
		return result, ports.Stats{}, nil
	}
	started := time.Now()
	query := db.NewSelect().Table("active.group_supervisors").
		Column("id", "tenant_id", "group_id", "staff_id", "created_at", "updated_at", "role", "start_date", "end_date").
		Where("tenant_id = ?", tenantID)
	if filter.IDs != nil {
		query = query.Where("id IN (?)", bun.List(filter.IDs))
	}
	if filter.GroupIDs != nil {
		query = query.Where("group_id IN (?)", bun.List(filter.GroupIDs))
	}
	if filter.StaffIDs != nil {
		query = query.Where("staff_id IN (?)", bun.List(filter.StaffIDs))
	}
	if filter.EndedBy != nil {
		query = query.Where("end_date <= ?", *filter.EndedBy)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}
	if filter.StaffID != nil {
		query = query.Where("staff_id = ?", *filter.StaffID)
	}
	if filter.OpenOnly {
		query = query.Where("end_date IS NULL")
	}
	if filter.StartedBefore != nil {
		query = query.Where("start_date < ?", *filter.StartedBefore)
	}
	if filter.ActiveOn != nil {
		query = query.Where("start_date <= ?", *filter.ActiveOn).Where("(end_date IS NULL OR end_date > ?)", *filter.ActiveOn)
	}
	if filter.ForUpdate {
		query = query.For("UPDATE")
	}
	err = query.Order("id").Scan(ctx, &result)
	stats := ports.Stats{Queries: 1, Rows: int64(len(result)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("list group supervisions: %w", err)
	}
	return result, stats, nil
}

func (s *Store) StaffIDsWithSupervisionOn(ctx context.Context, date ports.Date) ([]int64, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	result := []int64{}
	started := time.Now()
	err = db.NewSelect().Table("active.group_supervisors").Column("staff_id").Distinct().
		Where("tenant_id = ?", tenantID).
		Where("(start_date = ? OR end_date = ? OR (start_date < ? AND (end_date IS NULL OR end_date > ?)))", date, date, date, date).
		Order("staff_id").Scan(ctx, &result)
	stats := ports.Stats{Queries: 1, Rows: int64(len(result)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("staff with supervision: %w", err)
	}
	return result, stats, nil
}

func (s *Store) SupervisedRoomsOn(ctx context.Context, date ports.Date) ([]ports.StaffRoomSupervision, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	result := []ports.StaffRoomSupervision{}
	started := time.Now()
	err = db.NewSelect().TableExpr("active.group_supervisors AS supervision").
		ColumnExpr("supervision.staff_id, session.room_id").Distinct().
		Join("JOIN active.groups AS session ON session.id = supervision.group_id AND session.tenant_id = supervision.tenant_id").
		Where("supervision.tenant_id = ?", tenantID).
		Where("supervision.start_date <= ?", date).
		Where("(supervision.end_date IS NULL OR supervision.end_date > ?)", date).
		Where("session.end_time IS NULL").Order("supervision.staff_id", "session.room_id").Scan(ctx, &result)
	stats := ports.Stats{Queries: 1, Rows: int64(len(result)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("supervised rooms: %w", err)
	}
	return result, stats, nil
}
