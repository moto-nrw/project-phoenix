package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models"
	"github.com/uptrace/bun"
)

// RoomStore implements basic database operations for room management
type RoomStore struct {
	db *bun.DB
}

// NewRoomStore returns a new RoomStore
func NewRoomStore(db *bun.DB) *RoomStore {
	return &RoomStore{
		db: db,
	}
}

// GetRooms returns all rooms
func (s *RoomStore) GetRooms(ctx context.Context) ([]models.Room, error) {
	var rooms []models.Room
	err := s.db.NewSelect().
		Model(&rooms).
		Order("room_name ASC").
		Scan(ctx)

	return rooms, err
}

// GetRoomsByCategory returns rooms filtered by category
func (s *RoomStore) GetRoomsByCategory(ctx context.Context, category string) ([]models.Room, error) {
	var rooms []models.Room
	err := s.db.NewSelect().
		Model(&rooms).
		Where("category = ?", category).
		Order("room_name ASC").
		Scan(ctx)

	return rooms, err
}

// GetRoomsByBuilding returns rooms filtered by building
func (s *RoomStore) GetRoomsByBuilding(ctx context.Context, building string) ([]models.Room, error) {
	var rooms []models.Room
	err := s.db.NewSelect().
		Model(&rooms).
		Where("building = ?", building).
		Order("room_name ASC").
		Scan(ctx)

	return rooms, err
}

// GetRoomsByFloor returns rooms filtered by floor
func (s *RoomStore) GetRoomsByFloor(ctx context.Context, floor int) ([]models.Room, error) {
	var rooms []models.Room
	err := s.db.NewSelect().
		Model(&rooms).
		Where("floor = ?", floor).
		Order("room_name ASC").
		Scan(ctx)

	return rooms, err
}

// GetRoomsByOccupied returns rooms filtered by occupancy status
func (s *RoomStore) GetRoomsByOccupied(ctx context.Context, occupied bool) ([]models.Room, error) {
	var rooms []models.Room
	query := s.db.NewSelect().Model(&rooms)

	if occupied {
		// Join with RoomOccupancy to find occupied rooms
		query = query.Join("JOIN room_occupancies ro ON rooms.id = ro.room_id")
	} else {
		// Find rooms that don't have any occupancy entries
		query = query.Where("id NOT IN (SELECT room_id FROM room_occupancies)")
	}

	err := query.Order("room_name ASC").Scan(ctx)
	return rooms, err
}

// GetRoomByID returns a room by ID
func (s *RoomStore) GetRoomByID(ctx context.Context, id int64) (*models.Room, error) {
	room := new(models.Room)
	err := s.db.NewSelect().
		Model(room).
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return room, nil
}

// CreateRoom creates a new room
func (s *RoomStore) CreateRoom(ctx context.Context, room *models.Room) error {
	room.CreatedAt = time.Now()
	room.ModifiedAt = time.Now()

	_, err := s.db.NewInsert().
		Model(room).
		Exec(ctx)

	return err
}

// UpdateRoom updates an existing room
func (s *RoomStore) UpdateRoom(ctx context.Context, room *models.Room) error {
	room.ModifiedAt = time.Now()

	_, err := s.db.NewUpdate().
		Model(room).
		WherePK().
		Exec(ctx)

	return err
}

// DeleteRoom deletes a room by ID
func (s *RoomStore) DeleteRoom(ctx context.Context, id int64) error {
	_, err := s.db.NewDelete().
		Model((*models.Room)(nil)).
		Where("id = ?", id).
		Exec(ctx)

	return err
}

// GetRoomsGroupedByCategory returns rooms grouped by category
func (s *RoomStore) GetRoomsGroupedByCategory(ctx context.Context) (map[string][]models.Room, error) {
	var rooms []models.Room
	err := s.db.NewSelect().
		Model(&rooms).
		Order("category ASC, room_name ASC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	// Group rooms by category
	groupedRooms := make(map[string][]models.Room)
	for _, room := range rooms {
		groupedRooms[room.Category] = append(groupedRooms[room.Category], room)
	}

	return groupedRooms, nil
}

// GetCurrentRoomOccupancy returns the current occupancy of a room
func (s *RoomStore) GetCurrentRoomOccupancy(ctx context.Context, roomID int64) (*models.RoomOccupancy, error) {
	occupancy := new(models.RoomOccupancy)
	err := s.db.NewSelect().
		Model(occupancy).
		Relation("Room").
		Relation("Ag").
		Relation("Group").
		Relation("Timespan").
		Relation("Supervisors").
		Relation("Supervisors.CustomUser").
		Where("room_id = ?", roomID).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("no occupancy found for this room")
		}
		return nil, err
	}

	return occupancy, nil
}

// RegisterTablet registers a tablet device to a room
func (s *RoomStore) RegisterTablet(ctx context.Context, roomID int64, deviceID string, agID *int64, groupID *int64) (*models.RoomOccupancy, error) {
	// Check if the tablet is already registered
	exists, err := s.db.NewSelect().
		Model((*models.RoomOccupancy)(nil)).
		Where("device_id = ?", deviceID).
		Exists(ctx)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("tablet is already registered")
	}

	// Get the current timespan
	var timespan models.Timespan
	err = s.db.NewSelect().
		Model(&timespan).
		Where("is_active = true").
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	// Create a new room occupancy
	occupancy := &models.RoomOccupancy{
		DeviceID:   deviceID,
		RoomID:     roomID,
		AgID:       agID,
		GroupID:    groupID,
		TimespanID: timespan.ID,
		CreatedAt:  time.Now(),
		ModifiedAt: time.Now(),
	}

	_, err = s.db.NewInsert().
		Model(occupancy).
		Returning("*").
		Exec(ctx, occupancy)
	if err != nil {
		return nil, err
	}

	// Load the complete occupancy with all relations
	err = s.db.NewSelect().
		Model(occupancy).
		Relation("Room").
		Relation("Ag").
		Relation("Group").
		Relation("Timespan").
		WherePK().
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	return occupancy, nil
}

// UnregisterTablet unregisters a tablet from a room
func (s *RoomStore) UnregisterTablet(ctx context.Context, roomID int64, deviceID string) error {
	// Retrieve the occupancy first for history recording
	occupancy := new(models.RoomOccupancy)
	err := s.db.NewSelect().
		Model(occupancy).
		Where("room_id = ? AND device_id = ?", roomID, deviceID).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("tablet not registered in this room")
		}
		return err
	}

	// Record occupancy history
	historyStore := NewRoomHistoryStore(s.db)
	err = historyStore.CreateRoomHistoryFromOccupancy(ctx, occupancy)
	if err != nil {
		return err
	}

	// Delete the occupancy
	_, err = s.db.NewDelete().
		Model((*models.RoomOccupancy)(nil)).
		Where("room_id = ? AND device_id = ?", roomID, deviceID).
		Exec(ctx)

	return err
}

// GetRoomHistoryByRoom retrieves room history records for a specific room
func (s *RoomStore) GetRoomHistoryByRoom(ctx context.Context, roomID int64) ([]models.RoomHistory, error) {
	var history []models.RoomHistory
	err := s.db.NewSelect().
		Model(&history).
		Where("room_id = ?", roomID).
		Order("day DESC").
		Scan(ctx)

	return history, err
}

// GetRoomHistoryByDateRange retrieves room history records within a date range
func (s *RoomStore) GetRoomHistoryByDateRange(ctx context.Context, startDate, endDate time.Time) ([]models.RoomHistory, error) {
	var history []models.RoomHistory
	err := s.db.NewSelect().
		Model(&history).
		Where("day BETWEEN ? AND ?", startDate, endDate).
		Order("day DESC").
		Scan(ctx)

	return history, err
}

// GetRoomHistoryBySupervisor retrieves room history records for a specific supervisor
func (s *RoomStore) GetRoomHistoryBySupervisor(ctx context.Context, supervisorID int64) ([]models.RoomHistory, error) {
	var history []models.RoomHistory
	err := s.db.NewSelect().
		Model(&history).
		Where("supervisor_id = ?", supervisorID).
		Order("day DESC").
		Scan(ctx)

	return history, err
}
