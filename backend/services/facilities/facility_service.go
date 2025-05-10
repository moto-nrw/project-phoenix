// backend/services/facilities/facilities_service.go
package facilities

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/uptrace/bun"
)

// service implements the facilities.Service interface
type service struct {
	roomRepo facilities.RoomRepository
	db       *bun.DB
}

// NewService creates a new facilities service
func NewService(
	roomRepo facilities.RoomRepository,
	db *bun.DB,
) Service {
	return &service{
		roomRepo: roomRepo,
		db:       db,
	}
}

// Room operations

// GetRoom retrieves a room by its ID
func (s *service) GetRoom(ctx context.Context, id int64) (*facilities.Room, error) {
	room, err := s.roomRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &FacilitiesError{Op: "get room", Err: ErrRoomNotFound}
	}
	return room, nil
}

// CreateRoom creates a new room
func (s *service) CreateRoom(ctx context.Context, room *facilities.Room) error {
	if room == nil {
		return &FacilitiesError{Op: "create room", Err: ErrInvalidRoomData}
	}

	if err := room.Validate(); err != nil {
		return &FacilitiesError{Op: "create room", Err: err}
	}

	// Check if a room with the same name already exists
	existingRoom, err := s.roomRepo.FindByName(ctx, room.Name)
	if err == nil && existingRoom != nil {
		return &FacilitiesError{Op: "create room", Err: ErrRoomAlreadyExists}
	}

	if err := s.roomRepo.Create(ctx, room); err != nil {
		return &FacilitiesError{Op: "create room", Err: err}
	}

	return nil
}

// UpdateRoom updates an existing room
func (s *service) UpdateRoom(ctx context.Context, room *facilities.Room) error {
	if room == nil {
		return &FacilitiesError{Op: "update room", Err: ErrInvalidRoomData}
	}

	if err := room.Validate(); err != nil {
		return &FacilitiesError{Op: "update room", Err: err}
	}

	// Check if the room exists
	_, err := s.roomRepo.FindByID(ctx, room.ID)
	if err != nil {
		return &FacilitiesError{Op: "update room", Err: ErrRoomNotFound}
	}

	// Check if a different room with the same name exists
	existingRoom, err := s.roomRepo.FindByName(ctx, room.Name)
	if err == nil && existingRoom != nil && existingRoom.ID != room.ID {
		return &FacilitiesError{Op: "update room", Err: ErrRoomAlreadyExists}
	}

	if err := s.roomRepo.Update(ctx, room); err != nil {
		return &FacilitiesError{Op: "update room", Err: err}
	}

	return nil
}

// DeleteRoom deletes a room by its ID
func (s *service) DeleteRoom(ctx context.Context, id int64) error {
	// Check if the room exists
	_, err := s.roomRepo.FindByID(ctx, id)
	if err != nil {
		return &FacilitiesError{Op: "delete room", Err: ErrRoomNotFound}
	}

	if err := s.roomRepo.Delete(ctx, id); err != nil {
		return &FacilitiesError{Op: "delete room", Err: err}
	}

	return nil
}

// ListRooms retrieves all rooms matching the provided filters
func (s *service) ListRooms(ctx context.Context, options *base.QueryOptions) ([]*facilities.Room, error) {
	// Convert QueryOptions to the map-based filters that the repository accepts
	filters := make(map[string]interface{})

	if options != nil && options.Filter != nil {
		// This is a simplified conversion - in a real implementation,
		// we should update the repository interface to accept QueryOptions directly
		// For now, we'll handle some basic filters

		// TODO: Implement proper conversion from QueryOptions to map[string]interface{}
		// This is just a placeholder for demonstration
	}

	rooms, err := s.roomRepo.List(ctx, filters)
	if err != nil {
		return nil, &FacilitiesError{Op: "list rooms", Err: err}
	}

	return rooms, nil
}

// Room search operations

// FindRoomByName retrieves a room by its name
func (s *service) FindRoomByName(ctx context.Context, name string) (*facilities.Room, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, &FacilitiesError{Op: "find room by name", Err: ErrInvalidSearchCriteria}
	}

	room, err := s.roomRepo.FindByName(ctx, name)
	if err != nil {
		return nil, &FacilitiesError{Op: "find room by name", Err: ErrRoomNotFound}
	}
	return room, nil
}

// FindRoomsByBuilding retrieves rooms by building
func (s *service) FindRoomsByBuilding(ctx context.Context, building string) ([]*facilities.Room, error) {
	building = strings.TrimSpace(building)
	if building == "" {
		return nil, &FacilitiesError{Op: "find rooms by building", Err: ErrInvalidSearchCriteria}
	}

	rooms, err := s.roomRepo.FindByBuilding(ctx, building)
	if err != nil {
		return nil, &FacilitiesError{Op: "find rooms by building", Err: err}
	}
	return rooms, nil
}

// FindRoomsByCategory retrieves rooms by category
func (s *service) FindRoomsByCategory(ctx context.Context, category string) ([]*facilities.Room, error) {
	category = strings.TrimSpace(category)
	if category == "" {
		return nil, &FacilitiesError{Op: "find rooms by category", Err: ErrInvalidSearchCriteria}
	}

	rooms, err := s.roomRepo.FindByCategory(ctx, category)
	if err != nil {
		return nil, &FacilitiesError{Op: "find rooms by category", Err: err}
	}
	return rooms, nil
}

// FindRoomsByFloor retrieves rooms by building and floor
func (s *service) FindRoomsByFloor(ctx context.Context, building string, floor int) ([]*facilities.Room, error) {
	rooms, err := s.roomRepo.FindByFloor(ctx, building, floor)
	if err != nil {
		return nil, &FacilitiesError{Op: "find rooms by floor", Err: err}
	}
	return rooms, nil
}

// FindRoomsWithCapacity retrieves rooms with at least the specified capacity
func (s *service) FindRoomsWithCapacity(ctx context.Context, minCapacity int) ([]*facilities.Room, error) {
	if minCapacity < 0 {
		return nil, &FacilitiesError{Op: "find rooms with capacity", Err: ErrInvalidCapacity}
	}

	rooms, err := s.roomRepo.FindWithCapacity(ctx, minCapacity)
	if err != nil {
		return nil, &FacilitiesError{Op: "find rooms with capacity", Err: err}
	}
	return rooms, nil
}

// SearchRoomsByText performs a text search across multiple room fields
func (s *service) SearchRoomsByText(ctx context.Context, searchText string) ([]*facilities.Room, error) {
	searchText = strings.TrimSpace(searchText)
	if searchText == "" {
		return nil, &FacilitiesError{Op: "search rooms by text", Err: ErrInvalidSearchCriteria}
	}

	rooms, err := s.roomRepo.SearchByText(ctx, searchText)
	if err != nil {
		return nil, &FacilitiesError{Op: "search rooms by text", Err: err}
	}
	return rooms, nil
}
