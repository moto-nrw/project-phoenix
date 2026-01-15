// backend/services/facilities/facilities_service.go
package facilities

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/uptrace/bun"
)

// Operation name constants to avoid string duplication
const (
	opCreateRoom = "create room"
	opUpdateRoom = "update room"
)

// service implements the facilities.Service interface
type service struct {
	roomRepo        facilities.RoomRepository
	activeGroupRepo active.GroupRepository
	db              *bun.DB
	txHandler       *base.TxHandler
}

// NewService creates a new facilities service
func NewService(roomRepo facilities.RoomRepository, activeGroupRepo active.GroupRepository, db *bun.DB) Service {
	return &service{
		roomRepo:        roomRepo,
		activeGroupRepo: activeGroupRepo,
		db:              db,
		txHandler:       base.NewTxHandler(db),
	}
}

// WithTx returns a new service that uses the provided transaction
func (s *service) WithTx(tx bun.Tx) interface{} {
	// Get repository with transaction if it implements the TransactionalRepository interface
	var roomRepo = s.roomRepo
	var activeGroupRepo = s.activeGroupRepo

	// Try to cast repository to TransactionalRepository and apply the transaction
	if txRepo, ok := s.roomRepo.(base.TransactionalRepository); ok {
		roomRepo = txRepo.WithTx(tx).(facilities.RoomRepository)
	}

	if txRepo, ok := s.activeGroupRepo.(base.TransactionalRepository); ok {
		activeGroupRepo = txRepo.WithTx(tx).(active.GroupRepository)
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
func (s *service) GetRoomWithOccupancy(ctx context.Context, id int64) (RoomWithOccupancy, error) {
	// Define result structure for scanning
	type roomQueryResult struct {
		// Room fields
		ID        int64     `bun:"id"`
		Name      string    `bun:"name"`
		Building  string    `bun:"building"`
		Floor     *int      `bun:"floor"`
		Capacity  *int      `bun:"capacity"`
		Category  *string   `bun:"category"`
		Color     *string   `bun:"color"`
		CreatedAt time.Time `bun:"created_at"`
		UpdatedAt time.Time `bun:"updated_at"`

		// Occupancy fields
		IsOccupied   bool    `bun:"is_occupied"`
		GroupName    *string `bun:"group_name"`
		CategoryName *string `bun:"category_name"`
	}

	// Build query with LEFT JOINs for occupancy information
	var result roomQueryResult
	err := s.db.NewSelect().
		TableExpr("facilities.rooms AS r").
		ColumnExpr("r.id, r.name, r.building, r.floor, r.capacity, r.category, r.color, r.created_at, r.updated_at").
		ColumnExpr("CASE WHEN ag.id IS NOT NULL THEN true ELSE false END AS is_occupied").
		ColumnExpr("act_group.name AS group_name").
		ColumnExpr("cat.name AS category_name").
		Join("LEFT JOIN active.groups AS ag ON ag.room_id = r.id AND ag.end_time IS NULL").
		Join("LEFT JOIN activities.groups AS act_group ON act_group.id = ag.group_id").
		Join("LEFT JOIN activities.categories AS cat ON cat.id = act_group.category_id").
		Where("r.id = ?", id).
		Scan(ctx, &result)

	if err != nil {
		// Only treat "no rows" as "room not found" - preserve other database errors
		if errors.Is(err, sql.ErrNoRows) {
			return RoomWithOccupancy{}, &FacilitiesError{Op: "get room with occupancy", Err: ErrRoomNotFound}
		}
		return RoomWithOccupancy{}, &FacilitiesError{Op: "get room with occupancy", Err: err}
	}

	// Convert result to RoomWithOccupancy
	return RoomWithOccupancy{
		Room: &facilities.Room{
			Model: base.Model{
				ID:        result.ID,
				CreatedAt: result.CreatedAt,
				UpdatedAt: result.UpdatedAt,
			},
			Name:     result.Name,
			Building: result.Building,
			Floor:    result.Floor,
			Capacity: result.Capacity,
			Category: result.Category,
			Color:    result.Color,
		},
		IsOccupied:   result.IsOccupied,
		GroupName:    result.GroupName,
		CategoryName: result.CategoryName,
	}, nil
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
func (s *service) ListRooms(ctx context.Context, options *base.QueryOptions) ([]RoomWithOccupancy, error) {
	// Define result structure for scanning
	type roomQueryResult struct {
		// Room fields
		ID        int64     `bun:"id"`
		Name      string    `bun:"name"`
		Building  string    `bun:"building"`
		Floor     *int      `bun:"floor"`
		Capacity  *int      `bun:"capacity"`
		Category  *string   `bun:"category"`
		Color     *string   `bun:"color"`
		CreatedAt time.Time `bun:"created_at"`
		UpdatedAt time.Time `bun:"updated_at"`

		// Occupancy fields
		IsOccupied   bool    `bun:"is_occupied"`
		GroupName    *string `bun:"group_name"`
		CategoryName *string `bun:"category_name"`
	}

	// Build query with LEFT JOINs for occupancy information
	// Use DISTINCT ON to handle rooms with multiple active groups (e.g., Schulhof with Freispiel + Garten)
	query := s.db.NewSelect().
		TableExpr("facilities.rooms AS r").
		DistinctOn("r.id").
		ColumnExpr("r.id, r.name, r.building, r.floor, r.capacity, r.category, r.color, r.created_at, r.updated_at").
		ColumnExpr("CASE WHEN ag.id IS NOT NULL THEN true ELSE false END AS is_occupied").
		ColumnExpr("act_group.name AS group_name").
		ColumnExpr("cat.name AS category_name").
		Join("LEFT JOIN active.groups AS ag ON ag.room_id = r.id AND ag.end_time IS NULL").
		Join("LEFT JOIN activities.groups AS act_group ON act_group.id = ag.group_id").
		Join("LEFT JOIN activities.categories AS cat ON cat.id = act_group.category_id").
		OrderExpr("r.id, r.name ASC")

	// Apply filters if provided
	if options != nil && options.Filter != nil {
		// Set table alias for the filter and apply to query
		options.Filter.WithTableAlias("r")
		query = options.Filter.ApplyToQuery(query)
	}

	// Execute query
	var results []roomQueryResult
	if err := query.Scan(ctx, &results); err != nil {
		// sql.ErrNoRows for list queries should return empty array, not error
		if errors.Is(err, sql.ErrNoRows) {
			return []RoomWithOccupancy{}, nil
		}
		return nil, &FacilitiesError{Op: "list rooms", Err: err}
	}

	// Convert results to RoomWithOccupancy
	roomsWithOccupancy := make([]RoomWithOccupancy, len(results))
	for i, r := range results {
		roomsWithOccupancy[i] = RoomWithOccupancy{
			Room: &facilities.Room{
				Model: base.Model{
					ID:        r.ID,
					CreatedAt: r.CreatedAt,
					UpdatedAt: r.UpdatedAt,
				},
				Name:     r.Name,
				Building: r.Building,
				Floor:    r.Floor,
				Capacity: r.Capacity,
				Category: r.Category,
				Color:    r.Color,
			},
			IsOccupied:   r.IsOccupied,
			GroupName:    r.GroupName,
			CategoryName: r.CategoryName,
		}
	}

	return roomsWithOccupancy, nil
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

// GetAvailableRoomsWithOccupancy finds all rooms that can accommodate the given capacity
// and includes their current occupancy status
func (s *service) GetAvailableRoomsWithOccupancy(ctx context.Context, capacity int) ([]RoomWithOccupancy, error) {
	// Get all rooms - using empty filter map for now
	allRooms, err := s.roomRepo.List(ctx, make(map[string]interface{}))
	if err != nil {
		return nil, &FacilitiesError{Op: "get available rooms with occupancy", Err: err}
	}

	// Filter rooms by capacity and check occupancy
	var roomsWithOccupancy []RoomWithOccupancy
	for _, room := range allRooms {
		if room.IsAvailable(capacity) {
			// Check if room is occupied
			activeGroups, err := s.activeGroupRepo.FindActiveByRoomID(ctx, room.ID)
			if err != nil {
				return nil, &FacilitiesError{Op: "check room occupancy", Err: err}
			}

			roomWithOccupancy := RoomWithOccupancy{
				Room:       room,
				IsOccupied: len(activeGroups) > 0,
			}
			roomsWithOccupancy = append(roomsWithOccupancy, roomWithOccupancy)
		}
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

	// Query active_visits table for room history
	var history []RoomHistoryEntry

	// Build the query
	err = s.db.NewSelect().
		TableExpr("active.visits AS v").
		ColumnExpr("v.student_id").
		ColumnExpr("CONCAT(p.first_name, ' ', p.last_name) AS student_name").
		ColumnExpr("ag.group_id AS group_id").
		ColumnExpr("g.name AS group_name").
		ColumnExpr("v.entry_time AS checked_in").
		ColumnExpr("v.exit_time AS checked_out").
		Join("INNER JOIN active.groups AS ag ON ag.id = v.active_group_id").
		Join("INNER JOIN activities.groups AS g ON g.id = ag.group_id").
		Join("INNER JOIN users.students AS s ON s.id = v.student_id").
		Join("INNER JOIN users.persons AS p ON p.id = s.person_id").
		Where("ag.room_id = ?", roomID).
		Where("v.entry_time >= ?", startTime).
		Where("v.entry_time <= ?", endTime).
		OrderExpr("v.entry_time DESC").
		Scan(ctx, &history)

	if err != nil {
		return nil, &FacilitiesError{Op: "get room history", Err: err}
	}

	return history, nil
}
