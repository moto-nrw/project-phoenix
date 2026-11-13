package postgres

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	"github.com/moto-ogs/project-phoenix/modules/timetable/internal/domain"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// CountRoomReferences counts, in one statement, the activities, templates
// and blocks of the caller's school that reference roomID (#6845). A block
// is upcoming from today on while it is planned or running.
func (s *Store) CountRoomReferences(ctx context.Context, roomID int64, today calendar.Date) (timetable.RoomReferences, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return timetable.RoomReferences{}, domain.OperationStats{}, err
	}
	var row struct {
		ActivityGroups         int `bun:"activity_groups"`
		ArchivedActivityGroups int `bun:"archived_activity_groups"`
		UpcomingBlocks         int `bun:"upcoming_blocks"`
		PastBlocks             int `bun:"past_blocks"`
	}
	query := db.NewSelect().
		ColumnExpr(`(SELECT COUNT(*) FROM activities.groups AS g
			WHERE g.tenant_id = ?0 AND g.planned_room_id = ?1 AND g.archived_at IS NULL)::int AS activity_groups`, tenantID, roomID).
		ColumnExpr(`(SELECT COUNT(*) FROM activities.groups AS g
			WHERE g.tenant_id = ?0 AND g.planned_room_id = ?1 AND g.archived_at IS NOT NULL)::int AS archived_activity_groups`, tenantID, roomID).
		ColumnExpr(`(SELECT COUNT(*) FROM schedule.activity_instances AS i
			WHERE i.tenant_id = ?0 AND i.room_id = ?1 AND i.date >= ?2 AND i.status IN (?3, ?4))::int AS upcoming_blocks`,
			tenantID, roomID, today, timetable.InstanceStatusPlanned, timetable.InstanceStatusActive).
		ColumnExpr(`(SELECT COUNT(*) FROM schedule.activity_instances AS i
			WHERE i.tenant_id = ?0 AND i.room_id = ?1 AND NOT (i.date >= ?2 AND i.status IN (?3, ?4)))::int AS past_blocks`,
			tenantID, roomID, today, timetable.InstanceStatusPlanned, timetable.InstanceStatusActive)
	stats, err := scanAllInto(ctx, query, &row, "count room references")
	if err != nil {
		return timetable.RoomReferences{}, stats, err
	}
	stats.Rows = 1
	return timetable.RoomReferences{
		ActivityGroups: row.ActivityGroups, ArchivedActivityGroups: row.ArchivedActivityGroups,
		UpcomingBlocks: row.UpcomingBlocks, PastBlocks: row.PastBlocks,
	}, stats, nil
}
