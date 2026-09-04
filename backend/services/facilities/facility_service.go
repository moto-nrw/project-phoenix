package facilities

import (
	"context"
	"errors"
	"maps"
	"slices"
	"time"

	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
)

type OccupancyProjection func(context.Context, []facilitiesModule.Room) ([]RoomWithOccupancy, error)
type RoomHistoryProjection func(context.Context, int64, time.Time, time.Time, *int64) ([]RoomSessionEntry, error)

type ServiceConfig struct {
	Rooms            facilitiesModule.Capability
	Occupancy        OccupancyProjection
	History          RoomHistoryProjection
	ValidateDeletion func(context.Context, int64) error
}

type service struct{ config ServiceConfig }

func NewServiceWithConfig(config ServiceConfig) Service {
	if config.Rooms == nil || config.Occupancy == nil || config.History == nil || config.ValidateDeletion == nil {
		panic("facilities compatibility service: all dependencies are required")
	}
	return &service{config: config}
}

func (s *service) GetRoom(ctx context.Context, id int64) (*facilitiesModule.Room, error) {
	room, err := s.config.Rooms.FindRoom(ctx, id)
	if err != nil {
		return nil, wrap("get room", err)
	}
	return roomPointer(room), nil
}

func (s *service) GetRoomWithOccupancy(ctx context.Context, id int64) (RoomWithOccupancy, error) {
	room, err := s.config.Rooms.FindRoom(ctx, id)
	if err != nil {
		return RoomWithOccupancy{}, wrap("get room with occupancy", err)
	}
	rows, err := s.config.Occupancy(ctx, []facilitiesModule.Room{room})
	if err != nil {
		return RoomWithOccupancy{}, wrap("get room with occupancy", err)
	}
	if len(rows) != 1 {
		return RoomWithOccupancy{}, wrap("get room with occupancy", errors.New("occupancy projection returned an invalid result"))
	}
	return rows[0], nil
}

func (s *service) CreateRoom(ctx context.Context, room *facilitiesModule.Room) error {
	if room == nil {
		return wrap("create room", errors.New("room cannot be nil"))
	}
	created, err := s.config.Rooms.CreateRoom(ctx, facilitiesModule.CreateRoom{
		Name: room.Name, Building: room.Building, Floor: room.Floor, Capacity: room.Capacity,
		Category: room.Category, Color: room.Color, IsSystem: room.IsSystem,
	})
	if err != nil {
		return wrap("create room", err)
	}
	*room = created
	return nil
}

func (s *service) UpdateRoom(ctx context.Context, room *facilitiesModule.Room) error {
	if room == nil {
		return wrap("update room", errors.New("room cannot be nil"))
	}
	updated, err := s.config.Rooms.UpdateRoom(ctx, facilitiesModule.UpdateRoom{
		ID: room.ID, Name: room.Name, Building: room.Building, Floor: room.Floor,
		Capacity: room.Capacity, Category: room.Category, Color: room.Color,
	})
	if err != nil {
		return wrap("update room", err)
	}
	*room = updated
	return nil
}

func (s *service) DeleteRoom(ctx context.Context, id int64) error {
	return wrap("delete room", s.config.Rooms.DeleteRoom(ctx, id))
}

func (s *service) ValidateRoomDeletion(ctx context.Context, id int64) error {
	return wrap("delete room", s.config.ValidateDeletion(ctx, id))
}

func (s *service) ListRooms(ctx context.Context) ([]RoomWithOccupancy, error) {
	rooms, err := s.config.Rooms.ListRooms(ctx, facilitiesModule.RoomFilter{})
	if err != nil {
		return nil, wrap("list rooms", err)
	}
	rows, err := s.ProjectRoomOccupancy(ctx, rooms)
	return rows, wrap("list rooms", err)
}

func (s *service) ProjectRoomOccupancy(ctx context.Context, rooms []facilitiesModule.Room) ([]RoomWithOccupancy, error) {
	return s.config.Occupancy(ctx, rooms)
}

func (s *service) FindRoomByName(ctx context.Context, name string) (*facilitiesModule.Room, error) {
	room, err := s.config.Rooms.FindRoomByName(ctx, name)
	if err != nil {
		return nil, wrap("find room by name", err)
	}
	return roomPointer(room), nil
}

func (s *service) FindToiletRoom(ctx context.Context, exclude int64) (*facilitiesModule.Room, error) {
	room, err := s.config.Rooms.FindToiletRoom(ctx, exclude)
	if err != nil {
		return nil, wrap("find toilet room", err)
	}
	return roomPointer(room), nil
}

func (s *service) FindRoomsByCategory(ctx context.Context, category string) ([]*facilitiesModule.Room, error) {
	rooms, err := s.config.Rooms.ListRooms(ctx, facilitiesModule.RoomFilter{Category: &category})
	if err != nil {
		return nil, wrap("find rooms by category", err)
	}
	return roomPointers(rooms), nil
}

func (s *service) GetAvailableRooms(ctx context.Context, capacity int) ([]*facilitiesModule.Room, error) {
	rooms, err := s.config.Rooms.ListRooms(ctx, facilitiesModule.RoomFilter{MinimumCapacity: &capacity})
	if err != nil {
		return nil, wrap("get available rooms", err)
	}
	return roomPointers(rooms), nil
}

func (s *service) GetAvailableRoomsWithOccupancy(ctx context.Context, capacity int) ([]RoomWithOccupancy, error) {
	rooms, err := s.config.Rooms.ListRooms(ctx, facilitiesModule.RoomFilter{MinimumCapacity: &capacity})
	if err != nil {
		return nil, wrap("get available rooms with occupancy", err)
	}
	rows, err := s.config.Occupancy(ctx, rooms)
	return rows, wrap("get available rooms with occupancy", err)
}

func (s *service) GetBuildingList(ctx context.Context) ([]string, error) {
	rooms, err := s.config.Rooms.ListRooms(ctx, facilitiesModule.RoomFilter{})
	if err != nil {
		return nil, wrap("get building list", err)
	}
	values := make(map[string]struct{})
	for _, room := range rooms {
		if room.Building != "" {
			values[room.Building] = struct{}{}
		}
	}
	return slices.Sorted(maps.Keys(values)), nil
}

func (s *service) GetCategoryList(ctx context.Context) ([]string, error) {
	rooms, err := s.config.Rooms.ListRooms(ctx, facilitiesModule.RoomFilter{})
	if err != nil {
		return nil, wrap("get category list", err)
	}
	values := make(map[string]struct{})
	for _, room := range rooms {
		if room.Category != nil && *room.Category != "" {
			values[*room.Category] = struct{}{}
		}
	}
	return slices.Sorted(maps.Keys(values)), nil
}

func (s *service) GetRoomHistory(ctx context.Context, roomID int64, start, end time.Time, staffID *int64) ([]RoomSessionEntry, error) {
	if _, err := s.config.Rooms.FindRoom(ctx, roomID); err != nil {
		return nil, wrap("get room history", err)
	}
	rows, err := s.config.History(ctx, roomID, start, end, staffID)
	return rows, wrap("get room history", err)
}

func roomPointer(room facilitiesModule.Room) *facilitiesModule.Room { return &room }

func roomPointers(rooms []facilitiesModule.Room) []*facilitiesModule.Room {
	result := make([]*facilitiesModule.Room, len(rooms))
	for index := range rooms {
		result[index] = &rooms[index]
	}
	return result
}

func wrap(operation string, err error) error {
	if err == nil {
		return nil
	}
	return &FacilitiesError{Op: operation, Err: err}
}
