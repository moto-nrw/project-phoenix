package postgres

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Store) ListVisitLocations(ctx context.Context, filter ports.VisitLocationFilter) ([]ports.VisitLocation, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	rows := []visitLocationRow{}
	if (filter.IDs != nil && len(filter.IDs) == 0) || (filter.StudentIDs != nil && len(filter.StudentIDs) == 0) || (filter.ActiveGroupIDs != nil && len(filter.ActiveGroupIDs) == 0) {
		return []ports.VisitLocation{}, ports.Stats{}, nil
	}
	// DISTINCT ON requires the student key to precede entry time. Preserve
	// newest-per-student selection before applying the result limit.
	if filter.LatestPerStudent {
		filter.StudentOrder, filter.NewestFirst = true, true
	}
	query := visitQuery(db, tenantID, &rows, filter.VisitFilter).
		ColumnExpr("visit.*").
		ColumnExpr("g.id AS location_group_id, g.room_id AS location_room_id").
		ColumnExpr("g.created_at AS location_created_at, g.updated_at AS location_updated_at").
		ColumnExpr("g.start_time AS location_start_time, g.last_activity AS location_last_activity").
		ColumnExpr("g.end_time AS location_end_time, g.group_id AS location_template_id").
		ColumnExpr("g.device_id AS location_device_id, g.timeout_minutes AS location_timeout_minutes").
		Join("LEFT JOIN active.groups AS g ON g.id = visit.active_group_id AND g.tenant_id = visit.tenant_id")
	if filter.RunningGroupsOnly {
		query = query.Where("g.id IS NOT NULL AND g.end_time IS NULL")
	}
	if filter.LatestPerStudent {
		query = query.DistinctOn("visit.student_id")
	}
	started := time.Now()
	err = query.Scan(ctx)
	stats := ports.Stats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	result := make([]ports.VisitLocation, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.record())
	}
	return result, stats, err
}

func (s *Store) CountOpenVisitsInGroup(ctx context.Context, id int64) (int, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, ports.Stats{}, err
	}
	started := time.Now()
	count, err := db.NewSelect().TableExpr("active.visits AS v").Where("v.tenant_id = ?", tenantID).
		Where("v.active_group_id = ?", id).Where("v.exit_time IS NULL").Count(ctx)
	return count, ports.Stats{Queries: 1, Rows: 1, StatementDuration: time.Since(started)}, err
}

func (s *Store) CountOpenVisitsInRoom(ctx context.Context, id int64) (int, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, ports.Stats{}, err
	}
	started := time.Now()
	count, err := db.NewSelect().TableExpr("active.visits AS v").
		Join("JOIN active.groups AS g ON g.id = v.active_group_id AND g.tenant_id = v.tenant_id").
		Where("v.tenant_id = ?", tenantID).Where("g.room_id = ?", id).
		Where("g.end_time IS NULL").Where("v.exit_time IS NULL").Count(ctx)
	return count, ports.Stats{Queries: 1, Rows: 1, StatementDuration: time.Since(started)}, err
}

func (s *Store) ListOpenVisitRooms(ctx context.Context, roomID int64) ([]ports.OpenVisitRoom, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	rows := []openVisitRoomRow{}
	query := db.NewSelect().TableExpr("active.visits AS v").ColumnExpr("v.student_id, g.room_id").
		Join("JOIN active.groups AS g ON g.id = v.active_group_id AND g.tenant_id = v.tenant_id").
		Where("v.tenant_id = ?", tenantID).Where("g.end_time IS NULL").Where("v.exit_time IS NULL")
	if roomID != 0 {
		query = query.Where("g.room_id = ?", roomID)
	} else {
		query = query.Distinct()
	}
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats := ports.Stats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	result := make([]ports.OpenVisitRoom, 0, len(rows))
	for _, row := range rows {
		result = append(result, ports.OpenVisitRoom(row))
	}
	return result, stats, err
}
