package postgres

import (
	"context"

	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/uptrace/bun"
)

// ListRoomOccupancy batch-loads every open-group aggregate in one query.
type roomOccupancyRow struct {
	RoomID             int64   `bun:"room_id"`
	ActivityGroupIDs   []int64 `bun:"activity_group_ids,array"`
	StudentCount       int     `bun:"student_count"`
	SupervisorStaffIDs []int64 `bun:"supervisor_staff_ids,array"`
}

func (s *Store) ListRoomOccupancy(ctx context.Context, roomIDs []int64) ([]ports.RoomOccupancy, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	if len(roomIDs) == 0 {
		return []ports.RoomOccupancy{}, ports.Stats{}, nil
	}
	rows := []roomOccupancyRow{}
	query := db.NewSelect().
		TableExpr(`active.groups AS "group"`).
		ColumnExpr(`"group".room_id`).
		ColumnExpr(`COALESCE(array_agg(DISTINCT "group".group_id ORDER BY "group".group_id) FILTER (WHERE "group".group_id IS NOT NULL), '{}'::bigint[]) AS activity_group_ids`).
		ColumnExpr(`COUNT(DISTINCT "visit".student_id) FILTER (WHERE "visit".exit_time IS NULL)::int AS student_count`).
		ColumnExpr(`COALESCE(array_agg(DISTINCT "supervisor".staff_id) FILTER (WHERE "supervisor".staff_id IS NOT NULL AND "supervisor".end_date IS NULL), '{}'::bigint[]) AS supervisor_staff_ids`).
		Join(`LEFT JOIN active.visits AS "visit" ON "visit".active_group_id = "group".id AND "visit".tenant_id = "group".tenant_id`).
		Join(`LEFT JOIN active.group_supervisors AS "supervisor" ON "supervisor".group_id = "group".id AND "supervisor".tenant_id = "group".tenant_id`).
		Where(`"group".room_id IN (?)`, bun.List(roomIDs)).
		Where(`"group".end_time IS NULL`).
		GroupExpr(`"group".room_id`).
		OrderExpr(`"group".room_id`)
	query = query.Where(`"group".tenant_id = ?`, tenantID)
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats := ports.Stats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("list room occupancy: %w", err)
	}
	result := make([]ports.RoomOccupancy, 0, len(rows))
	for _, row := range rows {
		result = append(result, ports.RoomOccupancy(row))
	}
	return result, stats, nil
}
