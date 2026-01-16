// backend/services/facilities/facilities_service.go
package facilities

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/facilities"
	activePort "github.com/moto-nrw/project-phoenix/internal/core/port/active"
	facilitiesPort "github.com/moto-nrw/project-phoenix/internal/core/port/facilities"
	"github.com/uptrace/bun"
)

// Operation name constants to avoid string duplication
const (
	opCreateRoom = "create room"
	opUpdateRoom = "update room"
)

// service implements the facilities.Service interface
type service struct {
	roomRepo        facilitiesPort.RoomRepository
	activeGroupRepo activePort.GroupReadRepository
	db              *bun.DB
	txHandler       *base.TxHandler
}

// NewService creates a new facilities service
func NewService(roomRepo facilitiesPort.RoomRepository, activeGroupRepo activePort.GroupReadRepository, db *bun.DB) Service {
	return &service{
		roomRepo:        roomRepo,
		activeGroupRepo: activeGroupRepo,
		db:              db,
		txHandler:       base.NewTxHandler(db),
	}
}

// WithTx returns a new service that uses the provided transaction
func (s *service) WithTx(tx bun.Tx) any {
	// Get repository with transaction if it implements the TransactionalRepository interface
	var roomRepo = s.roomRepo
	var activeGroupRepo = s.activeGroupRepo

	// Try to cast repository to TransactionalRepository and apply the transaction
	if txRepo, ok := s.roomRepo.(base.TransactionalRepository); ok {
		roomRepo = txRepo.WithTx(tx).(facilitiesPort.RoomRepository)
	}

	if txRepo, ok := s.activeGroupRepo.(base.TransactionalRepository); ok {
		activeGroupRepo = txRepo.WithTx(tx).(activePort.GroupReadRepository)
	}

	// Return a new service with the transaction
	return &service{
		roomRepo:        roomRepo,
		activeGroupRepo: activeGroupRepo,
		db:              s.db,
		txHandler:       s.txHandler.WithTx(tx),
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

// GetRoomWithOccupancy retrieves a room by its ID with occupancy status
func (s *service) GetRoomWithOccupancy(ctx context.Context, id int64) (*facilities.RoomWithOccupancy, error) {
	result, err := s.roomRepo.FindByIDWithOccupancy(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &FacilitiesError{Op: "get room with occupancy", Err: ErrRoomNotFound}
		}
		return nil, &FacilitiesError{Op: "get room with occupancy", Err: err}
	}
	return result, nil
}

// CreateRoom creates a new room
func (s *service) CreateRoom(ctx context.Context, room *facilities.Room) error {
	// Validate room data
	if err := room.Validate(); err != nil {
		return &FacilitiesError{Op: opCreateRoom, Err: err}
	}

	// Check if a room with the same name already exists
	existing, err := s.roomRepo.FindByName(ctx, room.Name)
	if err == nil && existing != nil {
		return &FacilitiesError{Op: opCreateRoom, Err: ErrDuplicateRoom}
	}

	// Create the room
	if err := s.roomRepo.Create(ctx, room); err != nil {
		return &FacilitiesError{Op: opCreateRoom, Err: err}
	}

	return nil
}

// UpdateRoom updates an existing room
func (s *service) UpdateRoom(ctx context.Context, room *facilities.Room) error {
	// Validate room data
	if err := room.Validate(); err != nil {
		return &FacilitiesError{Op: opUpdateRoom, Err: err}
	}

	// Check if room exists
	existingRoom, err := s.roomRepo.FindByID(ctx, room.ID)
	if err != nil {
		return &FacilitiesError{Op: opUpdateRoom, Err: ErrRoomNotFound}
	}

	// If name is changing, check for duplicates
	if existingRoom.Name != room.Name {
		existing, err := s.roomRepo.FindByName(ctx, room.Name)
		if err == nil && existing != nil && existing.ID != room.ID {
			return &FacilitiesError{Op: opUpdateRoom, Err: ErrDuplicateRoom}
		}
	}

	// Update the room
	if err := s.roomRepo.Update(ctx, room); err != nil {
		return &FacilitiesError{Op: opUpdateRoom, Err: err}
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

// ListRooms retrieves all rooms with occupancy status
func (s *service) ListRooms(ctx context.Context, options *base.QueryOptions) ([]facilities.RoomWithOccupancy, error) {
	results, err := s.roomRepo.ListWithOccupancy(ctx, options)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []facilities.RoomWithOccupancy{}, nil
		}
		return nil, &FacilitiesError{Op: "list rooms", Err: err}
	}

	return results, nil
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
	allRooms, err := s.roomRepo.List(ctx, make(map[string]any))
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

// GetAvailableRoomsWithOccupancy finds all rooms that can accommodate the given capacity
// and includes their current occupancy status
func (s *service) GetAvailableRoomsWithOccupancy(ctx context.Context, capacity int) ([]facilities.RoomWithOccupancy, error) {
	// Get all rooms - using empty filter map for now
	allRooms, err := s.roomRepo.List(ctx, make(map[string]any))
	if err != nil {
		return nil, &FacilitiesError{Op: "get available rooms with occupancy", Err: err}
	}

	// First pass: filter rooms by capacity and collect IDs
	var availableRooms []*facilities.Room
	var roomIDs []int64
	for _, room := range allRooms {
		if room.IsAvailable(capacity) {
			availableRooms = append(availableRooms, room)
			roomIDs = append(roomIDs, room.ID)
		}
	}

	// Batch fetch occupied room IDs (avoids N+1 query problem)
	occupiedRoomIDs, err := s.activeGroupRepo.GetOccupiedRoomIDs(ctx, roomIDs)
	if err != nil {
		return nil, &FacilitiesError{Op: "check room occupancy", Err: err}
	}

	// Build response with occupancy status from map lookup
	roomsWithOccupancy := make([]facilities.RoomWithOccupancy, 0, len(availableRooms))
	for _, room := range availableRooms {
		roomsWithOccupancy = append(roomsWithOccupancy, facilities.RoomWithOccupancy{
			Room:       room,
			IsOccupied: occupiedRoomIDs[room.ID],
		})
	}

	return roomsWithOccupancy, nil
}

// GetRoomUtilization calculates the current utilization of a room
func (s *service) GetRoomUtilization(ctx context.Context, roomID int64) (float64, error) {
	room, err := s.roomRepo.FindByID(ctx, roomID)
	if err != nil {
		return 0, &FacilitiesError{Op: "get room utilization", Err: ErrRoomNotFound}
	}

	// This would typically be implemented by querying other systems
	// For now just return a placeholder value
	if room.Capacity == nil || *room.Capacity <= 0 {
		return 0, nil
	}

	// Placeholder logic
	return 0.0, nil
}

// GetBuildingList returns a list of all buildings in the system
func (s *service) GetBuildingList(ctx context.Context) ([]string, error) {
	// Get all rooms - using empty filter map for now
	allRooms, err := s.roomRepo.List(ctx, make(map[string]any))
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
	allRooms, err := s.roomRepo.List(ctx, make(map[string]any))
	if err != nil {
		return nil, &FacilitiesError{Op: "get category list", Err: err}
	}

	// Extract unique category names
	categoryMap := make(map[string]bool)
	for _, room := range allRooms {
		if room.Category != nil && *room.Category != "" {
			categoryMap[*room.Category] = true
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

// GetRoomHistory retrieves the visit history for a room within the specified time range
func (s *service) GetRoomHistory(ctx context.Context, roomID int64, startTime, endTime time.Time) ([]RoomHistoryEntry, error) {
	// First verify the room exists
	_, err := s.roomRepo.FindByID(ctx, roomID)
	if err != nil {
		return nil, &FacilitiesError{Op: "get room history", Err: ErrRoomNotFound}
	}

	// Query room history via repository
	history, err := s.roomRepo.GetRoomHistory(ctx, roomID, startTime, endTime)
	if err != nil {
		return nil, &FacilitiesError{Op: "get room history", Err: err}
	}

	return history, nil
}

// GetRoomsByIDs retrieves rooms by their IDs and returns them as a map
func (s *service) GetRoomsByIDs(ctx context.Context, ids []int64) (map[int64]*facilities.Room, error) {
	if len(ids) == 0 {
		return make(map[int64]*facilities.Room), nil
	}

	rooms, err := s.roomRepo.FindByIDs(ctx, ids)
	if err != nil {
		return nil, &FacilitiesError{Op: "get rooms by IDs", Err: err}
	}

	roomMap := make(map[int64]*facilities.Room, len(rooms))
	for _, room := range rooms {
		roomMap[room.ID] = room
	}

	return roomMap, nil
}
