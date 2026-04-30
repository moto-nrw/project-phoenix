// backend/services/facilities/facilities_service.go
package facilities

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/constants"
	repoBase "github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
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
}

// NewService creates a new facilities service
func NewService(roomRepo facilities.RoomRepository, activeGroupRepo active.GroupRepository, db *bun.DB) Service {
	return &service{
		roomRepo:        roomRepo,
		activeGroupRepo: activeGroupRepo,
		db:              db,
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
		IsOccupied      bool    `bun:"is_occupied"`
		GroupName       *string `bun:"group_name"`
		CategoryName    *string `bun:"category_name"`
		StudentCount    int     `bun:"student_count"`
		SupervisorNames *string `bun:"supervisor_names"`
	}

	// Build query with LEFT JOINs for occupancy information
	var result roomQueryResult
	err := repoBase.GetDB(ctx, s.db).NewSelect().
		TableExpr("facilities.rooms AS r").
		ColumnExpr("r.id, r.name, r.building, r.floor, r.capacity, r.category, r.color, r.created_at, r.updated_at").
		ColumnExpr("CASE WHEN ag.id IS NOT NULL THEN true ELSE false END AS is_occupied").
		ColumnExpr("act_group.name AS group_name").
		ColumnExpr("cat.name AS category_name").
		// Student count: count active visits for this room's active group
		ColumnExpr(`COALESCE((
			SELECT COUNT(DISTINCT v.student_id)
			FROM active.visits v
			WHERE v.active_group_id = ag.id AND v.exit_time IS NULL
		), 0)::int AS student_count`).
		// Supervisor names: aggregate staff names for this room's active group
		ColumnExpr(`(
			SELECT string_agg(DISTINCT CONCAT(p.first_name, ' ', p.last_name), ', ')
			FROM active.group_supervisors gs
			INNER JOIN users.staff st ON st.id = gs.staff_id
			INNER JOIN users.persons p ON p.id = st.person_id
			WHERE gs.group_id = ag.id AND gs.end_date IS NULL
		) AS supervisor_names`).
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
		IsOccupied:      result.IsOccupied,
		GroupName:       result.GroupName,
		CategoryName:    result.CategoryName,
		StudentCount:    result.StudentCount,
		SupervisorNames: result.SupervisorNames,
	}, nil
}

// CreateRoom creates a new room
func (s *service) CreateRoom(ctx context.Context, room *facilities.Room) error {
	// Validate room data
	if err := room.Validate(); err != nil {
		return &FacilitiesError{Op: opCreateRoom, Err: translateValidationError(err)}
	}

	// Set tenant ID from context
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		room.SetTenantID(tenantID)
	}

	// Check if a room with the same name already exists
	existing, err := s.roomRepo.FindByName(ctx, room.Name)
	if err == nil && existing != nil {
		return &FacilitiesError{Op: opCreateRoom, Err: ErrDuplicateRoom}
	}

	// Create the room
	if err := s.roomRepo.Create(ctx, room); err != nil {
		if isUniqueColorViolation(err) {
			return &FacilitiesError{Op: opCreateRoom, Err: ErrColorAlreadyInUse}
		}
		return &FacilitiesError{Op: opCreateRoom, Err: err}
	}

	return nil
}

// translateValidationError maps Validate() sentinels to the service-level
// errors that the API layer can render with the correct HTTP status. Any
// other validation error is forwarded unchanged so the existing
// ErrorInvalidRequest path renders 400.
func translateValidationError(err error) error {
	if errors.Is(err, facilities.ErrReservedColor) {
		return ErrColorReserved
	}
	return err
}

// isUniqueColorViolation reports whether err is a PostgreSQL 23505 raised by
// the partial unique index on facilities.rooms (tenant_id, lower(color)). The
// constraint name is checked so other future unique indexes on the table do
// not accidentally surface as "color already in use" toasts.
func isUniqueColorViolation(err error) bool {
	if err == nil {
		return false
	}
	var dbErr *base.DatabaseError
	if errors.As(err, &dbErr) {
		err = dbErr.Err
	}
	var pgErr pgdriver.Error
	if !errors.As(err, &pgErr) {
		return false
	}
	if pgErr.Field('C') != "23505" {
		return false
	}
	return pgErr.Field('n') == facilities.RoomColorUniqueConstraintName
}

// equalStringPtr compares two *string for equality treating nil as "no value"
// — used to detect "color actually changed" without false-positives from the
// nil/empty distinction.
func equalStringPtr(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// UpdateRoom updates an existing room
func (s *service) UpdateRoom(ctx context.Context, room *facilities.Room) error {
	// Validate room data
	if err := room.Validate(); err != nil {
		return &FacilitiesError{Op: opUpdateRoom, Err: translateValidationError(err)}
	}

	// Check if room exists
	existingRoom, err := s.roomRepo.FindByID(ctx, room.ID)
	if err != nil {
		return &FacilitiesError{Op: opUpdateRoom, Err: ErrRoomNotFound}
	}

	// Block renaming system rooms (Schulhof, WC)
	if constants.IsSystemRoomName(existingRoom.Name) && room.Name != existingRoom.Name {
		return &FacilitiesError{Op: opUpdateRoom, Err: ErrSystemRoomProtected}
	}

	// System-room color handling.
	//
	// The frontend strips the color picker for Schulhof/WC, so a benign edit
	// (e.g. changing capacity) arrives with room.Color == nil regardless of
	// what's persisted. If we treated nil as "user wants to clear", every
	// non-color edit on a Schulhof that still carries the legacy #4F46E5
	// bug-default would 403 — the migration covers most cases but not all
	// (e.g. a tenant that hasn't run migrations yet, or a system room
	// imported with a colour later).
	//
	// Strategy: treat a nil incoming colour as "request did not touch the
	// colour field" and silently preserve the existing value. Only block
	// when the request explicitly sets a different non-nil colour — that's
	// the path a malicious or buggy direct API call would take, and the one
	// the rule actually exists to stop.
	//
	// Side-effect of this strategy: a system room cannot have its colour
	// *cleared* via the API. If a Schulhof somehow carries a stale colour
	// (legacy bug-default that escaped migration cleanup, or an imported
	// dataset), the only way to drop it is direct SQL. Acceptable trade-off
	// — system rooms shouldn't have admin-set colours anyway, and the
	// alternative is admins losing the ability to edit any other field.
	//
	// Why both `room.Color != nil` AND equalStringPtr?
	//   - Inline `*room.Color != *existingRoom.Color` would NPE when the
	//     existing colour is nil and the request sets a new one (case A).
	//   - Dropping the outer guard would re-block benign updates whose
	//     incoming colour is nil but existing is non-nil (the very bug
	//     this comment block exists to fix).
	// Both checks together give: "block only when the request explicitly
	// names a different non-nil colour, ignore colour-field absence".
	if constants.IsSystemRoomName(existingRoom.Name) {
		if room.Color != nil && !equalStringPtr(room.Color, existingRoom.Color) {
			return &FacilitiesError{Op: opUpdateRoom, Err: ErrSystemRoomProtected}
		}
		room.Color = existingRoom.Color
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
		if isUniqueColorViolation(err) {
			return &FacilitiesError{Op: opUpdateRoom, Err: ErrColorAlreadyInUse}
		}
		return &FacilitiesError{Op: opUpdateRoom, Err: err}
	}

	return nil
}

// DeleteRoom deletes a room by its ID
func (s *service) DeleteRoom(ctx context.Context, id int64) error {
	// Check if room exists
	existingRoom, err := s.roomRepo.FindByID(ctx, id)
	if err != nil {
		return &FacilitiesError{Op: "delete room", Err: ErrRoomNotFound}
	}

	// Block deletion of system rooms (Schulhof, WC)
	if constants.IsSystemRoomName(existingRoom.Name) {
		return &FacilitiesError{Op: "delete room", Err: ErrSystemRoomProtected}
	}

	// Best-effort pre-check: active groups would block deletion via FK RESTRICT.
	// The real protection is the DB constraint; this gives a user-friendly error message.
	activeGroups, preCheckErr := s.activeGroupRepo.FindActiveByRoomID(ctx, id)
	if preCheckErr != nil {
		slog.Warn("room_delete_precheck_failed",
			"room_id", id,
			"error", preCheckErr.Error(),
		)
	} else if len(activeGroups) > 0 {
		return &FacilitiesError{Op: "delete room", Err: ErrRoomInUse}
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
		IsOccupied      bool    `bun:"is_occupied"`
		GroupName       *string `bun:"group_name"`
		CategoryName    *string `bun:"category_name"`
		StudentCount    int     `bun:"student_count"`
		SupervisorNames *string `bun:"supervisor_names"`
	}

	// Build query with LEFT JOINs for occupancy information
	// Use DISTINCT ON to handle rooms with multiple active groups (e.g., Schulhof with Freispiel + Garten)
	query := repoBase.GetDB(ctx, s.db).NewSelect().
		TableExpr("facilities.rooms AS r").
		DistinctOn("r.id").
		ColumnExpr("r.id, r.name, r.building, r.floor, r.capacity, r.category, r.color, r.created_at, r.updated_at").
		ColumnExpr("CASE WHEN ag.id IS NOT NULL THEN true ELSE false END AS is_occupied").
		ColumnExpr("act_group.name AS group_name").
		ColumnExpr("cat.name AS category_name").
		// Student count: count active visits for this room's active group
		ColumnExpr(`COALESCE((
			SELECT COUNT(DISTINCT v.student_id)
			FROM active.visits v
			WHERE v.active_group_id = ag.id AND v.exit_time IS NULL
		), 0)::int AS student_count`).
		// Supervisor names: aggregate staff names for this room's active group
		ColumnExpr(`(
			SELECT string_agg(DISTINCT CONCAT(p.first_name, ' ', p.last_name), ', ')
			FROM active.group_supervisors gs
			INNER JOIN users.staff st ON st.id = gs.staff_id
			INNER JOIN users.persons p ON p.id = st.person_id
			WHERE gs.group_id = ag.id AND gs.end_date IS NULL
		) AS supervisor_names`).
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
			IsOccupied:      r.IsOccupied,
			GroupName:       r.GroupName,
			CategoryName:    r.CategoryName,
			StudentCount:    r.StudentCount,
			SupervisorNames: r.SupervisorNames,
		}
	}

	return roomsWithOccupancy, nil
}

// FindRoomByName finds a room by its name
func (s *service) FindRoomByName(ctx context.Context, name string) (*facilities.Room, error) {
	room, err := s.roomRepo.FindByName(ctx, name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &FacilitiesError{Op: "find room by name", Err: ErrRoomNotFound}
		}
		return nil, &FacilitiesError{Op: "find room by name", Err: err}
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
	roomsWithOccupancy := make([]RoomWithOccupancy, 0, len(availableRooms))
	for _, room := range availableRooms {
		roomsWithOccupancy = append(roomsWithOccupancy, RoomWithOccupancy{
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
	err = repoBase.GetDB(ctx, s.db).NewSelect().
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
