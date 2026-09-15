// backend/database/repositories/active/group.go
package active

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
)

// GroupRepository implements active.GroupRepository interface
type GroupRepository struct {
	activities GroupActivityDirectory
	rooms      RoomDirectory
	devices    DeviceDirectory
	records    GroupRecords
}

type GroupRecordFilter struct {
	EndedOnly          bool
	Limit, Offset      int
	DeviceManagedOnly  bool
	LastActivityBefore *time.Time
	RoomID, DeviceID   *int64
	ActivityGroupIDs   []int64
	OpenOnly           bool
	From, Until        *time.Time
}

type GroupSupervisionRecord struct {
	ID, TenantID, GroupID, StaffID int64
	CreatedAt, UpdatedAt           time.Time
	Role                           string
	StartDate                      string
	EndDate                        *string
}

type GroupActivityDirectory interface {
	FindByIDs(context.Context, []int64) ([]*active.SessionActivity, error)
}

type GroupRecords interface {
	RecordErrors
	CreateGroupRecord(context.Context, *active.Group) error
	UpdateGroupRecord(context.Context, *active.Group) error
	DeleteGroupRecord(context.Context, int64) error
	ListGroupSupervisions(context.Context, int64) ([]GroupSupervisionRecord, error)
	OccupiedActivityGroupIDs(context.Context, []int64) ([]int64, error)
	QueryGroupRecords(context.Context, GroupRecordFilter) ([]*active.Group, error)
	RecordGroupActivity(context.Context, int64, time.Time) error
	ListGroupRecords(context.Context, []int64) ([]*active.Group, error)
	LockGroupRecord(context.Context, int64) (*active.Group, error)
}

// BindRoomDirectory installs the Facilities directory the group reads
// resolve rooms through (#2665).
func (r *GroupRepository) BindRoomDirectory(rooms RoomDirectory) {
	r.rooms = rooms
}

// NewGroupRepository creates a new GroupRepository
// NewGroupRepository builds the session repository. devices is the Device
// Fleet directory session reads resolve their device through (#2676); a nil
// directory makes every device-bearing read fail loudly instead of falling
// back to a join this package no longer owns.
func NewGroupRepository(devices DeviceDirectory, records GroupRecords, activityDirectory GroupActivityDirectory) active.GroupRepository {
	return &GroupRepository{
		activities: activityDirectory,
		devices:    devices,
		records:    records,
	}
}

func (r *GroupRepository) FindActiveByRoomID(ctx context.Context, roomID int64) ([]*active.Group, error) {
	return r.records.QueryGroupRecords(ctx, GroupRecordFilter{RoomID: &roomID, OpenOnly: true})
}

func (r *GroupRepository) FindActiveByRoomIDAndDeviceID(ctx context.Context, roomID, deviceID int64) (*active.Group, error) {
	rows, err := r.records.QueryGroupRecords(ctx, GroupRecordFilter{RoomID: &roomID, DeviceID: &deviceID, OpenOnly: true})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

func (r *GroupRepository) FindActiveByGroupID(ctx context.Context, groupID int64) ([]*active.Group, error) {
	return r.FindActiveByGroupIDs(ctx, []int64{groupID})
}

func (r *GroupRepository) FindActiveByGroupIDs(ctx context.Context, groupIDs []int64) ([]*active.Group, error) {
	if len(groupIDs) == 0 {
		return []*active.Group{}, nil
	}
	return r.records.QueryGroupRecords(ctx, GroupRecordFilter{ActivityGroupIDs: groupIDs, OpenOnly: true})
}

func (r *GroupRepository) FindByTimeRange(ctx context.Context, start, end time.Time) ([]*active.Group, error) {
	return r.records.QueryGroupRecords(ctx, GroupRecordFilter{From: &start, Until: &end})
}

// FindByID reads a tenant-owned group through StudentPresence.
func (r *GroupRepository) FindByID(ctx context.Context, id int64) (*active.Group, error) {
	groups, err := r.records.ListGroupRecords(ctx, []int64{id})
	if err != nil {
		return nil, r.records.WrapRecordError("find by id", err)
	}
	if len(groups) == 0 {
		return nil, r.records.MissingRecordError("find by id")
	}
	return groups[0], nil
}

// FindWithSupervisors retrieves a group with its associated supervisors.
func (r *GroupRepository) FindWithSupervisors(ctx context.Context, id int64) (*active.Group, error) {
	groups, err := r.records.ListGroupRecords(ctx, []int64{id})
	if err != nil {
		return nil, r.records.WrapRecordError("find with supervisors - group", err)
	}
	if len(groups) == 0 {
		return nil, r.records.MissingRecordError("find with supervisors - group")
	}
	group := groups[0]
	supervisors, err := r.records.ListGroupSupervisions(ctx, id)
	if err != nil {
		return nil, r.records.WrapRecordError("find with supervisors - supervisors", err)
	}
	group.Supervisors, err = legacyGroupSupervisions(supervisors)
	if err != nil {
		return nil, err
	}
	return group, nil
}

// Activity session conflict detection methods

// FindActiveByDeviceID finds the current active session for a specific device
func (r *GroupRepository) FindActiveByDeviceID(ctx context.Context, deviceID int64) (*active.Group, error) {
	groups, err := r.records.QueryGroupRecords(ctx, GroupRecordFilter{DeviceID: &deviceID, OpenOnly: true})
	if err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return nil, nil
	}
	return groups[0], nil
}

// FindActiveByDeviceIDWithNames finds the current active session for a device with owner-provided activity and room names
func (r *GroupRepository) FindActiveByDeviceIDWithNames(ctx context.Context, deviceID int64) (*active.Group, error) {
	session, err := r.FindActiveByDeviceID(ctx, deviceID)
	if err != nil || session == nil {
		return nil, err
	}

	activityNames := map[int64]string{}
	if session.GroupID != nil {
		rows, readErr := r.queryActivityGroupsByIDs(ctx, []int64{*session.GroupID})
		err = readErr
		for _, row := range rows {
			activityNames[row.ID] = row.Name
		}
		if err != nil {
			return nil, r.records.WrapRecordError("find active by device ID with names", err)
		}
	}
	if session.GroupID != nil && activityNames[*session.GroupID] != "" {
		session.ActualGroup = &active.SessionActivity{
			ID:   *session.GroupID,
			Name: activityNames[*session.GroupID],
		}
	}

	// Add room info if available: the room owner resolves the name the
	// former LEFT JOIN projected (#2665).
	rooms, err := roomsByID(ctx, r.rooms, []int64{session.RoomID})
	if err != nil {
		return nil, r.records.WrapRecordError("find active by device ID with names", err)
	}
	if room, ok := rooms[session.RoomID]; ok && room.Name != "" {
		session.Room = &active.SessionRoom{
			ID:   session.RoomID,
			Name: room.Name,
		}
	}

	return session, nil
}

// CheckRoomConflict reports another open group occupying the room.
func (r *GroupRepository) CheckRoomConflict(ctx context.Context, roomID int64, excludeGroupID int64) (bool, *active.Group, error) {
	groups, err := r.records.QueryGroupRecords(ctx, GroupRecordFilter{RoomID: &roomID, OpenOnly: true})
	if err != nil {
		return false, nil, err
	}
	for _, group := range groups {
		if excludeGroupID <= 0 || group.ID != excludeGroupID {
			return true, group, nil
		}
	}
	return false, nil, nil
}

// UpdateLastActivity updates the last activity timestamp for a session
func (r *GroupRepository) UpdateLastActivity(ctx context.Context, id int64, at time.Time) error {
	return r.records.RecordGroupActivity(ctx, id, at)
}

// FindActiveSessionsOlderThan finds active sessions that haven't had activity since the cutoff time.
// Devices are resolved through the Device Fleet owner so the session check can
// see their online status.
func (r *GroupRepository) FindActiveSessionsOlderThan(ctx context.Context, cutoffTime time.Time) ([]*active.Group, error) {
	groups, err := r.records.QueryGroupRecords(ctx, GroupRecordFilter{
		OpenOnly: true, DeviceManagedOnly: true, LastActivityBefore: &cutoffTime,
	})
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(groups, func(a, b *active.Group) int { return a.LastActivity.Compare(b.LastActivity) })
	if err := attachDevices(ctx, r.devices, groups); err != nil {
		return nil, r.records.WrapRecordError("find active sessions older than", err)
	}

	return groups, nil
}

// FindActiveGroups finds all open groups, oldest start first.
func (r *GroupRepository) FindActiveGroups(ctx context.Context) ([]*active.Group, error) {
	groups, err := r.records.QueryGroupRecords(ctx, GroupRecordFilter{OpenOnly: true})
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(groups, func(a, b *active.Group) int { return a.StartTime.Compare(b.StartTime) })
	return groups, nil
}

// FindByIDs finds active groups by their IDs in a single query
func (r *GroupRepository) FindByIDs(ctx context.Context, ids []int64) (map[int64]*active.Group, error) {
	if len(ids) == 0 {
		return make(map[int64]*active.Group), nil
	}

	groups, err := r.queryGroupsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	if err := r.loadRoomsForGroups(ctx, groups); err != nil {
		return nil, err
	}
	_, activityGroupIDs := collectRelationIDs(groups)
	if err := r.loadAndAssignActivityGroups(ctx, groups, activityGroupIDs); err != nil {
		return nil, err
	}

	return groupsToMap(groups), nil
}

// FindByIDForUpdate finds a group by ID and locks it for the current
// transaction. Returns nil when the row is not visible in the current tenant.
func (r *GroupRepository) FindByIDForUpdate(ctx context.Context, id int64) (*active.Group, error) {
	return r.records.LockGroupRecord(ctx, id)
}

func (r *GroupRepository) queryGroupsByIDs(ctx context.Context, ids []int64) ([]*active.Group, error) {
	return r.records.ListGroupRecords(ctx, ids)
}

// loadRoomsForGroups batch loads rooms for the given groups
func (r *GroupRepository) loadRoomsForGroups(ctx context.Context, groups []*active.Group) error {
	roomIDs := collectRoomIDs(groups)
	if len(roomIDs) == 0 {
		return nil
	}

	rooms, err := r.queryRoomsByIDs(ctx, roomIDs, "find group rooms by IDs")
	if err != nil {
		return err
	}

	assignRoomsToGroups(groups, rooms)
	return nil
}

// collectRoomIDs extracts unique room IDs from groups
func collectRoomIDs(groups []*active.Group) []int64 {
	seen := make(map[int64]struct{})
	ids := make([]int64, 0)
	for _, g := range groups {
		if g.RoomID > 0 {
			if _, exists := seen[g.RoomID]; !exists {
				seen[g.RoomID] = struct{}{}
				ids = append(ids, g.RoomID)
			}
		}
	}
	return ids
}

// queryRoomsByIDs fetches rooms by their IDs through the Facilities owner.
func (r *GroupRepository) queryRoomsByIDs(ctx context.Context, ids []int64, op string) ([]*active.SessionRoom, error) {
	found, err := roomsByID(ctx, r.rooms, ids)
	if err != nil {
		return nil, r.records.WrapRecordError(op, err)
	}
	rooms := make([]*active.SessionRoom, 0, len(found))
	for _, room := range found {
		rooms = append(rooms, room.legacy())
	}
	return rooms, nil
}

// assignRoomsToGroups assigns rooms to groups based on room ID
func assignRoomsToGroups(groups []*active.Group, rooms []*active.SessionRoom) {
	roomMap := make(map[int64]*active.SessionRoom, len(rooms))
	for _, room := range rooms {
		roomMap[room.ID] = room
	}
	for _, g := range groups {
		if room, ok := roomMap[g.RoomID]; ok {
			g.Room = room
		}
	}
}

// groupsToMap converts a slice of groups to a map keyed by ID
func groupsToMap(groups []*active.Group) map[int64]*active.Group {
	result := make(map[int64]*active.Group, len(groups))
	for _, g := range groups {
		result[g.ID] = g
	}
	return result
}

// collectRelationIDs extracts unique room and group IDs. Spontaneous sessions
// (g.GroupID == nil) have no parent template — they are skipped here rather
// than materialising as spurious zero IDs.
func collectRelationIDs(groups []*active.Group) (roomIDs, groupIDs []int64) {
	roomSeen := make(map[int64]bool)
	groupSeen := make(map[int64]bool)

	for _, g := range groups {
		if g.RoomID > 0 && !roomSeen[g.RoomID] {
			roomIDs = append(roomIDs, g.RoomID)
			roomSeen[g.RoomID] = true
		}
		if templateID, ok := g.TemplateID(); ok && !groupSeen[templateID] {
			groupIDs = append(groupIDs, templateID)
			groupSeen[templateID] = true
		}
	}
	return roomIDs, groupIDs
}

// loadAndAssignActivityGroups loads activity groups and assigns them
func (r *GroupRepository) loadAndAssignActivityGroups(ctx context.Context, groups []*active.Group, groupIDs []int64) error {
	if len(groupIDs) == 0 {
		return nil
	}

	activityGroups, err := r.queryActivityGroupsByIDs(ctx, groupIDs)
	if err != nil {
		return err
	}

	assignActivityGroupsToGroups(groups, activityGroups)
	return nil
}

// queryActivityGroupsByIDs fetches activity groups by their IDs
func (r *GroupRepository) queryActivityGroupsByIDs(ctx context.Context, ids []int64) ([]*active.SessionActivity, error) {
	if r.activities == nil {
		return nil, errors.New("group activity directory is required")
	}
	groups, err := r.activities.FindByIDs(ctx, ids)
	if err != nil {
		return nil, r.records.WrapRecordError("batch load activity groups for unclaimed groups", err)
	}
	return groups, nil
}

// assignActivityGroupsToGroups assigns activity groups to active groups
func assignActivityGroupsToGroups(groups []*active.Group, activityGroups []*active.SessionActivity) {
	agMap := make(map[int64]*active.SessionActivity, len(activityGroups))
	for _, ag := range activityGroups {
		agMap[ag.ID] = ag
	}
	for _, g := range groups {
		if templateID, ok := g.TemplateID(); ok {
			if ag, found := agMap[templateID]; found {
				g.ActualGroup = ag
			}
		}
	}
}

// GetOccupiedActivityGroupIDs returns a set of activity group IDs that currently have active sessions
// This is optimized for checking activity occupancy without fetching full group records
func (r *GroupRepository) GetOccupiedActivityGroupIDs(ctx context.Context, groupIDs []int64) (map[int64]bool, error) {
	if len(groupIDs) == 0 {
		return make(map[int64]bool), nil
	}

	occupiedGroupIDs, err := r.records.OccupiedActivityGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}

	// Convert to set for O(1) lookup
	result := make(map[int64]bool, len(occupiedGroupIDs))
	for _, id := range occupiedGroupIDs {
		result[id] = true
	}

	return result, nil
}

func (r *GroupRepository) Delete(ctx context.Context, id int64) error {
	return r.records.DeleteGroupRecord(ctx, id)
}

func (r *GroupRepository) Update(ctx context.Context, group *active.Group) error {
	if group == nil {
		return errors.New("group cannot be nil or zero value")
	}
	if err := group.Validate(); err != nil {
		return err
	}
	return r.records.UpdateGroupRecord(ctx, group)
}

func (r *GroupRepository) Create(ctx context.Context, group *active.Group) error {
	if group == nil {
		return errors.New("group cannot be nil or zero value")
	}
	if err := group.Validate(); err != nil {
		return err
	}
	return r.records.CreateGroupRecord(ctx, group)
}

func legacyGroupSupervisions(supervisors []GroupSupervisionRecord) ([]*active.GroupSupervisor, error) {
	result := make([]*active.GroupSupervisor, 0, len(supervisors))
	for _, row := range supervisors {
		start, err := timezone.ParseDate(row.StartDate)
		if err != nil {
			return nil, err
		}
		var end *timezone.Date
		if row.EndDate != nil {
			value, err := timezone.ParseDate(*row.EndDate)
			if err != nil {
				return nil, err
			}
			end = &value
		}
		record := &active.GroupSupervisor{GroupID: row.GroupID, StaffID: row.StaffID, Role: row.Role, StartDate: start, EndDate: end}
		record.ID, record.CreatedAt, record.UpdatedAt = row.ID, row.CreatedAt, row.UpdatedAt
		record.SetTenantID(row.TenantID)
		result = append(result, record)
	}
	return result, nil
}
