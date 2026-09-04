package iot

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/models/iot"
)

// DirectoryRoom is the Facilities projection this package reads.
// facilities.rooms belongs to that owner (#2665); the composition root binds
// the directory behind RoomDirectory instead of the former SQL joins.
type DirectoryRoom struct {
	ID       int64
	TenantID int64
	Name     string
}

// RoomDirectory is the owner query the device repository reads room names
// through. Every method fails while unbound; there is no fallback join.
type RoomDirectory interface {
	// ListRoomsByID returns the rooms visible in the caller's transaction.
	// Missing IDs are absent, like the former LEFT JOIN.
	ListRoomsByID(ctx context.Context, ids []int64) ([]DirectoryRoom, error)
}

var errRoomDirectoryRequired = errors.New("iot repositories: room directory is not bound")

// attachRoomNames fills Device.RoomName from the owner. The former join also
// required the room to belong to the device's tenant, so a room of another
// tenant leaves the name unset exactly as before.
func attachRoomNames(ctx context.Context, directory RoomDirectory, devices []*iot.Device) error {
	if directory == nil {
		return errRoomDirectoryRequired
	}
	ids := make([]int64, 0, len(devices))
	seen := make(map[int64]struct{}, len(devices))
	for _, device := range devices {
		if device == nil || device.RoomID == nil || *device.RoomID <= 0 {
			continue
		}
		if _, found := seen[*device.RoomID]; found {
			continue
		}
		seen[*device.RoomID] = struct{}{}
		ids = append(ids, *device.RoomID)
	}
	if len(ids) == 0 {
		return nil
	}
	rooms, err := directory.ListRoomsByID(ctx, ids)
	if err != nil {
		return err
	}
	byID := make(map[int64]DirectoryRoom, len(rooms))
	for _, room := range rooms {
		byID[room.ID] = room
	}
	for _, device := range devices {
		if device == nil || device.RoomID == nil {
			continue
		}
		if room, ok := byID[*device.RoomID]; ok && room.TenantID == device.TenantID {
			name := room.Name
			device.RoomName = &name
		}
	}
	return nil
}
