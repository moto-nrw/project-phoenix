package repositories

import (
	"context"
	"errors"
	"time"

	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/uptrace/bun"
)

type presenceGroupRecords struct{ source *studentpresence.Module }

func (presenceGroupRecords) WrapRecordError(operation string, err error) error {
	return usersRepo.WrapError(operation, err)
}

func (presenceGroupRecords) MissingRecordError(operation string) error {
	return usersRepo.NotFoundError(operation)
}

func NewPresenceGroupRecords(db *bun.DB) activeRepo.GroupRecords {
	return presenceGroupRecords{newStudentPresence(db)}
}

func (p presenceGroupRecords) ListGroupRecords(ctx context.Context, ids []int64) ([]*activeModels.Group, error) {
	rows, err := p.source.ListLiveGroups(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]*activeModels.Group, 0, len(rows))
	for _, row := range rows {
		result = append(result, legacyLiveGroup(row))
	}
	return result, nil
}
func (p presenceGroupRecords) LockGroupRecord(ctx context.Context, id int64) (*activeModels.Group, error) {
	row, err := p.source.LockGroup(ctx, id)
	if errors.Is(err, studentpresence.ErrGroupNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return legacyLiveGroup(row), nil
}
func legacyLiveGroup(row studentpresence.LiveGroup) *activeModels.Group {
	group := &activeModels.Group{StartTime: row.StartTime, EndTime: row.EndTime, LastActivity: row.LastActivity,
		TimeoutMinutes: row.TimeoutMinutes, GroupID: row.ActivityGroupID, DeviceID: row.DeviceID, RoomID: row.RoomID}
	group.ID, group.CreatedAt, group.UpdatedAt = row.ID, row.CreatedAt, row.UpdatedAt
	group.SetTenantID(row.TenantID)
	return group
}

func (p presenceGroupRecords) RecordGroupActivity(ctx context.Context, id int64, at time.Time) error {
	return p.source.RecordGroupActivity(ctx, id, at)
}

func (p presenceGroupRecords) QueryGroupRecords(ctx context.Context, filter activeRepo.GroupRecordFilter) ([]*activeModels.Group, error) {
	rows, err := p.source.QueryLiveGroups(ctx, studentpresence.LiveGroupFilter{
		RoomID: filter.RoomID, DeviceID: filter.DeviceID,
		EndedOnly: filter.EndedOnly, Limit: filter.Limit, Offset: filter.Offset,
		DeviceManagedOnly: filter.DeviceManagedOnly, LastActivityBefore: filter.LastActivityBefore, ActivityGroupIDs: filter.ActivityGroupIDs,
		OpenOnly: filter.OpenOnly, From: filter.From, Until: filter.Until,
	})
	if err != nil {
		return nil, err
	}
	result := make([]*activeModels.Group, 0, len(rows))
	for _, row := range rows {
		result = append(result, legacyLiveGroup(row))
	}
	return result, nil
}

func (p presenceGroupRecords) OccupiedActivityGroupIDs(ctx context.Context, ids []int64) ([]int64, error) {
	return p.source.OccupiedActivityGroupIDs(ctx, ids)
}

func (p presenceGroupRecords) ListGroupSupervisions(ctx context.Context, id int64) ([]activeRepo.GroupSupervisionRecord, error) {
	rows, err := p.source.ListGroupSupervisions(ctx, id)
	if err != nil {
		return nil, err
	}
	result := make([]activeRepo.GroupSupervisionRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, activeRepo.GroupSupervisionRecord(row))
	}
	return result, nil
}

func (p presenceGroupRecords) DeleteGroupRecord(ctx context.Context, id int64) error {
	return p.source.DeleteGroup(ctx, id)
}

func (p presenceGroupRecords) UpdateGroupRecord(ctx context.Context, group *activeModels.Group) error {
	row, err := p.source.ReviseGroup(ctx, studentpresence.LiveGroup{
		ID: group.ID, StartTime: group.StartTime, EndTime: group.EndTime, LastActivity: group.LastActivity,
		TimeoutMinutes: group.TimeoutMinutes, ActivityGroupID: group.GroupID, DeviceID: group.DeviceID, RoomID: group.RoomID,
	})
	if err != nil {
		return err
	}
	group.CreatedAt, group.UpdatedAt = row.CreatedAt, row.UpdatedAt
	group.SetTenantID(row.TenantID)
	return nil
}

func (p presenceGroupRecords) CreateGroupRecord(ctx context.Context, group *activeModels.Group) error {
	row, err := p.source.RecordGroup(ctx, studentpresence.LiveGroup{
		ID: group.ID, CreatedAt: group.CreatedAt, UpdatedAt: group.UpdatedAt,
		StartTime: group.StartTime, EndTime: group.EndTime, LastActivity: group.LastActivity,
		TimeoutMinutes: group.TimeoutMinutes, ActivityGroupID: group.GroupID, DeviceID: group.DeviceID, RoomID: group.RoomID,
	})
	if err != nil {
		return err
	}
	group.ID, group.CreatedAt, group.UpdatedAt = row.ID, row.CreatedAt, row.UpdatedAt
	group.SetTenantID(row.TenantID)
	return nil
}

func (p presenceGroupRecords) QueryGroupSupervisions(ctx context.Context, filter activeRepo.SupervisionFilter) ([]activeRepo.GroupSupervisionRecord, error) {
	rows, err := p.source.QueryGroupSupervisions(ctx, studentpresence.GroupSupervisionFilter{StaffIDs: filter.StaffIDs, EndedBy: filter.EndedBy, Limit: filter.Limit, Offset: filter.Offset, IDs: filter.IDs, GroupIDs: filter.GroupIDs, StaffID: filter.StaffID, ForUpdate: filter.ForUpdate, ActiveOn: filter.ActiveOn, OpenOnly: filter.OpenOnly, StartedBefore: filter.StartedBefore})
	if err != nil {
		return nil, err
	}
	result := make([]activeRepo.GroupSupervisionRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, activeRepo.GroupSupervisionRecord(row))
	}
	return result, nil
}

func NewPresenceSupervisionRecords(db *bun.DB) activeRepo.SupervisionRecords {
	return presenceGroupRecords{newStudentPresence(db)}
}

func (p presenceGroupRecords) StaffIDsWithSupervisionOn(ctx context.Context, date string) ([]int64, error) {
	return p.source.StaffIDsWithSupervisionOn(ctx, date)
}

func (p presenceGroupRecords) SupervisedRoomsOn(ctx context.Context, date string) ([]activeRepo.StaffRoomSupervision, error) {
	rows, err := p.source.SupervisedRoomsOn(ctx, date)
	if err != nil {
		return nil, err
	}
	result := make([]activeRepo.StaffRoomSupervision, 0, len(rows))
	for _, row := range rows {
		result = append(result, activeRepo.StaffRoomSupervision(row))
	}
	return result, nil
}

func (p presenceGroupRecords) EndSupervisionOn(ctx context.Context, id int64, date string) (int, error) {
	return p.source.EndSupervisionOn(ctx, id, date)
}

func (p presenceGroupRecords) EndStaffSupervisionsOn(ctx context.Context, id int64, date string) (int, error) {
	return p.source.EndStaffSupervisionsOn(ctx, id, date)
}

func supervisionRecord(row *activeModels.GroupSupervisor) studentpresence.GroupSupervision {
	result := studentpresence.GroupSupervision{ID: row.ID, StaffID: row.StaffID, GroupID: row.GroupID,
		Role: row.Role, StartDate: row.StartDate.String(), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
	if row.EndDate != nil {
		value := row.EndDate.String()
		result.EndDate = &value
	}
	return result
}

func (p presenceGroupRecords) CreateSupervisionRecord(ctx context.Context, row *activeModels.GroupSupervisor) error {
	result, err := p.source.RecordSupervision(ctx, supervisionRecord(row))
	if err != nil {
		return err
	}
	row.ID, row.CreatedAt, row.UpdatedAt = result.ID, result.CreatedAt, result.UpdatedAt
	row.SetTenantID(result.TenantID)
	return nil
}

func (p presenceGroupRecords) UpdateSupervisionRecord(ctx context.Context, row *activeModels.GroupSupervisor) (bool, error) {
	result, err := p.source.ReviseSupervision(ctx, supervisionRecord(row))
	if errors.Is(err, studentpresence.ErrSupervisionNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	row.CreatedAt, row.UpdatedAt = result.CreatedAt, result.UpdatedAt
	row.SetTenantID(result.TenantID)
	return true, nil
}

func (p presenceGroupRecords) DeleteSupervisionRecord(ctx context.Context, id int64) error {
	return p.source.RemoveSupervision(ctx, id)
}

func (p presenceGroupRecords) SetSupervisionEnd(ctx context.Context, id int64, date string, at time.Time) (int64, error) {
	return p.source.SetSupervisionEnd(ctx, id, date, at)
}

func (p presenceGroupRecords) EndOpenGroupSupervisions(ctx context.Context, groupID, staffID int64, date string) (int, error) {
	return p.source.EndOpenGroupSupervisions(ctx, groupID, staffID, date)
}
