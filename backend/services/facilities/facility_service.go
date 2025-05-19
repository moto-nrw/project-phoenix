// backend/services/facilities/facilities_service.go
package facilities

import (
	"context"
	"sort"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/uptrace/bun"
)

// service implements the facilities.Service interface
type service struct {
	roomRepo  facilities.RoomRepository
	db        *bun.DB
	txHandler *base.TxHandler
}

// NewService creates a new facilities service
func NewService(roomRepo facilities.RoomRepository, db *bun.DB) Service {
	return &service{
		roomRepo:  roomRepo,
		db:        db,
		txHandler: base.NewTxHandler(db),
	}
}

// WithTx returns a new service that uses the provided transaction
func (s *service) WithTx(tx bun.Tx) interface{} {
	// Get repository with transaction if it implements the TransactionalRepository interface
	var roomRepo = s.roomRepo

	// Try to cast repository to TransactionalRepository and apply the transaction
	if txRepo, ok := s.roomRepo.(base.TransactionalRepository); ok {
		roomRepo = txRepo.WithTx(tx).(facilities.RoomRepository)
	}

	// Return a new service with the transaction
	return &service{
		roomRepo:  roomRepo,
		db:        s.db,
		txHandler: s.txHandler.WithTx(tx),
	}
}

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
	// Validate room data
	if err := room.Validate(); err != nil {
		return &FacilitiesError{Op: "create room", Err: err}
	}

	// Check if a room with the same name already exists
	existing, err := s.roomRepo.FindByName(ctx, room.Name)
	if err == nil && existing != nil {
		return &FacilitiesError{Op: "create room", Err: ErrDuplicateRoom}
	}

	// Create the room
	if err := s.roomRepo.Create(ctx, room); err != nil {
		return &FacilitiesError{Op: "create room", Err: err}
	}

	return nil
}

// UpdateRoom updates an existing room
func (s *service) UpdateRoom(ctx context.Context, room *facilities.Room) error {
	// Validate room data
	if err := room.Validate(); err != nil {
		return &FacilitiesError{Op: "update room", Err: err}
	}

	// Check if room exists
	existingRoom, err := s.roomRepo.FindByID(ctx, room.ID)
	if err != nil {
		return &FacilitiesError{Op: "update room", Err: ErrRoomNotFound}
	}

	// If name is changing, check for duplicates
	if existingRoom.Name != room.Name {
		existing, err := s.roomRepo.FindByName(ctx, room.Name)
		if err == nil && existing != nil && existing.ID != room.ID {
			return &FacilitiesError{Op: "update room", Err: ErrDuplicateRoom}
		}
	}

	// Update the room
	if err := s.roomRepo.Update(ctx, room); err != nil {
		return &FacilitiesError{Op: "update room", Err: err}
	}

	return nil
}

// DeleteRoom deletes a room by its ID
func (s *service) DeleteRoom(ctx context.Context, id int64) error {
	// Check if room exists
	_, err := s.roomRepo.FindByID(ctx, id)
	if err != nil {
		return &FacilitiesError{Op: "delete room", Err: ErrRoomNotFound}
	}

	// Delete the room
	if err := s.roomRepo.Delete(ctx, id); err != nil {
		return &FacilitiesError{Op: "delete room", Err: err}
	}

	return nil
}

// ListRooms retrieves all rooms matching the provided filters
func (s *service) ListRooms(ctx context.Context, options *base.QueryOptions) ([]*facilities.Room, error) {
	// TODO: Follow education.groups pattern - add ListWithOptions to RoomRepository interface
	// and call it directly instead of converting to map[string]interface{}
	// See education service for the correct implementation pattern
	
	// Convert QueryOptions to map[string]interface{} for now
	// This is a temporary solution until RoomRepository is updated to use QueryOptions
	filters := make(map[string]interface{})

	if options != nil && options.Filter != nil {
		// You might want to implement a conversion from QueryOptions to the old format
		// For now we'll just set some basic filters if available
		// This is not a complete conversion, just a simple example
		// TODO: Implement filter conversion
		_ = options.Filter // Mark as intentionally unused for now
	}

	rooms, err := s.roomRepo.List(ctx, filters)
	if err != nil {
		return nil, &FacilitiesError{Op: "list rooms", Err: err}
	}

	return rooms, nil
}

// FindRoomByName finds a room by its name
func (s *service) FindRoomByName(ctx context.Context, name string) (*facilities.Room, error) {
	room, err := s.roomRepo.FindByName(ctx, name)
	if err != nil {
		return nil, &FacilitiesError{Op: "find room by name", Err: ErrRoomNotFound}
	}

	return room, nil
}

// FindRoomsByBuilding finds rooms by building
func (s *service) FindRoomsByBuilding(ctx context.Context, building string) ([]*facilities.Room, error) {
	rooms, err := s.roomRepo.FindByBuilding(ctx, building)
	if err != nil {
		return nil, &FacilitiesError{Op: "find rooms by building", Err: err}
	}

	return rooms, nil
}

// FindRoomsByCategory finds rooms by category
func (s *service) FindRoomsByCategory(ctx context.Context, category string) ([]*facilities.Room, error) {
	rooms, err := s.roomRepo.FindByCategory(ctx, category)
	if err != nil {
		return nil, &FacilitiesError{Op: "find rooms by category", Err: err}
	}

	return rooms, nil
}

// FindRoomsByFloor finds rooms by building and floor
func (s *service) FindRoomsByFloor(ctx context.Context, building string, floor int) ([]*facilities.Room, error) {
	rooms, err := s.roomRepo.FindByFloor(ctx, building, floor)
	if err != nil {
		return nil, &FacilitiesError{Op: "find rooms by floor", Err: err}
	}

	return rooms, nil
}

// CheckRoomAvailability checks if a room is available for a given capacity
func (s *service) CheckRoomAvailability(ctx context.Context, roomID int64, requiredCapacity int) (bool, error) {
	room, err := s.roomRepo.FindByID(ctx, roomID)
	if err != nil {
		return false, &FacilitiesError{Op: "check room availability", Err: ErrRoomNotFound}
	}

	return room.IsAvailable(requiredCapacity), nil
}

// GetAvailableRooms finds all rooms that can accommodate the given capacity
func (s *service) GetAvailableRooms(ctx context.Context, capacity int) ([]*facilities.Room, error) {
	// Get all rooms - using empty filter map for now
	allRooms, err := s.roomRepo.List(ctx, make(map[string]interface{}))
	if err != nil {
		return nil, &FacilitiesError{Op: "get available rooms", Err: err}
	}

	// Filter rooms by capacity
	var availableRooms []*facilities.Room
	for _, room := range allRooms {
		if room.IsAvailable(capacity) {
			availableRooms = append(availableRooms, room)
		}
	}

	return availableRooms, nil
}

// GetRoomUtilization calculates the current utilization of a room
func (s *service) GetRoomUtilization(ctx context.Context, roomID int64) (float64, error) {
	room, err := s.roomRepo.FindByID(ctx, roomID)
	if err != nil {
		return 0, &FacilitiesError{Op: "get room utilization", Err: ErrRoomNotFound}
	}

	// This would typically be implemented by querying other systems
	// For now just return a placeholder value
	if room.Capacity <= 0 {
		return 0, nil
	}

	// Placeholder logic
	return 0.0, nil
}

// GetBuildingList returns a list of all buildings in the system
func (s *service) GetBuildingList(ctx context.Context) ([]string, error) {
	// Get all rooms - using empty filter map for now
	allRooms, err := s.roomRepo.List(ctx, make(map[string]interface{}))
	if err != nil {
		return nil, &FacilitiesError{Op: "get building list", Err: err}
	}

	// Extract unique building names
	buildingMap := make(map[string]bool)
	for _, room := range allRooms {
		if room.Building != "" {
			buildingMap[room.Building] = true
		}
	}

	// Convert map to sorted slice
	buildings := make([]string, 0, len(buildingMap))
	for building := range buildingMap {
		buildings = append(buildings, building)
	}
	sort.Strings(buildings)

	return buildings, nil
}

// GetCategoryList returns a list of all room categories in the system
func (s *service) GetCategoryList(ctx context.Context) ([]string, error) {
	// Get all rooms - using empty filter map for now
	allRooms, err := s.roomRepo.List(ctx, make(map[string]interface{}))
	if err != nil {
		return nil, &FacilitiesError{Op: "get category list", Err: err}
	}

	// Extract unique category names
	categoryMap := make(map[string]bool)
	for _, room := range allRooms {
		if room.Category != "" {
			categoryMap[room.Category] = true
		}
	}

	// Convert map to sorted slice
	categories := make([]string, 0, len(categoryMap))
	for category := range categoryMap {
		categories = append(categories, category)
	}
	sort.Strings(categories)

	return categories, nil
}
