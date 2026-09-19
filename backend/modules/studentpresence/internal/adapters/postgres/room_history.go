package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

type roomHistoryRow struct {
	SessionID          int64
	ActivityGroupID    *int64
	StartedAt          time.Time
	EndedAt            *time.Time
	DurationMinutes    *int
	SupervisorStaffIDs []int64 `bun:"supervisor_staff_ids,array"`
	StudentCount       int
}

func (s *Store) ListRoomSessionHistory(ctx context.Context, roomID int64, start, end time.Time, staffID *int64) ([]ports.RoomSessionHistory, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	var rows []roomHistoryRow
	query := db.NewSelect().TableExpr("active.groups AS ag").
		ColumnExpr("ag.id AS session_id,ag.group_id AS activity_group_id,ag.start_time AS started_at,ag.end_time AS ended_at").
		ColumnExpr("CASE WHEN ag.end_time IS NULL THEN NULL ELSE CAST(EXTRACT(EPOCH FROM (ag.end_time-ag.start_time))/60 AS INTEGER) END AS duration_minutes").
		ColumnExpr("ARRAY(SELECT DISTINCT gs.staff_id FROM active.group_supervisors gs WHERE gs.group_id = ag.id AND gs.tenant_id = ag.tenant_id ORDER BY gs.staff_id) AS supervisor_staff_ids").
		ColumnExpr("(SELECT COUNT(DISTINCT v.student_id) FROM active.visits v WHERE v.active_group_id = ag.id AND v.tenant_id = ag.tenant_id AND v.entry_time <= ? AND (v.exit_time IS NULL OR v.exit_time >= ?)) AS student_count", end, start).
		Where("ag.tenant_id = ? AND ag.room_id = ?", tenantID, roomID).
		Where("ag.start_time <= ? AND (ag.end_time IS NULL OR ag.end_time >= ?)", end, start).
		OrderExpr("ag.start_time DESC, ag.id DESC")
	if staffID != nil {
		query = query.Where("EXISTS (SELECT 1 FROM active.group_supervisors gs WHERE gs.group_id = ag.id AND gs.tenant_id = ag.tenant_id AND gs.staff_id = ?)", *staffID)
	}
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats := ports.Stats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("room session history: %w", err)
	}
	result := make([]ports.RoomSessionHistory, 0, len(rows))
	for _, row := range rows {
		result = append(result, ports.RoomSessionHistory(row))
	}
	return result, stats, nil
}
