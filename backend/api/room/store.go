package room

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/database"
	models2 "github.com/moto-nrw/project-phoenix/models"
	"github.com/uptrace/bun"
)

// RoomStore defines database operations for room management
type RoomStore interface {
	// Room operations
	GetRooms(ctx context.Context) ([]models2.Room, error)
	GetRoomsByCategory(ctx context.Context, category string) ([]models2.Room, error)
	GetRoomsByBuilding(ctx context.Context, building string) ([]models2.Room, error)
	GetRoomsByFloor(ctx context.Context, floor int) ([]models2.Room, error)
	GetRoomsByOccupied(ctx context.Context, occupied bool) ([]models2.Room, error)
	GetRoomByID(ctx context.Context, id int64) (*models2.Room, error)
	CreateRoom(ctx context.Context, room *models2.Room) error
	UpdateRoom(ctx context.Context, room *models2.Room) error
	DeleteRoom(ctx context.Context, id int64) error
	GetRoomsGroupedByCategory(ctx context.Context) (map[string][]models2.Room, error)

	// Room occupancy operations
	GetAllRoomOccupancies(ctx context.Context) ([]RoomOccupancyDetail, error)
	GetRoomOccupancyByID(ctx context.Context, id int64) (*RoomOccupancyDetail, error)
	GetCurrentRoomOccupancy(ctx context.Context, roomID int64) (*RoomOccupancyDetail, error)
	RegisterTablet(ctx context.Context, roomID int64, req *RegisterTabletRequest) (*RoomOccupancy, error)
	UnregisterTablet(ctx context.Context, roomID int64, deviceID string) error
	AddSupervisorToRoomOccupancy(ctx context.Context, roomOccupancyID, supervisorID int64) error

	// Room merging operations
	MergeRooms(ctx context.Context, sourceRoomID, targetRoomID int64, name string, validUntil *time.Time, accessPolicy string) (*models2.CombinedGroup, error)
	GetCombinedGroupForRoom(ctx context.Context, roomID int64) (*models2.CombinedGroup, error)
	FindActiveCombinedGroups(ctx context.Context) ([]models2.CombinedGroup, error)
	DeactivateCombinedGroup(ctx context.Context, id int64) error
}

type roomStore struct {
	db             *bun.DB
	baseStore      *database.RoomStore
	occupancyStore OccupancyStore
	mergeStore     MergeStore
}

// NewRoomStore returns a new RoomStore implementation
func NewRoomStore(db *bun.DB) RoomStore {
	return &roomStore{
		db:             db,
		baseStore:      database.NewRoomStore(db),
		occupancyStore: NewOccupancyStore(db),
		mergeStore:     NewMergeStore(db),
	}
}

// GetRooms returns all rooms
func (s *roomStore) GetRooms(ctx context.Context) ([]models2.Room, error) {
	return s.baseStore.GetRooms(ctx)
}

// GetRoomsByCategory returns rooms filtered by category
func (s *roomStore) GetRoomsByCategory(ctx context.Context, category string) ([]models2.Room, error) {
	return s.baseStore.GetRoomsByCategory(ctx, category)
}

// GetRoomsByBuilding returns rooms filtered by building
func (s *roomStore) GetRoomsByBuilding(ctx context.Context, building string) ([]models2.Room, error) {
	return s.baseStore.GetRoomsByBuilding(ctx, building)
}

// GetRoomsByFloor returns rooms filtered by floor
func (s *roomStore) GetRoomsByFloor(ctx context.Context, floor int) ([]models2.Room, error) {
	return s.baseStore.GetRoomsByFloor(ctx, floor)
}

// GetRoomsByOccupied returns rooms filtered by occupancy status
func (s *roomStore) GetRoomsByOccupied(ctx context.Context, occupied bool) ([]models2.Room, error) {
	return s.baseStore.GetRoomsByOccupied(ctx, occupied)
}

// GetRoomByID returns a room by ID
func (s *roomStore) GetRoomByID(ctx context.Context, id int64) (*models2.Room, error) {
	return s.baseStore.GetRoomByID(ctx, id)
}

// CreateRoom creates a new room
func (s *roomStore) CreateRoom(ctx context.Context, room *models2.Room) error {
	return s.baseStore.CreateRoom(ctx, room)
}

// UpdateRoom updates an existing room
func (s *roomStore) UpdateRoom(ctx context.Context, room *models2.Room) error {
	return s.baseStore.UpdateRoom(ctx, room)
}

// DeleteRoom deletes a room by ID
func (s *roomStore) DeleteRoom(ctx context.Context, id int64) error {
	return s.baseStore.DeleteRoom(ctx, id)
}

// GetRoomsGroupedByCategory returns rooms grouped by category
func (s *roomStore) GetRoomsGroupedByCategory(ctx context.Context) (map[string][]models2.Room, error) {
	return s.baseStore.GetRoomsGroupedByCategory(ctx)
}

// GetAllRoomOccupancies returns all room occupancies with details
func (s *roomStore) GetAllRoomOccupancies(ctx context.Context) ([]RoomOccupancyDetail, error) {
	return s.occupancyStore.GetAllRoomOccupancies(ctx)
}

// GetRoomOccupancyByID returns room occupancy details by ID
func (s *roomStore) GetRoomOccupancyByID(ctx context.Context, id int64) (*RoomOccupancyDetail, error) {
	return s.occupancyStore.GetRoomOccupancyByID(ctx, id)
}

// GetCurrentRoomOccupancy returns current occupancy for a room
func (s *roomStore) GetCurrentRoomOccupancy(ctx context.Context, roomID int64) (*RoomOccupancyDetail, error) {
	return s.occupancyStore.GetCurrentRoomOccupancy(ctx, roomID)
}

// RegisterTablet registers a tablet to a room
func (s *roomStore) RegisterTablet(ctx context.Context, roomID int64, req *RegisterTabletRequest) (*RoomOccupancy, error) {
	return s.occupancyStore.RegisterTablet(ctx, roomID, req)
}

// UnregisterTablet unregisters a tablet from a room
func (s *roomStore) UnregisterTablet(ctx context.Context, roomID int64, deviceID string) error {
	return s.occupancyStore.UnregisterTablet(ctx, roomID, deviceID)
}

// AddSupervisorToRoomOccupancy adds a supervisor to a room occupancy
func (s *roomStore) AddSupervisorToRoomOccupancy(ctx context.Context, roomOccupancyID, supervisorID int64) error {
	return s.occupancyStore.AddSupervisorToRoomOccupancy(ctx, roomOccupancyID, supervisorID)
}

// MergeRooms merges two rooms and creates a combined group
func (s *roomStore) MergeRooms(ctx context.Context, sourceRoomID, targetRoomID int64, name string, validUntil *time.Time, accessPolicy string) (*models2.CombinedGroup, error) {
	return s.mergeStore.MergeRooms(ctx, sourceRoomID, targetRoomID, name, validUntil, accessPolicy)
}

// GetCombinedGroupForRoom retrieves the combined group that includes a room
func (s *roomStore) GetCombinedGroupForRoom(ctx context.Context, roomID int64) (*models2.CombinedGroup, error) {
	return s.mergeStore.GetCombinedGroupForRoom(ctx, roomID)
}

// FindActiveCombinedGroups returns all active combined groups
func (s *roomStore) FindActiveCombinedGroups(ctx context.Context) ([]models2.CombinedGroup, error) {
	return s.mergeStore.FindActiveCombinedGroups(ctx)
}

// DeactivateCombinedGroup deactivates a combined group
func (s *roomStore) DeactivateCombinedGroup(ctx context.Context, id int64) error {
	return s.mergeStore.DeactivateCombinedGroup(ctx, id)
}
