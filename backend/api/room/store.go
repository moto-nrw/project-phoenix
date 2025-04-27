package room

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/moto-nrw/project-phoenix/database"
	models2 "github.com/moto-nrw/project-phoenix/models"
	"time"

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
	db        *bun.DB
	baseStore *database.RoomStore
}

// NewRoomStore returns a new RoomStore implementation
func NewRoomStore(db *bun.DB) RoomStore {
	return &roomStore{
		db:        db,
		baseStore: database.NewRoomStore(db),
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
	// 1. Query all active RoomOccupancy entries
	var occupancies []RoomOccupancy
	err := s.db.NewSelect().
		Model(&occupancies).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	// 2. For each occupancy, construct a RoomOccupancyDetail
	var details []RoomOccupancyDetail
	for _, occupancy := range occupancies {
		// Get room
		room := new(models2.Room)
		err := s.db.NewSelect().
			Model(room).
			Where("id = ?", occupancy.RoomID).
			Scan(ctx)
		if err != nil {
			continue // Skip if we can't find the room
		}

		// Get timespan
		timespan := new(models2.Timespan)
		err = s.db.NewSelect().
			Model(timespan).
			Where("id = ?", occupancy.TimespanID).
			Scan(ctx)
		if err != nil {
			continue // Skip if we can't find the timespan
		}

		// Only include active timespans (no end time or end time in the future)
		if timespan.EndTime != nil && timespan.EndTime.Before(time.Now()) {
			continue
		}

		// Get supervisors
		var supervisors []SupervisorInfo
		err = s.db.NewSelect().
			Table("room_occupancy_supervisors").
			Column("specialist_id AS id").
			Where("room_occupancy_id = ?", occupancy.ID).
			Scan(ctx, &supervisors)
		if err != nil {
			supervisors = []SupervisorInfo{} // Continue without supervisors
		}

		// Construct RoomOccupancyDetail
		detail := RoomOccupancyDetail{
			Room: RoomInfo{
				RoomName: room.RoomName,
				Floor:    room.Floor,
				Capacity: room.Capacity,
			},
			Supervisor: supervisors,
			Timespan: TimespanInfo{
				StartTime: timespan.StartTime.Format("15:04"),
			},
		}

		if timespan.EndTime != nil {
			endTimeStr := timespan.EndTime.Format("15:04")
			detail.Timespan.EndTime = endTimeStr
		}

		details = append(details, detail)
	}

	return details, nil
}

// GetRoomOccupancyByID returns room occupancy details by ID
func (s *roomStore) GetRoomOccupancyByID(ctx context.Context, id int64) (*RoomOccupancyDetail, error) {
	// 1. Query RoomOccupancy by ID
	occupancy := new(RoomOccupancy)
	err := s.db.NewSelect().
		Model(occupancy).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	// 2. Get Room
	room := new(models2.Room)
	err = s.db.NewSelect().
		Model(room).
		Where("id = ?", occupancy.RoomID).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	// 3. Get Timespan
	timespan := new(models2.Timespan)
	err = s.db.NewSelect().
		Model(timespan).
		Where("id = ?", occupancy.TimespanID).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	// 4. Get Supervisors
	var supervisors []SupervisorInfo
	err = s.db.NewSelect().
		Table("room_occupancy_supervisors").
		Column("specialist_id AS id").
		Where("room_occupancy_id = ?", occupancy.ID).
		Scan(ctx, &supervisors)
	if err != nil {
		supervisors = []SupervisorInfo{} // Continue without supervisors
	}

	// 5. Construct RoomOccupancyDetail
	detail := &RoomOccupancyDetail{
		Room: RoomInfo{
			RoomName: room.RoomName,
			Floor:    room.Floor,
			Capacity: room.Capacity,
		},
		Supervisor: supervisors,
		Timespan: TimespanInfo{
			StartTime: timespan.StartTime.Format("15:04"),
		},
	}

	if timespan.EndTime != nil {
		endTimeStr := timespan.EndTime.Format("15:04")
		detail.Timespan.EndTime = endTimeStr
	}

	// AG info would be added when AG model is implemented

	return detail, nil
}

// GetCurrentRoomOccupancy returns current occupancy for a room
func (s *roomStore) GetCurrentRoomOccupancy(ctx context.Context, roomID int64) (*RoomOccupancyDetail, error) {
	// 1. Query Room to make sure it exists
	room, err := s.baseStore.GetRoomByID(ctx, roomID)
	if err != nil {
		return nil, err
	}

	// 2. Query RoomOccupancy by roomID
	occupancy := new(RoomOccupancy)
	err = s.db.NewSelect().
		Model(occupancy).
		Where("room_id = ?", roomID).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("room is not currently occupied")
	}

	// 3. Get Timespan for the occupancy
	timespan := new(models2.Timespan)
	err = s.db.NewSelect().
		Model(timespan).
		Where("id = ?", occupancy.TimespanID).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	// Only include active timespans (no end time or end time in the future)
	if timespan.EndTime != nil && timespan.EndTime.Before(time.Now()) {
		return nil, fmt.Errorf("room is not currently occupied")
	}

	// 4. Get supervisors for the occupancy
	var supervisors []SupervisorInfo
	err = s.db.NewSelect().
		Table("room_occupancy_supervisors").
		Column("specialist_id AS id").
		Where("room_occupancy_id = ?", occupancy.ID).
		Scan(ctx, &supervisors)
	if err != nil {
		// Continue without supervisors if we can't fetch them
		supervisors = []SupervisorInfo{}
	}

	// For now, we'll return limited supervisor info as we don't have access to the specialist model yet

	// 5. Construct RoomOccupancyDetail object
	detail := &RoomOccupancyDetail{
		Room: RoomInfo{
			RoomName: room.RoomName,
			Floor:    room.Floor,
			Capacity: room.Capacity,
		},
		Supervisor: supervisors,
		Timespan: TimespanInfo{
			StartTime: timespan.StartTime.Format("15:04"),
		},
	}

	if timespan.EndTime != nil {
		endTimeStr := timespan.EndTime.Format("15:04")
		detail.Timespan.EndTime = endTimeStr
	}

	// We'll add AG info when AG model is implemented

	return detail, nil
}

// RegisterTablet registers a tablet to a room
func (s *roomStore) RegisterTablet(ctx context.Context, roomID int64, req *RegisterTabletRequest) (*RoomOccupancy, error) {
	// Start a transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 1. Check if room exists
	_, err = s.baseStore.GetRoomByID(ctx, roomID)
	if err != nil {
		return nil, err
	}

	// 2. Check if tablet is already registered
	var existingOccupancy RoomOccupancy
	err = tx.NewSelect().
		Model(&existingOccupancy).
		Where("device_id = ?", req.DeviceID).
		Scan(ctx)
	if err == nil {
		// Tablet is already registered
		return nil, fmt.Errorf("tablet is already registered")
	}

	// 3. Create a timespan
	timespan := &models2.Timespan{
		StartTime: time.Now(),
		CreatedAt: time.Now(),
	}
	_, err = tx.NewInsert().
		Model(timespan).
		Exec(ctx)
	if err != nil {
		return nil, err
	}

	// 4. Create a new AG if requested (placeholder for now)
	var agID int64
	if req.NewAg != nil {
		// AG creation logic would go here
		// For now, we'll just use the provided AgID if any
		if req.AgID != nil {
			agID = *req.AgID
		}
	} else if req.AgID != nil {
		agID = *req.AgID
	}

	// 5. Create RoomOccupancy entry
	occupancy := &RoomOccupancy{
		DeviceID:   req.DeviceID,
		RoomID:     roomID,
		TimespanID: timespan.ID,
		CreatedAt:  time.Now(),
	}

	if req.GroupID != nil {
		occupancy.GroupID = *req.GroupID
	}

	if agID != 0 {
		occupancy.AgID = agID
	}

	_, err = tx.NewInsert().
		Model(occupancy).
		Exec(ctx)
	if err != nil {
		return nil, err
	}

	// 6. Add supervisors to RoomOccupancySupervisor table
	for _, supervisorID := range req.Supervisors {
		supervisor := &RoomOccupancySupervisor{
			RoomOccupancyID: occupancy.ID,
			SpecialistID:    supervisorID,
			CreatedAt:       time.Now(),
		}
		_, err = tx.NewInsert().
			Model(supervisor).
			Exec(ctx)
		if err != nil {
			return nil, err
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return occupancy, nil
}

// UnregisterTablet unregisters a tablet from a room
func (s *roomStore) UnregisterTablet(ctx context.Context, roomID int64, deviceID string) error {
	// Start a transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Find RoomOccupancy by roomID and deviceID
	occupancy := new(RoomOccupancy)
	err = tx.NewSelect().
		Model(occupancy).
		Where("room_id = ? AND device_id = ?", roomID, deviceID).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("tablet not registered to this room")
	}

	// Get the timespan to update its end time
	timespan := new(models2.Timespan)
	err = tx.NewSelect().
		Model(timespan).
		Where("id = ?", occupancy.TimespanID).
		Scan(ctx)
	if err != nil {
		return err
	}

	// Update the timespan to mark the end time
	endTime := time.Now()
	timespan.EndTime = &endTime
	_, err = tx.NewUpdate().
		Model(timespan).
		Column("endtime").
		Where("id = ?", timespan.ID).
		Exec(ctx)
	if err != nil {
		return err
	}

	// 2. Delete related RoomOccupancySupervisor entries
	_, err = tx.NewDelete().
		Model((*RoomOccupancySupervisor)(nil)).
		Where("room_occupancy_id = ?", occupancy.ID).
		Exec(ctx)
	if err != nil {
		return err
	}

	// 3. Delete RoomOccupancy entry
	_, err = tx.NewDelete().
		Model(occupancy).
		WherePK().
		Exec(ctx)
	if err != nil {
		return err
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

// AddSupervisorToRoomOccupancy adds a supervisor to a room occupancy
func (s *roomStore) AddSupervisorToRoomOccupancy(ctx context.Context, roomOccupancyID, supervisorID int64) error {
	supervisor := &RoomOccupancySupervisor{
		RoomOccupancyID: roomOccupancyID,
		SpecialistID:    supervisorID,
		CreatedAt:       time.Now(),
	}

	_, err := s.db.NewInsert().
		Model(supervisor).
		Exec(ctx)

	return err
}

// MergeRooms merges two rooms and creates a combined group
func (s *roomStore) MergeRooms(ctx context.Context, sourceRoomID, targetRoomID int64, name string, validUntil *time.Time, accessPolicy string) (*models2.CombinedGroup, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Get source room
	sourceRoom, err := s.baseStore.GetRoomByID(ctx, sourceRoomID)
	if err != nil {
		return nil, fmt.Errorf("source room not found: %w", err)
	}

	// Get target room
	targetRoom, err := s.baseStore.GetRoomByID(ctx, targetRoomID)
	if err != nil {
		return nil, fmt.Errorf("target room not found: %w", err)
	}

	// Get groups for source room
	var sourceGroups []models2.Group
	err = tx.NewSelect().
		Model(&sourceGroups).
		Where("room_id = ?", sourceRoomID).
		Scan(ctx)

	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	// Get groups for target room
	var targetGroups []models2.Group
	err = tx.NewSelect().
		Model(&targetGroups).
		Where("room_id = ?", targetRoomID).
		Scan(ctx)

	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	// If both rooms have no associated groups, we can't merge them
	if len(sourceGroups) == 0 && len(targetGroups) == 0 {
		return nil, fmt.Errorf("no groups found for either room")
	}

	// Generate default name if not provided
	if name == "" {
		name = fmt.Sprintf("%s + %s", sourceRoom.RoomName, targetRoom.RoomName)
	}

	// Use default access policy if not provided
	if accessPolicy == "" {
		accessPolicy = "all" // All supervisors from both groups have access
	}

	// Create a combined group
	combinedGroup := &models2.CombinedGroup{
		Name:         name,
		IsActive:     true,
		ValidUntil:   validUntil,
		AccessPolicy: accessPolicy,
		CreatedAt:    time.Now(),
	}

	_, err = tx.NewInsert().
		Model(combinedGroup).
		Exec(ctx)

	if err != nil {
		return nil, err
	}

	// Add all groups to the combined group
	allGroups := append(sourceGroups, targetGroups...)
	addedGroupIDs := make(map[int64]bool)

	for _, group := range allGroups {
		// Skip duplicate groups
		if addedGroupIDs[group.ID] {
			continue
		}

		combinedGroupGroup := &models2.CombinedGroupGroup{
			CombinedGroupID: combinedGroup.ID,
			GroupID:         group.ID,
			CreatedAt:       time.Now(),
		}

		_, err = tx.NewInsert().
			Model(combinedGroupGroup).
			Exec(ctx)

		if err != nil {
			return nil, err
		}

		addedGroupIDs[group.ID] = true
	}

	// Collect all supervisors from all groups
	var allSupervisorIDs []int64

	for _, group := range allGroups {
		var supervisors []struct {
			SpecialistID int64 `bun:"specialist_id"`
		}

		err = tx.NewSelect().
			TableExpr("group_supervisors").
			Column("specialist_id").
			Where("group_id = ?", group.ID).
			Scan(ctx, &supervisors)

		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}

		for _, supervisor := range supervisors {
			allSupervisorIDs = append(allSupervisorIDs, supervisor.SpecialistID)
		}
	}

	// Add all unique supervisors to the combined group
	addedSpecialistIDs := make(map[int64]bool)

	for _, specialistID := range allSupervisorIDs {
		if !addedSpecialistIDs[specialistID] {
			combinedGroupSpecialist := &models2.CombinedGroupSpecialist{
				CombinedGroupID: combinedGroup.ID,
				SpecialistID:    specialistID,
				CreatedAt:       time.Now(),
			}

			_, err = tx.NewInsert().
				Model(combinedGroupSpecialist).
				Exec(ctx)

			if err != nil {
				return nil, err
			}

			addedSpecialistIDs[specialistID] = true
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Fetch the complete combined group with relations
	result := new(models2.CombinedGroup)
	err = s.db.NewSelect().
		Model(result).
		Relation("Groups").
		Relation("AccessSpecialists").
		Relation("AccessSpecialists.CustomUser").
		Where("id = ?", combinedGroup.ID).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return result, nil
}

// GetCombinedGroupForRoom retrieves the combined group that includes a room
func (s *roomStore) GetCombinedGroupForRoom(ctx context.Context, roomID int64) (*models2.CombinedGroup, error) {
	// First find groups associated with the room
	var groups []models2.Group
	err := s.db.NewSelect().
		Model(&groups).
		Where("room_id = ?", roomID).
		Scan(ctx)

	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	if len(groups) == 0 {
		return nil, fmt.Errorf("no group found for room %d", roomID)
	}

	// For each group, find if it's part of an active combined group
	for _, group := range groups {
		var combinedGroupIDs []int64
		err = s.db.NewSelect().
			TableExpr("combined_group_groups cgg").
			Column("cgg.combinedgroup_id").
			Join("JOIN combined_groups cg ON cg.id = cgg.combinedgroup_id").
			Where("cgg.group_id = ? AND cg.is_active = ?", group.ID, true).
			Scan(ctx, &combinedGroupIDs)

		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}

		// If this group is part of a combined group, return the first one
		if len(combinedGroupIDs) > 0 {
			combinedGroup := new(models2.CombinedGroup)
			err = s.db.NewSelect().
				Model(combinedGroup).
				Relation("Groups").
				Relation("AccessSpecialists").
				Relation("AccessSpecialists.CustomUser").
				Where("id = ? AND is_active = ?", combinedGroupIDs[0], true).
				Scan(ctx)

			if err != nil {
				return nil, err
			}

			return combinedGroup, nil
		}
	}

	// If we get here, no active combined groups were found
	return nil, fmt.Errorf("no active combined group found for room %d", roomID)
}

// FindActiveCombinedGroups returns all active combined groups
func (s *roomStore) FindActiveCombinedGroups(ctx context.Context) ([]models2.CombinedGroup, error) {
	var combinedGroups []models2.CombinedGroup

	err := s.db.NewSelect().
		Model(&combinedGroups).
		Relation("Groups").
		Relation("AccessSpecialists").
		Relation("AccessSpecialists.CustomUser").
		Where("is_active = ?", true).
		OrderExpr("name ASC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	// Filter out expired combined groups
	var activeGroups []models2.CombinedGroup
	now := time.Now()

	for _, group := range combinedGroups {
		// If ValidUntil is set and it's in the past, mark as inactive
		if group.ValidUntil != nil && group.ValidUntil.Before(now) {
			// Update the group to set IsActive to false
			group.IsActive = false
			_, err = s.db.NewUpdate().
				Model(&group).
				Column("is_active").
				WherePK().
				Exec(ctx)

			// Don't include in results even if update fails
			continue
		}

		activeGroups = append(activeGroups, group)
	}

	return activeGroups, nil
}

// DeactivateCombinedGroup deactivates a combined group
func (s *roomStore) DeactivateCombinedGroup(ctx context.Context, id int64) error {
	// Get the combined group
	combinedGroup := new(models2.CombinedGroup)
	err := s.db.NewSelect().
		Model(combinedGroup).
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		return err
	}

	// Update to set inactive
	combinedGroup.IsActive = false
	_, err = s.db.NewUpdate().
		Model(combinedGroup).
		Column("is_active").
		WherePK().
		Exec(ctx)

	return err
}
