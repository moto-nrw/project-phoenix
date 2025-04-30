package database

import (
	"context"
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
