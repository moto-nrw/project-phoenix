package presenceservice

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application/presence"
	activeModels "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
)

// LockExceptionDay serializes status-day writes against Care Plan's
// exception-day writers for one student and date.
type LockExceptionDay = func(context.Context, int64, string) error

// presenceEngine is the application service plus the reads it offers beyond
// the retained Service interface.
type presenceEngine interface {
	presence.Service
	EnsureOpenRoomSession(ctx context.Context, roomID, activityID int64) (*activeModels.Group, error)
	MoveStudentsToOpenRoomSessionAuthorized(ctx context.Context, studentIDs []int64, roomSessionID int64, auth presence.StudentMoveAuthorization) (*presence.StudentMoveResult, error)
	GetActiveGroupVisitsWithDisplayForGroups(ctx context.Context, groupIDs []int64) ([]*presence.VisitWithStudentDisplay, error)
	ResolveSchulhofRoomColor(ctx context.Context) *string
}

// presenceFacade publishes the application service through the public
// facades. Methods whose signatures already use public values are promoted;
// the session reads and writes below translate the retained session rows.
type presenceFacade struct {
	presenceEngine
	engine presence.Service
}

var _ studentpresence.Presence = (*presenceFacade)(nil)

func newPresenceFacade(service presence.Service) *presenceFacade {
	engine, ok := service.(presenceEngine)
	if !ok {
		panic(fmt.Sprintf("student presence compose: %T does not provide the presence reads", service))
	}
	return &presenceFacade{presenceEngine: engine, engine: service}
}

func (p *presenceFacade) GetActiveGroup(ctx context.Context, id int64) (*studentpresence.SessionDetail, error) {
	group, err := p.presenceEngine.GetActiveGroup(ctx, id)
	return sessionDetail(group), err
}

func (p *presenceFacade) GetActiveGroupsByIDs(ctx context.Context, groupIDs []int64) (map[int64]*studentpresence.SessionDetail, error) {
	groups, err := p.presenceEngine.GetActiveGroupsByIDs(ctx, groupIDs)
	if groups == nil {
		return nil, err
	}
	result := make(map[int64]*studentpresence.SessionDetail, len(groups))
	for id, group := range groups {
		result[id] = sessionDetail(group)
	}
	return result, err
}

func (p *presenceFacade) GetUnclaimedActiveGroups(ctx context.Context) ([]*studentpresence.SessionDetail, error) {
	groups, err := p.presenceEngine.GetUnclaimedActiveGroups(ctx)
	return SessionDetails(groups), err
}

func (p *presenceFacade) GetRoomsByIDs(ctx context.Context, ids []int64) ([]*studentpresence.SessionRoomSummary, error) {
	rooms, err := p.presenceEngine.GetRoomsByIDs(ctx, ids)
	if rooms == nil {
		return nil, err
	}
	result := make([]*studentpresence.SessionRoomSummary, len(rooms))
	for i, room := range rooms {
		result[i] = sessionRoom(room)
	}
	return result, err
}

func (p *presenceFacade) StartActivitySessionWithSupervisors(ctx context.Context, activityID, deviceID int64, supervisorIDs []int64, roomID *int64) (*studentpresence.SessionDetail, error) {
	group, err := p.presenceEngine.StartActivitySessionWithSupervisors(ctx, activityID, deviceID, supervisorIDs, roomID)
	return sessionDetail(group), err
}

func (p *presenceFacade) ForceStartActivitySessionWithSupervisors(ctx context.Context, activityID, deviceID int64, supervisorIDs []int64, roomID *int64) (*studentpresence.SessionDetail, error) {
	group, err := p.presenceEngine.ForceStartActivitySessionWithSupervisors(ctx, activityID, deviceID, supervisorIDs, roomID)
	return sessionDetail(group), err
}

func (p *presenceFacade) CheckActivityConflict(ctx context.Context, activityID, deviceID int64) (*studentpresence.ActivityConflictInfo, error) {
	info, err := p.presenceEngine.CheckActivityConflict(ctx, activityID, deviceID)
	if info == nil {
		return nil, err
	}
	result := &studentpresence.ActivityConflictInfo{
		HasConflict: info.HasConflict, ConflictingDevice: info.ConflictingDevice,
		ConflictMessage: info.ConflictMessage, CanOverride: info.CanOverride,
	}
	if info.ConflictingGroup != nil {
		group := liveGroup(info.ConflictingGroup)
		result.ConflictingGroup = &group
	}
	return result, err
}

func (p *presenceFacade) GetDeviceCurrentSession(ctx context.Context, deviceID int64) (*studentpresence.SessionDetail, error) {
	group, err := p.presenceEngine.GetDeviceCurrentSession(ctx, deviceID)
	return sessionDetail(group), err
}

func (p *presenceFacade) FindDeviceActiveGroupInRoom(ctx context.Context, roomID, deviceID int64) (*studentpresence.SessionDetail, error) {
	group, err := p.presenceEngine.FindDeviceActiveGroupInRoom(ctx, roomID, deviceID)
	return sessionDetail(group), err
}

func (p *presenceFacade) UpdateActiveGroupSupervisors(ctx context.Context, activeGroupID int64, supervisorIDs []int64) (*studentpresence.SessionDetail, error) {
	group, err := p.presenceEngine.UpdateActiveGroupSupervisors(ctx, activeGroupID, supervisorIDs)
	return sessionDetail(group), err
}

func (p *presenceFacade) EnsureOpenRoomSession(ctx context.Context, roomID, activityID int64) (*studentpresence.SessionDetail, error) {
	group, err := p.presenceEngine.EnsureOpenRoomSession(ctx, roomID, activityID)
	return sessionDetail(group), err
}

// CreateActiveGroup stores the session and copies the stored identity,
// timestamps and defaults back into group.
func (p *presenceFacade) CreateActiveGroup(ctx context.Context, group *studentpresence.LiveGroup) error {
	if group == nil {
		return p.presenceEngine.CreateActiveGroup(ctx, nil)
	}
	row := groupRow(*group)
	if err := p.presenceEngine.CreateActiveGroup(ctx, row); err != nil {
		return err
	}
	*group = liveGroup(row)
	return nil
}

// CreateGroupSupervisor stores the supervision and copies the stored identity
// and timestamps back into supervision. A malformed date is invalid data.
func (p *presenceFacade) CreateGroupSupervisor(ctx context.Context, supervision *studentpresence.GroupSupervision) error {
	if supervision == nil {
		return p.presenceEngine.CreateGroupSupervisor(ctx, nil)
	}
	row, err := supervisorRow(*supervision)
	if err != nil {
		return err
	}
	if err := p.presenceEngine.CreateGroupSupervisor(ctx, row); err != nil {
		return err
	}
	*supervision = groupSupervision(row)
	return nil
}

func liveGroup(group *activeModels.Group) studentpresence.LiveGroup {
	return studentpresence.LiveGroup{
		ID: group.ID, TenantID: group.TenantID, CreatedAt: group.CreatedAt, UpdatedAt: group.UpdatedAt,
		StartTime: group.StartTime, LastActivity: group.LastActivity, EndTime: group.EndTime,
		TimeoutMinutes: group.TimeoutMinutes, ActivityGroupID: group.GroupID, DeviceID: group.DeviceID, RoomID: group.RoomID,
	}
}

func groupRow(group studentpresence.LiveGroup) *activeModels.Group {
	row := &activeModels.Group{
		StartTime:      group.StartTime,
		EndTime:        group.EndTime,
		LastActivity:   group.LastActivity,
		TimeoutMinutes: group.TimeoutMinutes,
		GroupID:        group.ActivityGroupID,
		DeviceID:       group.DeviceID,
		RoomID:         group.RoomID,
	}
	row.ID, row.CreatedAt, row.UpdatedAt = group.ID, group.CreatedAt, group.UpdatedAt
	row.SetTenantID(group.TenantID)
	return row
}

// SessionDetails projects retained session rows onto the public session
// detail. It bridges the caller-context session read that still loads the
// rows itself until the session repository moves behind the owner (#3422).
func SessionDetails(groups []*activeModels.Group) []*studentpresence.SessionDetail {
	if groups == nil {
		return nil
	}
	result := make([]*studentpresence.SessionDetail, len(groups))
	for i, group := range groups {
		result[i] = sessionDetail(group)
	}
	return result
}

func sessionDetail(group *activeModels.Group) *studentpresence.SessionDetail {
	if group == nil {
		return nil
	}
	detail := &studentpresence.SessionDetail{
		ID: group.ID, TenantID: group.TenantID, CreatedAt: group.CreatedAt, UpdatedAt: group.UpdatedAt,
		StartTime: group.StartTime, LastActivity: group.LastActivity, EndTime: group.EndTime,
		TimeoutMinutes: group.TimeoutMinutes, ActivityGroupID: group.GroupID, DeviceID: group.DeviceID, RoomID: group.RoomID,
		Room: sessionRoom(group.Room),
	}
	if activity := group.ActualGroup; activity != nil {
		detail.Activity = &studentpresence.SessionActivitySummary{
			ID: activity.ID, Name: activity.Name, MaxParticipants: activity.MaxParticipants,
			PlannedRoomID: activity.PlannedRoomID, IsSystem: activity.IsSystem, IsOpen: activity.IsOpen,
		}
	}
	if group.Supervisors != nil {
		detail.Supervisors = make([]studentpresence.GroupSupervision, 0, len(group.Supervisors))
		for _, supervisor := range group.Supervisors {
			if supervisor != nil {
				detail.Supervisors = append(detail.Supervisors, groupSupervision(supervisor))
			}
		}
	}
	return detail
}

func sessionRoom(room *activeModels.SessionRoom) *studentpresence.SessionRoomSummary {
	if room == nil {
		return nil
	}
	return &studentpresence.SessionRoomSummary{ID: room.ID, Name: room.Name, Building: room.Building, Category: room.Category, Color: room.Color}
}

func groupSupervision(row *activeModels.GroupSupervisor) studentpresence.GroupSupervision {
	result := studentpresence.GroupSupervision{
		ID: row.ID, TenantID: row.TenantID, GroupID: row.GroupID, StaffID: row.StaffID,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Role: row.Role, StartDate: row.StartDate.String(),
	}
	if row.EndDate != nil {
		end := row.EndDate.String()
		result.EndDate = &end
	}
	return result
}

func supervisorRow(row studentpresence.GroupSupervision) (*activeModels.GroupSupervisor, error) {
	start, err := timezone.ParseDate(row.StartDate)
	if err != nil {
		return nil, &studentpresence.OperationError{Op: "ParseSupervisionDate", Err: studentpresence.ErrInvalidData}
	}
	supervisor := &activeModels.GroupSupervisor{StaffID: row.StaffID, GroupID: row.GroupID, Role: row.Role, StartDate: start}
	supervisor.ID, supervisor.CreatedAt, supervisor.UpdatedAt = row.ID, row.CreatedAt, row.UpdatedAt
	supervisor.SetTenantID(row.TenantID)
	if row.EndDate != nil {
		end, err := timezone.ParseDate(*row.EndDate)
		if err != nil {
			return nil, &studentpresence.OperationError{Op: "ParseSupervisionDate", Err: studentpresence.ErrInvalidData}
		}
		supervisor.EndDate = &end
	}
	return supervisor, nil
}
