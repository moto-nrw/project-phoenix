package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (e engine) ListVisitLocations(ctx context.Context, filter studentpresence.VisitLocationFilter) ([]studentpresence.VisitLocation, error) {
	rows, err := e.Service.ListVisitLocations(ctx, ports.VisitLocationFilter{
		VisitFilter: ports.VisitFilter(filter.VisitFilter), RunningGroupsOnly: filter.RunningGroupsOnly, LatestPerStudent: filter.LatestPerStudent,
	})
	if err != nil {
		return nil, err
	}
	result := make([]studentpresence.VisitLocation, 0, len(rows))
	for _, row := range rows {
		location := studentpresence.VisitLocation{Visit: visitToPublic(&row.Visit)}
		if row.GroupID != nil {
			location.Group = &studentpresence.VisitGroup{
				ID: *row.GroupID, RoomID: row.RoomID, CreatedAt: row.GroupCreatedAt, UpdatedAt: row.GroupUpdatedAt,
				StartTime: row.StartTime, LastActivity: row.LastActivity, EndTime: row.EndTime,
				TemplateID: row.TemplateID, DeviceID: row.DeviceID, TimeoutMinutes: row.TimeoutMinutes,
			}
		}
		result = append(result, location)
	}
	return result, nil
}

func (e engine) ListOpenVisitRooms(ctx context.Context, roomID int64) ([]studentpresence.OpenVisitRoom, error) {
	rows, err := e.Service.ListOpenVisitRooms(ctx, roomID)
	if err != nil {
		return nil, err
	}
	result := make([]studentpresence.OpenVisitRoom, 0, len(rows))
	for _, row := range rows {
		result = append(result, studentpresence.OpenVisitRoom(row))
	}
	return result, nil
}
