package active

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/uptrace/bun"
)

// ListRoomOccupancy batch-loads every open-group aggregate in one query.
func (r *GroupRepository) ListRoomOccupancy(ctx context.Context, roomIDs []int64) ([]activeModels.RoomOccupancy, error) {
	if len(roomIDs) == 0 {
		return []activeModels.RoomOccupancy{}, nil
	}
	rows := []activeModels.RoomOccupancy{}
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`active.groups AS "group"`).
		ColumnExpr(`"group".room_id`).
		ColumnExpr(`COALESCE(array_agg(DISTINCT "group".group_id ORDER BY "group".group_id) FILTER (WHERE "group".group_id IS NOT NULL), '{}'::bigint[]) AS activity_group_ids`).
		ColumnExpr(`COUNT(DISTINCT "visit".student_id) FILTER (WHERE "visit".exit_time IS NULL)::int AS student_count`).
		ColumnExpr(`COALESCE(array_agg(DISTINCT "supervisor".staff_id) FILTER (WHERE "supervisor".staff_id IS NOT NULL AND "supervisor".end_date IS NULL), '{}'::bigint[]) AS supervisor_staff_ids`).
		Join(`LEFT JOIN active.visits AS "visit" ON "visit".active_group_id = "group".id`).
		Join(`LEFT JOIN active.group_supervisors AS "supervisor" ON "supervisor".group_id = "group".id`).
		Where(`"group".room_id IN (?)`, bun.In(roomIDs)).
		Where(`"group".end_time IS NULL`).
		GroupExpr(`"group".room_id`).
		OrderExpr(`"group".room_id`)
	query = base.WithTenantFilter(ctx, query, "group")
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}
