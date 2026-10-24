package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

type sessionRepository interface {
	ports.ActiveGroupRepository
	RoomDirectory() ports.RoomDirectory
}

// SessionRecords publishes the room sessions and their supervision through
// studentpresence.SessionRecords and studentpresence.SupervisionRecords. The
// embedded repositories serve the operations whose values already are the
// owner's public ones; every method declared here translates the rows. The
// session repository also hands out the room directory installed at
// construction, so the composition root can rebind the room owner behind it
// (#2665).
type SessionRecords struct {
	sessionRepository
	ports.GroupSupervisorRepository
	staff ports.SupervisionStaffDirectory
}

var (
	_ studentpresence.SessionRecords     = (*SessionRecords)(nil)
	_ studentpresence.SupervisionRecords = (*SessionRecords)(nil)
)

func NewSessionRecords(groups sessionRepository, supervisors ports.GroupSupervisorRepository, staff ports.SupervisionStaffDirectory) *SessionRecords {
	return &SessionRecords{sessionRepository: groups, GroupSupervisorRepository: supervisors, staff: staff}
}

// SessionRepositories returns the session and supervision repositories the
// presence services run on, for records NewSessionRecords built.
func SessionRepositories(records studentpresence.SessionRecords) (ports.ActiveGroupRepository, ports.GroupSupervisorRepository) {
	value, ok := records.(*SessionRecords)
	if !ok {
		panic(fmt.Sprintf("student presence: %T are not the composed session records", records))
	}
	return value.sessionRepository, value.GroupSupervisorRepository
}

// SupervisionStaff returns the staff directory installed at construction, so
// the composition root can bind School Membership and the People Directory
// behind it once they are observed.
func (r *SessionRecords) SupervisionStaff() ports.SupervisionStaffDirectory {
	return r.staff
}

func (r *SessionRecords) CreateSession(ctx context.Context, group *studentpresence.LiveGroup) error {
	if group == nil {
		return r.sessionRepository.Create(ctx, nil)
	}
	row := activeGroupRow(*group)
	if err := r.sessionRepository.Create(ctx, row); err != nil {
		return err
	}
	*group = *livePointer(row)
	return nil
}

func (r *SessionRecords) FindSession(ctx context.Context, id int64) (*studentpresence.LiveGroup, error) {
	group, err := r.sessionRepository.FindByID(ctx, id)
	return livePointer(group), err
}

func (r *SessionRecords) FindByIDForUpdate(ctx context.Context, id int64) (*studentpresence.LiveGroup, error) {
	group, err := r.sessionRepository.FindByIDForUpdate(ctx, id)
	return livePointer(group), err
}

func (r *SessionRecords) FindByIDs(ctx context.Context, ids []int64) (map[int64]*studentpresence.SessionDetail, error) {
	groups, err := r.sessionRepository.FindByIDs(ctx, ids)
	if groups == nil {
		return nil, err
	}
	result := make(map[int64]*studentpresence.SessionDetail, len(groups))
	for id, group := range groups {
		result[id] = sessionDetailOf(group)
	}
	return result, err
}

func (r *SessionRecords) FindActiveByRoomID(ctx context.Context, roomID int64) ([]*studentpresence.LiveGroup, error) {
	groups, err := r.sessionRepository.FindActiveByRoomID(ctx, roomID)
	return livePointers(groups), err
}

func (r *SessionRecords) FindActiveByDeviceIDWithNames(ctx context.Context, deviceID int64) (*studentpresence.SessionDetail, error) {
	group, err := r.sessionRepository.FindActiveByDeviceIDWithNames(ctx, deviceID)
	return sessionDetailOf(group), err
}

func (r *SessionRecords) FindActiveGroups(ctx context.Context) ([]*studentpresence.LiveGroup, error) {
	groups, err := r.sessionRepository.FindActiveGroups(ctx)
	return livePointers(groups), err
}

func (r *SessionRecords) CheckRoomConflict(ctx context.Context, roomID, excludeGroupID int64) (bool, *studentpresence.LiveGroup, error) {
	conflict, group, err := r.sessionRepository.CheckRoomConflict(ctx, roomID, excludeGroupID)
	return conflict, livePointer(group), err
}

// SessionRows returns the sessions in the order of ids, skipping unknown
// ones, as the JSON rows the caller-context routes render.
func (r *SessionRecords) SessionRows(ctx context.Context, ids []int64) (any, error) {
	result := make([]*ports.ActiveGroup, 0, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	byID, err := r.sessionRepository.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if session := byID[id]; session != nil {
			result = append(result, session)
		}
	}
	return result, nil
}

func (r *SessionRecords) CreateSupervision(ctx context.Context, supervision *studentpresence.GroupSupervision) error {
	if supervision == nil {
		return r.GroupSupervisorRepository.Create(ctx, nil)
	}
	row, err := supervisorRow(*supervision)
	if err != nil {
		return err
	}
	if err := r.GroupSupervisorRepository.Create(ctx, row); err != nil {
		return err
	}
	*supervision = supervisionOf(row)
	return nil
}

func (r *SessionRecords) FindByActiveGroupID(ctx context.Context, activeGroupID int64, activeOnly bool) ([]*studentpresence.StaffedSupervision, error) {
	rows, err := r.GroupSupervisorRepository.FindByActiveGroupID(ctx, activeGroupID, activeOnly)
	return staffedSupervisions(rows), err
}

func (r *SessionRecords) FindByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, activeOnly bool) ([]*studentpresence.StaffedSupervision, error) {
	rows, err := r.GroupSupervisorRepository.FindByActiveGroupIDs(ctx, activeGroupIDs, activeOnly)
	return staffedSupervisions(rows), err
}

func (r *SessionRecords) FindActiveByStaffID(ctx context.Context, staffID int64) ([]*studentpresence.GroupSupervision, error) {
	rows, err := r.GroupSupervisorRepository.FindActiveByStaffID(ctx, staffID)
	if rows == nil {
		return nil, err
	}
	result := make([]*studentpresence.GroupSupervision, 0, len(rows))
	for _, row := range rows {
		value := supervisionOf(row)
		result = append(result, &value)
	}
	return result, err
}

func (r *SessionRecords) ListActiveSupervisionBlockers(ctx context.Context, staffID int64) ([]studentpresence.SupervisionBlocker, error) {
	rows, err := r.GroupSupervisorRepository.ListActiveSupervisionBlockers(ctx, staffID)
	if rows == nil {
		return nil, err
	}
	result := make([]studentpresence.SupervisionBlocker, 0, len(rows))
	for _, row := range rows {
		result = append(result, studentpresence.SupervisionBlocker(row))
	}
	return result, err
}

func livePointer(group *ports.ActiveGroup) *studentpresence.LiveGroup {
	if group == nil {
		return nil
	}
	return &studentpresence.LiveGroup{
		ID: group.ID, TenantID: group.TenantID, CreatedAt: group.CreatedAt, UpdatedAt: group.UpdatedAt,
		StartTime: group.StartTime, LastActivity: group.LastActivity, EndTime: group.EndTime,
		TimeoutMinutes: group.TimeoutMinutes, ActivityGroupID: group.GroupID, DeviceID: group.DeviceID, RoomID: group.RoomID,
	}
}

func livePointers(groups []*ports.ActiveGroup) []*studentpresence.LiveGroup {
	if groups == nil {
		return nil
	}
	result := make([]*studentpresence.LiveGroup, len(groups))
	for i, group := range groups {
		result[i] = livePointer(group)
	}
	return result
}

func activeGroupRow(group studentpresence.LiveGroup) *ports.ActiveGroup {
	return &ports.ActiveGroup{
		ID: group.ID, TenantID: group.TenantID, CreatedAt: group.CreatedAt, UpdatedAt: group.UpdatedAt,
		StartTime: group.StartTime, EndTime: group.EndTime, LastActivity: group.LastActivity,
		TimeoutMinutes: group.TimeoutMinutes, GroupID: group.ActivityGroupID, DeviceID: group.DeviceID, RoomID: group.RoomID,
	}
}

func sessionDetailOf(group *ports.ActiveGroup) *studentpresence.SessionDetail {
	if group == nil {
		return nil
	}
	detail := &studentpresence.SessionDetail{
		ID: group.ID, TenantID: group.TenantID, CreatedAt: group.CreatedAt, UpdatedAt: group.UpdatedAt,
		StartTime: group.StartTime, LastActivity: group.LastActivity, EndTime: group.EndTime,
		TimeoutMinutes: group.TimeoutMinutes, ActivityGroupID: group.GroupID, DeviceID: group.DeviceID, RoomID: group.RoomID,
	}
	if room := group.Room; room != nil {
		detail.Room = &studentpresence.SessionRoomSummary{ID: room.ID, Name: room.Name, Building: room.Building, Category: room.Category, Color: room.Color}
	}
	if activity := group.ActualGroup; activity != nil {
		detail.Activity = &studentpresence.SessionActivitySummary{
			ID: activity.ID, Name: activity.Name, MaxParticipants: activity.MaxParticipants,
			PlannedRoomID: activity.PlannedRoomID, IsSystem: activity.IsSystem, IsOpen: activity.IsOpen,
		}
	}
	for _, supervisor := range group.Supervisors {
		if supervisor != nil {
			detail.Supervisors = append(detail.Supervisors, supervisionOf(supervisor))
		}
	}
	return detail
}

func supervisionOf(row *ports.GroupSupervisor) studentpresence.GroupSupervision {
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

func staffedSupervisions(rows []*ports.GroupSupervisor) []*studentpresence.StaffedSupervision {
	if rows == nil {
		return nil
	}
	result := make([]*studentpresence.StaffedSupervision, 0, len(rows))
	for _, row := range rows {
		staffed := &studentpresence.StaffedSupervision{GroupSupervision: supervisionOf(row)}
		if row.Staff != nil && row.Staff.Person != nil {
			staffed.StaffName = row.Staff.Person.GetFullName()
		}
		result = append(result, staffed)
	}
	return result
}

// supervisorRow parses the supervision's dates; a malformed date is invalid
// data.
func supervisorRow(row studentpresence.GroupSupervision) (*ports.GroupSupervisor, error) {
	record, err := SupervisionToRecord(row)
	if err != nil {
		return nil, &studentpresence.OperationError{Op: "ParseSupervisionDate", Err: studentpresence.ErrInvalidData}
	}
	return &ports.GroupSupervisor{
		ID: record.ID, TenantID: row.TenantID, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt,
		StaffID: record.StaffID, GroupID: record.GroupID, Role: record.Role, StartDate: record.StartDate, EndDate: record.EndDate,
	}, nil
}
