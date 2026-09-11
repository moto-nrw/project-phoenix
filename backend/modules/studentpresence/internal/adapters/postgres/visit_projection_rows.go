package postgres

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

type visitLocationRow struct {
	visitRow
	GroupID        *int64     `bun:"location_group_id"`
	RoomID         int64      `bun:"location_room_id"`
	GroupCreatedAt time.Time  `bun:"location_created_at"`
	GroupUpdatedAt time.Time  `bun:"location_updated_at"`
	StartTime      time.Time  `bun:"location_start_time"`
	LastActivity   time.Time  `bun:"location_last_activity"`
	EndTime        *time.Time `bun:"location_end_time"`
	TemplateID     *int64     `bun:"location_template_id"`
	DeviceID       *int64     `bun:"location_device_id"`
	TimeoutMinutes int        `bun:"location_timeout_minutes"`
}

func (row visitLocationRow) record() ports.VisitLocation {
	return ports.VisitLocation{
		Visit: *row.visitRow.record(), GroupID: row.GroupID, RoomID: row.RoomID,
		GroupCreatedAt: row.GroupCreatedAt, GroupUpdatedAt: row.GroupUpdatedAt,
		StartTime: row.StartTime, LastActivity: row.LastActivity, EndTime: row.EndTime,
		TemplateID: row.TemplateID, DeviceID: row.DeviceID, TimeoutMinutes: row.TimeoutMinutes,
	}
}

type openVisitRoomRow struct {
	StudentID int64 `bun:"student_id"`
	RoomID    int64 `bun:"room_id"`
}

type visitRetentionCountRow struct {
	StudentID int64 `bun:"student_id"`
	Count     int   `bun:"visit_count"`
}

type visitMonthCountRow struct {
	Month string `bun:"month"`
	Count int64  `bun:"count"`
}
