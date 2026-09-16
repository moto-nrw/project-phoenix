package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/uptrace/bun"
)

type openRoomSessionRow struct {
	ActiveGroupID      int64
	RoomID             int64
	ActivityGroupID    *int64
	DeviceID           *int64
	StartTime          time.Time
	SupervisorStaffIDs []int64 `bun:"supervisor_staff_ids,array"`
}

func (s *Store) ListOpenRoomSessions(ctx context.Context, ids []int64) ([]ports.OpenRoomSession, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	result := []ports.OpenRoomSession{}
	if len(ids) == 0 {
		return result, ports.Stats{}, nil
	}
	var rows []openRoomSessionRow
	started := time.Now()
	err = db.NewSelect().TableExpr("active.groups AS ag").
		ColumnExpr("ag.id AS active_group_id,ag.room_id,ag.group_id AS activity_group_id,ag.device_id,ag.start_time").
		ColumnExpr("ARRAY(SELECT gs.staff_id FROM active.group_supervisors gs WHERE gs.group_id = ag.id AND gs.tenant_id = ag.tenant_id AND gs.end_date IS NULL ORDER BY gs.staff_id) AS supervisor_staff_ids").
		Where("ag.tenant_id = ? AND ag.end_time IS NULL", tenantID).
		Where("ag.room_id IN (?)", bun.List(ids)).OrderExpr("ag.id").Scan(ctx, &rows)
	stats := ports.Stats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("list open room sessions: %w", err)
	}
	for _, row := range rows {
		result = append(result, ports.OpenRoomSession(row))
	}
	return result, stats, nil
}
