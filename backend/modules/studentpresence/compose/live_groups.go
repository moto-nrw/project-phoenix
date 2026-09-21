package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (e engine) ListLiveGroups(ctx context.Context, ids []int64) ([]studentpresence.LiveGroup, error) {
	rows, err := e.Service.ListLiveGroups(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]studentpresence.LiveGroup, 0, len(rows))
	for _, row := range rows {
		result = append(result, liveGroupToPublic(row))
	}
	return result, nil
}

func (e engine) QueryLiveGroups(ctx context.Context, filter studentpresence.LiveGroupFilter) ([]studentpresence.LiveGroup, error) {
	rows, err := e.Service.QueryLiveGroups(ctx, ports.LiveGroupFilter{
		IDs: filter.IDs, RoomID: filter.RoomID, DeviceID: filter.DeviceID,
		EndedOnly: filter.EndedOnly, Limit: filter.Limit, Offset: filter.Offset,
		DeviceManagedOnly: filter.DeviceManagedOnly, LastActivityBefore: filter.LastActivityBefore,
		ActivityGroupIDs: filter.ActivityGroupIDs, OpenOnly: filter.OpenOnly, From: filter.From, Until: filter.Until,
	})
	if err != nil {
		return nil, err
	}
	result := make([]studentpresence.LiveGroup, 0, len(rows))
	for _, row := range rows {
		result = append(result, liveGroupToPublic(row))
	}
	return result, nil
}

func (e engine) ListSupervisedLiveGroups(ctx context.Context, staffID int64) ([]studentpresence.LiveGroup, error) {
	rows, err := e.Service.ListSupervisedLiveGroups(ctx, staffID, timezone.DateFromTime(e.now()))
	if err != nil {
		return nil, err
	}
	result := make([]studentpresence.LiveGroup, 0, len(rows))
	for _, row := range rows {
		result = append(result, liveGroupToPublic(row))
	}
	return result, nil
}

func (e engine) ListOpenLiveGroupsForActivities(ctx context.Context, ids []int64) ([]studentpresence.LiveGroup, error) {
	if ids == nil {
		ids = []int64{}
	}
	return e.QueryLiveGroups(ctx, studentpresence.LiveGroupFilter{ActivityGroupIDs: ids, OpenOnly: true})
}

func (e engine) OccupiedActivityGroupIDs(ctx context.Context, ids []int64) ([]int64, error) {
	return e.Service.OccupiedActivityGroupIDs(ctx, ids)
}

func (e engine) ListGroupSupervisions(ctx context.Context, groupID int64) ([]studentpresence.GroupSupervision, error) {
	rows, err := e.Service.ListGroupSupervisions(ctx, groupID)
	if err != nil {
		return nil, err
	}
	result := make([]studentpresence.GroupSupervision, 0, len(rows))
	for _, row := range rows {
		var endDate *string
		if row.EndDate != nil {
			value := row.EndDate.String()
			endDate = &value
		}
		result = append(result, studentpresence.GroupSupervision{
			ID: row.ID, TenantID: row.TenantID, GroupID: row.GroupID, StaffID: row.StaffID,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Role: row.Role,
			StartDate: row.StartDate.String(), EndDate: endDate,
		})
	}
	return result, nil
}

func (e engine) QueryGroupSupervisions(ctx context.Context, filter studentpresence.GroupSupervisionFilter) ([]studentpresence.GroupSupervision, error) {
	var endedBy *ports.Date
	if filter.EndedBy != nil {
		date, err := timezone.ParseDate(*filter.EndedBy)
		if err != nil {
			return nil, err
		}
		endedBy = &date
	}
	var startedBefore *ports.Date
	if filter.StartedBefore != nil {
		date, err := timezone.ParseDate(*filter.StartedBefore)
		if err != nil {
			return nil, err
		}
		startedBefore = &date
	}
	var activeOn *ports.Date
	if filter.ActiveOn != nil {
		date, err := timezone.ParseDate(*filter.ActiveOn)
		if err != nil {
			return nil, err
		}
		activeOn = &date
	}
	rows, err := e.Service.QueryGroupSupervisions(ctx, ports.GroupSupervisionFilter{StaffIDs: filter.StaffIDs, EndedBy: endedBy, Limit: filter.Limit, Offset: filter.Offset, IDs: filter.IDs, GroupIDs: filter.GroupIDs, StaffID: filter.StaffID, ForUpdate: filter.ForUpdate, ActiveOn: activeOn, OpenOnly: filter.OpenOnly, StartedBefore: startedBefore})
	if err != nil {
		return nil, err
	}
	result := make([]studentpresence.GroupSupervision, 0, len(rows))
	for _, row := range rows {
		var endDate *string
		if row.EndDate != nil {
			value := row.EndDate.String()
			endDate = &value
		}
		result = append(result, studentpresence.GroupSupervision{ID: row.ID, TenantID: row.TenantID, GroupID: row.GroupID, StaffID: row.StaffID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Role: row.Role, StartDate: row.StartDate.String(), EndDate: endDate})
	}
	return result, nil
}

func (e engine) StaffIDsWithSupervisionOn(ctx context.Context, value string) ([]int64, error) {
	date, err := timezone.ParseDate(value)
	if err != nil {
		return nil, err
	}
	return e.Service.StaffIDsWithSupervisionOn(ctx, date)
}

func (e engine) SupervisedRoomsOn(ctx context.Context, value string) ([]studentpresence.StaffRoomSupervision, error) {
	date, err := timezone.ParseDate(value)
	if err != nil {
		return nil, err
	}
	rows, err := e.Service.SupervisedRoomsOn(ctx, date)
	if err != nil {
		return nil, err
	}
	result := make([]studentpresence.StaffRoomSupervision, 0, len(rows))
	for _, row := range rows {
		result = append(result, studentpresence.StaffRoomSupervision(row))
	}
	return result, nil
}

func (e engine) EndSupervisionOn(ctx context.Context, id int64, value string) (int, error) {
	date, err := timezone.ParseDate(value)
	if err != nil {
		return 0, err
	}
	return e.Service.EndSupervisionOn(ctx, id, date)
}

func (e engine) EndStaffSupervisionsOn(ctx context.Context, id int64, value string) (int, error) {
	date, err := timezone.ParseDate(value)
	if err != nil {
		return 0, err
	}
	return e.Service.EndStaffSupervisionsOn(ctx, id, date)
}
