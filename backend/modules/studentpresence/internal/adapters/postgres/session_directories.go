package postgres

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

var (
	errDeviceDirectoryRequired   = errors.New("active repositories: device directory is not bound")
	errRoomDirectoryRequired     = errors.New("active repositories: room directory is not bound")
	errActivityDirectoryRequired = errors.New("group activity directory is required")
)

// attachDevices fills ActiveGroup.Device from the Device Fleet owner. The
// retired join also required the device to belong to the session's tenant, so
// a device of another tenant leaves the relation unset exactly as before.
func attachDevices(ctx context.Context, directory ports.DeviceDirectory, groups []*ActiveGroupRow) error {
	if directory == nil {
		return errDeviceDirectoryRequired
	}
	ids := uniquePositive(deviceIDs(groups))
	if len(ids) == 0 {
		return nil
	}
	devices, err := directory.ListDevicesByID(ctx, ids)
	if err != nil {
		return err
	}
	byID := make(map[int64]ports.DirectoryDevice, len(devices))
	for _, device := range devices {
		byID[device.ID] = device
	}
	for _, group := range groups {
		if group == nil || group.DeviceID == nil {
			continue
		}
		if device, ok := byID[*group.DeviceID]; ok && device.TenantID == group.TenantID {
			group.Device = sessionDevice(device)
		}
	}
	return nil
}

func deviceIDs(groups []*ActiveGroupRow) []int64 {
	ids := make([]int64, 0, len(groups))
	for _, group := range groups {
		if group != nil && group.DeviceID != nil {
			ids = append(ids, *group.DeviceID)
		}
	}
	return ids
}

func sessionDevice(device ports.DirectoryDevice) *ports.SessionDevice {
	return &ports.SessionDevice{
		ID: device.ID, CreatedAt: device.CreatedAt, UpdatedAt: device.UpdatedAt,
		TenantID: device.TenantID, DeviceID: device.DeviceID, DeviceType: device.DeviceType,
		Name: device.Name, Status: device.Status, LastSeen: device.LastSeen,
	}
}

// uniquePositive drops non-positive and repeated IDs, keeping first-seen order.
func uniquePositive(ids []int64) []int64 {
	unique := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, found := seen[id]; found {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}

// roomsByID resolves the unique positive ids through the bound directory.
func roomsByID(ctx context.Context, directory ports.RoomDirectory, ids []int64) (map[int64]ports.DirectoryRoom, error) {
	if directory == nil {
		return nil, errRoomDirectoryRequired
	}
	unique := uniquePositive(ids)
	result := make(map[int64]ports.DirectoryRoom, len(unique))
	if len(unique) == 0 {
		return result, nil
	}
	rooms, err := directory.ListRoomsByID(ctx, unique)
	if err != nil {
		return nil, err
	}
	for _, room := range rooms {
		result[room.ID] = room
	}
	return result, nil
}

// sessionRoom rebuilds the full room row the former batch load scanned, so
// group consumers (location snapshot colours included) see the same shape.
func sessionRoom(room ports.DirectoryRoom) *ports.SessionRoom {
	return &ports.SessionRoom{
		ID:         room.ID,
		TenantID:   room.TenantID,
		CreatedAt:  room.CreatedAt,
		UpdatedAt:  room.UpdatedAt,
		Name:       room.Name,
		Building:   room.Building,
		Floor:      room.Floor,
		Capacity:   room.Capacity,
		Category:   room.Category,
		Color:      room.Color,
		IsSystem:   room.IsSystem,
		IsOpenRoom: room.IsOpenRoom,
	}
}

// loadRoomsForGroups batch loads the rooms of the given groups.
func (r *SessionRepository) loadRoomsForGroups(ctx context.Context, groups []*ActiveGroupRow) error {
	roomIDs := make([]int64, 0, len(groups))
	for _, group := range groups {
		roomIDs = append(roomIDs, group.RoomID)
	}
	if len(uniquePositive(roomIDs)) == 0 {
		return nil
	}
	rooms, err := roomsByID(ctx, r.rooms, roomIDs)
	if err != nil {
		return r.errors.WrapRecordError("find group rooms by IDs", err)
	}
	for _, group := range groups {
		if room, ok := rooms[group.RoomID]; ok {
			group.Room = sessionRoom(room)
		}
	}
	return nil
}

// templateIDs lists the distinct templates of template-backed sessions.
// Spontaneous sessions have none and are skipped rather than materialising as
// spurious zero IDs.
func templateIDs(groups []*ActiveGroupRow) []int64 {
	ids := make([]int64, 0, len(groups))
	seen := make(map[int64]bool, len(groups))
	for _, group := range groups {
		if templateID, ok := group.TemplateID(); ok && !seen[templateID] {
			ids = append(ids, templateID)
			seen[templateID] = true
		}
	}
	return ids
}

// loadAndAssignActivityGroups loads the activity templates and assigns them.
func (r *SessionRepository) loadAndAssignActivityGroups(ctx context.Context, groups []*ActiveGroupRow, groupIDs []int64) error {
	if len(groupIDs) == 0 {
		return nil
	}
	activities, err := r.queryActivityGroupsByIDs(ctx, groupIDs)
	if err != nil {
		return err
	}
	byID := make(map[int64]*ports.SessionActivity, len(activities))
	for _, activity := range activities {
		byID[activity.ID] = activity
	}
	for _, group := range groups {
		if templateID, ok := group.TemplateID(); ok {
			if activity, found := byID[templateID]; found {
				group.ActualGroup = activity
			}
		}
	}
	return nil
}

// queryActivityGroupsByIDs fetches activity templates by their IDs.
func (r *SessionRepository) queryActivityGroupsByIDs(ctx context.Context, ids []int64) ([]*ports.SessionActivity, error) {
	if r.activities == nil {
		return nil, errActivityDirectoryRequired
	}
	groups, err := r.activities.FindByIDs(ctx, ids)
	if err != nil {
		return nil, r.errors.WrapRecordError("batch load activity groups for unclaimed groups", err)
	}
	return groups, nil
}
