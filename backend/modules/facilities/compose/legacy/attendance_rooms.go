package legacy

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
)

// AttendanceRoomRecords is the retained room read and lock contract.
type AttendanceRoomRecords interface {
	FindByID(context.Context, interface{}) (*facilities.Room, error)
	FindByIDForUpdate(context.Context, int64) (*facilities.Room, error)
	FindByIDs(context.Context, []int64) ([]*facilities.Room, error)
	List(context.Context, map[string]interface{}) ([]*facilities.Room, error)
}

type attendanceRooms struct{ records AttendanceRoomRecords }

// NewAttendanceRooms projects room facts while delegating locking to Facilities.
func NewAttendanceRooms(records AttendanceRoomRecords) presenceservice.AttendanceRooms {
	return attendanceRooms{records: records}
}

func (r attendanceRooms) FindByID(ctx context.Context, id interface{}) (*presenceservice.SessionRoom, error) {
	room, err := r.records.FindByID(ctx, id)
	return projectAttendanceRoom(room), err
}

func (r attendanceRooms) FindByIDForUpdate(ctx context.Context, id int64) (*presenceservice.SessionRoom, error) {
	room, err := r.records.FindByIDForUpdate(ctx, id)
	return projectAttendanceRoom(room), err
}

func (r attendanceRooms) FindByIDs(ctx context.Context, ids []int64) ([]*presenceservice.SessionRoom, error) {
	rooms, err := r.records.FindByIDs(ctx, ids)
	return projectAttendanceRooms(rooms), err
}

func (r attendanceRooms) List(ctx context.Context, filters map[string]interface{}) ([]*presenceservice.SessionRoom, error) {
	rooms, err := r.records.List(ctx, filters)
	return projectAttendanceRooms(rooms), err
}

func projectAttendanceRooms(rooms []*facilities.Room) []*presenceservice.SessionRoom {
	if rooms == nil {
		return nil
	}
	result := make([]*presenceservice.SessionRoom, len(rooms))
	for i, room := range rooms {
		result[i] = projectAttendanceRoom(room)
	}
	return result
}

func projectAttendanceRoom(room *facilities.Room) *presenceservice.SessionRoom {
	if room == nil {
		return nil
	}
	return &presenceservice.SessionRoom{
		ID: room.ID, TenantID: room.TenantID, CreatedAt: room.CreatedAt, UpdatedAt: room.UpdatedAt,
		Name: room.Name, Building: room.Building, Floor: room.Floor, Capacity: room.Capacity,
		Category: room.Category, Color: room.Color, IsSystem: room.IsSystem, IsOpenRoom: room.IsOpenRoom,
	}
}
