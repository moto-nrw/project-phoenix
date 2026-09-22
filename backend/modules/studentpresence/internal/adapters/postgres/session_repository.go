package postgres

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

// SessionRepository reads and writes room sessions through the owner's record
// store and projects the rooms, activities and devices of other owners onto
// them through their directories.
type SessionRepository struct {
	activities ports.ActivityDirectory
	rooms      ports.RoomDirectory
	devices    ports.DeviceDirectory
	records    ports.ActiveGroupRecords
	errors     ports.RecordErrors
}

// SessionDirectories are the owner queries session reads resolve their
// display relations through. A missing directory makes every read that needs
// it fail loudly instead of falling back to a join this module does not own.
type SessionDirectories struct {
	Devices    ports.DeviceDirectory
	Rooms      ports.RoomDirectory
	Activities ports.ActivityDirectory
}

// NewSessionRepository builds the session repository over the owner's record
// store, the composition root's error representation and the directories.
func NewSessionRepository(records ports.ActiveGroupRecords, errs ports.RecordErrors, directories SessionDirectories) *SessionRepository {
	return &SessionRepository{
		activities: directories.Activities,
		rooms:      directories.Rooms,
		devices:    directories.Devices,
		records:    records,
		errors:     errs,
	}
}

// RoomDirectory returns the directory installed at construction, so the
// composition root can rebind the room owner behind it once the observed
// Facilities capability exists (#2665).
func (r *SessionRepository) RoomDirectory() ports.RoomDirectory {
	return r.rooms
}

func (r *SessionRepository) query(ctx context.Context, filter ports.LiveGroupFilter) ([]*ActiveGroupRow, error) {
	rows, err := r.records.QueryLiveGroups(ctx, filter)
	if err != nil {
		return nil, err
	}
	return activeGroups(rows), nil
}

func (r *SessionRepository) first(ctx context.Context, filter ports.LiveGroupFilter) (*ActiveGroupRow, error) {
	groups, err := r.query(ctx, filter)
	if err != nil || len(groups) == 0 {
		return nil, err
	}
	return groups[0], nil
}

func (r *SessionRepository) FindActiveByRoomID(ctx context.Context, roomID int64) ([]*ActiveGroupRow, error) {
	return r.query(ctx, ports.LiveGroupFilter{RoomID: &roomID, OpenOnly: true})
}

func (r *SessionRepository) FindActiveByRoomIDAndDeviceID(ctx context.Context, roomID, deviceID int64) (*ActiveGroupRow, error) {
	return r.first(ctx, ports.LiveGroupFilter{RoomID: &roomID, DeviceID: &deviceID, OpenOnly: true})
}

func (r *SessionRepository) FindActiveByGroupID(ctx context.Context, groupID int64) ([]*ActiveGroupRow, error) {
	return r.FindActiveByGroupIDs(ctx, []int64{groupID})
}

func (r *SessionRepository) FindActiveByGroupIDs(ctx context.Context, groupIDs []int64) ([]*ActiveGroupRow, error) {
	if len(groupIDs) == 0 {
		return []*ActiveGroupRow{}, nil
	}
	return r.query(ctx, ports.LiveGroupFilter{ActivityGroupIDs: groupIDs, OpenOnly: true})
}

func (r *SessionRepository) FindByTimeRange(ctx context.Context, start, end time.Time) ([]*ActiveGroupRow, error) {
	return r.query(ctx, ports.LiveGroupFilter{From: &start, Until: &end})
}

// FindByID reads a tenant-owned group through Student Presence.
func (r *SessionRepository) FindByID(ctx context.Context, id int64) (*ActiveGroupRow, error) {
	return r.findOne(ctx, id, "find by id")
}

func (r *SessionRepository) findOne(ctx context.Context, id int64, operation string) (*ActiveGroupRow, error) {
	rows, err := r.records.ListLiveGroups(ctx, []int64{id})
	if err != nil {
		return nil, r.errors.WrapRecordError(operation, err)
	}
	if len(rows) == 0 {
		return nil, r.errors.MissingRecordError(operation)
	}
	return activeGroup(rows[0]), nil
}

// FindWithSupervisors retrieves a group with its associated supervisors.
func (r *SessionRepository) FindWithSupervisors(ctx context.Context, id int64) (*ActiveGroupRow, error) {
	group, err := r.findOne(ctx, id, "find with supervisors - group")
	if err != nil {
		return nil, err
	}
	supervisors, err := r.records.ListGroupSupervisions(ctx, id)
	if err != nil {
		return nil, r.errors.WrapRecordError("find with supervisors - supervisors", err)
	}
	group.Supervisors = groupSupervisors(supervisors)
	return group, nil
}

// FindActiveByDeviceID finds the current active session for a specific device
func (r *SessionRepository) FindActiveByDeviceID(ctx context.Context, deviceID int64) (*ActiveGroupRow, error) {
	return r.first(ctx, ports.LiveGroupFilter{DeviceID: &deviceID, OpenOnly: true})
}

// FindActiveByDeviceIDWithNames finds the current active session for a device
// with owner-provided activity and room names.
func (r *SessionRepository) FindActiveByDeviceIDWithNames(ctx context.Context, deviceID int64) (*ActiveGroupRow, error) {
	session, err := r.FindActiveByDeviceID(ctx, deviceID)
	if err != nil || session == nil {
		return nil, err
	}
	if err := r.attachActivityName(ctx, session); err != nil {
		return nil, r.errors.WrapRecordError("find active by device ID with names", err)
	}
	// The room owner resolves the name the former LEFT JOIN projected (#2665).
	rooms, err := roomsByID(ctx, r.rooms, []int64{session.RoomID})
	if err != nil {
		return nil, r.errors.WrapRecordError("find active by device ID with names", err)
	}
	if room, ok := rooms[session.RoomID]; ok && room.Name != "" {
		session.Room = &ports.SessionRoom{ID: session.RoomID, Name: room.Name}
	}
	return session, nil
}

// attachActivityName projects the template's name onto a template-backed session.
func (r *SessionRepository) attachActivityName(ctx context.Context, session *ActiveGroupRow) error {
	templateID, ok := session.TemplateID()
	if !ok {
		return nil
	}
	activities, err := r.queryActivityGroupsByIDs(ctx, []int64{templateID})
	if err != nil {
		return err
	}
	for _, activity := range activities {
		if activity.ID == templateID && activity.Name != "" {
			session.ActualGroup = &ports.SessionActivity{ID: templateID, Name: activity.Name}
		}
	}
	return nil
}

// CheckRoomConflict reports another open group occupying the room.
// Independent room stays (device-less sessions of a system activity) share
// the room with activities by design and do not count as occupancy (#3066).
func (r *SessionRepository) CheckRoomConflict(ctx context.Context, roomID int64, excludeGroupID int64) (bool, *ActiveGroupRow, error) {
	groups, err := r.query(ctx, ports.LiveGroupFilter{RoomID: &roomID, OpenOnly: true})
	if err != nil {
		return false, nil, err
	}
	occupants := slices.DeleteFunc(groups, func(group *ActiveGroupRow) bool {
		return excludeGroupID > 0 && group.ID == excludeGroupID
	})
	systemByActivity, err := r.systemActivitiesByID(ctx, deviceLessTemplateIDs(occupants))
	if err != nil {
		return false, nil, err
	}
	for _, group := range occupants {
		templateID, ok := group.TemplateID()
		if ok && group.IsIndependentRoomSession(systemByActivity[templateID]) {
			continue
		}
		return true, group, nil
	}
	return false, nil, nil
}

// deviceLessTemplateIDs lists the distinct templates of device-less sessions:
// only those can be independent room stays.
func deviceLessTemplateIDs(groups []*ActiveGroupRow) []int64 {
	ids := make([]int64, 0, len(groups))
	seen := make(map[int64]struct{}, len(groups))
	for _, group := range groups {
		templateID, ok := group.TemplateID()
		if group.DeviceID != nil || !ok {
			continue
		}
		if _, found := seen[templateID]; !found {
			seen[templateID] = struct{}{}
			ids = append(ids, templateID)
		}
	}
	return ids
}

func (r *SessionRepository) systemActivitiesByID(ctx context.Context, ids []int64) (map[int64]bool, error) {
	result := make(map[int64]bool, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	activities, err := r.queryActivityGroupsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, activity := range activities {
		if activity != nil {
			result[activity.ID] = activity.IsSystem
		}
	}
	return result, nil
}

// UpdateLastActivity updates the last activity timestamp for a session
func (r *SessionRepository) UpdateLastActivity(ctx context.Context, id int64, at time.Time) error {
	return r.records.RecordGroupActivity(ctx, id, at)
}

// FindActiveSessionsOlderThan finds active sessions that haven't had activity
// since the cutoff time. Devices are resolved through the Device Fleet owner
// so the session check can see their online status.
func (r *SessionRepository) FindActiveSessionsOlderThan(ctx context.Context, cutoffTime time.Time) ([]*ActiveGroupRow, error) {
	groups, err := r.query(ctx, ports.LiveGroupFilter{
		OpenOnly: true, DeviceManagedOnly: true, LastActivityBefore: &cutoffTime,
	})
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(groups, func(a, b *ActiveGroupRow) int { return a.LastActivity.Compare(b.LastActivity) })
	if err := attachDevices(ctx, r.devices, groups); err != nil {
		return nil, r.errors.WrapRecordError("find active sessions older than", err)
	}
	return groups, nil
}

// FindActiveGroups finds all open groups, oldest start first.
func (r *SessionRepository) FindActiveGroups(ctx context.Context) ([]*ActiveGroupRow, error) {
	groups, err := r.query(ctx, ports.LiveGroupFilter{OpenOnly: true})
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(groups, func(a, b *ActiveGroupRow) int { return a.StartTime.Compare(b.StartTime) })
	return groups, nil
}

// FindByIDs finds active groups by their IDs with their rooms and activity
// templates.
func (r *SessionRepository) FindByIDs(ctx context.Context, ids []int64) (map[int64]*ActiveGroupRow, error) {
	if len(ids) == 0 {
		return make(map[int64]*ActiveGroupRow), nil
	}
	rows, err := r.records.ListLiveGroups(ctx, ids)
	if err != nil {
		return nil, err
	}
	groups := activeGroups(rows)
	if err := r.loadRoomsForGroups(ctx, groups); err != nil {
		return nil, err
	}
	if err := r.loadAndAssignActivityGroups(ctx, groups, templateIDs(groups)); err != nil {
		return nil, err
	}
	result := make(map[int64]*ActiveGroupRow, len(groups))
	for _, group := range groups {
		result[group.ID] = group
	}
	return result, nil
}

// FindByIDForUpdate finds a group by ID and locks it for the current
// transaction. Returns nil when the row is not visible in the current tenant.
func (r *SessionRepository) FindByIDForUpdate(ctx context.Context, id int64) (*ActiveGroupRow, error) {
	row, err := r.records.LockGroup(ctx, id)
	if errors.Is(err, ports.ErrGroupNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return activeGroup(row), nil
}

// GetOccupiedActivityGroupIDs returns a set of activity group IDs that
// currently have active sessions.
func (r *SessionRepository) GetOccupiedActivityGroupIDs(ctx context.Context, groupIDs []int64) (map[int64]bool, error) {
	if len(groupIDs) == 0 {
		return make(map[int64]bool), nil
	}
	occupiedGroupIDs, err := r.records.OccupiedActivityGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]bool, len(occupiedGroupIDs))
	for _, id := range occupiedGroupIDs {
		result[id] = true
	}
	return result, nil
}

func (r *SessionRepository) Delete(ctx context.Context, id int64) error {
	return r.records.DeleteGroup(ctx, id)
}

func (r *SessionRepository) Update(ctx context.Context, group *ActiveGroupRow) error {
	if group == nil {
		return errors.New("group cannot be nil or zero value")
	}
	if err := group.Validate(); err != nil {
		return err
	}
	row, err := r.records.ReviseGroup(ctx, liveGroup(group))
	if err != nil {
		return err
	}
	group.CreatedAt, group.UpdatedAt, group.TenantID = row.CreatedAt, row.UpdatedAt, row.TenantID
	return nil
}

func (r *SessionRepository) Create(ctx context.Context, group *ActiveGroupRow) error {
	if group == nil {
		return errors.New("group cannot be nil or zero value")
	}
	if err := group.Validate(); err != nil {
		return err
	}
	row, err := r.records.RecordGroup(ctx, liveGroup(group))
	if err != nil {
		return err
	}
	group.ID, group.CreatedAt, group.UpdatedAt, group.TenantID = row.ID, row.CreatedAt, row.UpdatedAt, row.TenantID
	return nil
}

// ActiveGroupRow is the session the repository reads and writes.
type ActiveGroupRow = ports.ActiveGroup

var _ ports.ActiveGroupRepository = (*SessionRepository)(nil)

func activeGroups(rows []ports.LiveGroup) []*ActiveGroupRow {
	result := make([]*ActiveGroupRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, activeGroup(row))
	}
	return result
}

func activeGroup(row ports.LiveGroup) *ActiveGroupRow {
	return &ActiveGroupRow{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		StartTime: row.StartTime, EndTime: row.EndTime, LastActivity: row.LastActivity,
		TimeoutMinutes: row.TimeoutMinutes, GroupID: row.ActivityGroupID, DeviceID: row.DeviceID, RoomID: row.RoomID,
	}
}

func liveGroup(group *ActiveGroupRow) ports.LiveGroup {
	return ports.LiveGroup{
		ID: group.ID, CreatedAt: group.CreatedAt, UpdatedAt: group.UpdatedAt,
		StartTime: group.StartTime, EndTime: group.EndTime, LastActivity: group.LastActivity,
		TimeoutMinutes: group.TimeoutMinutes, ActivityGroupID: group.GroupID, DeviceID: group.DeviceID, RoomID: group.RoomID,
	}
}
