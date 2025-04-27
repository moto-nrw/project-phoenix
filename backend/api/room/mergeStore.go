package room

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/models"
	"github.com/uptrace/bun"
)

// MergeStore defines database operations for room merging functionality
type MergeStore interface {
	// Room merging operations
	MergeRooms(ctx context.Context, sourceRoomID, targetRoomID int64, name string, validUntil *time.Time, accessPolicy string) (*models.CombinedGroup, error)
	GetCombinedGroupForRoom(ctx context.Context, roomID int64) (*models.CombinedGroup, error)
	FindActiveCombinedGroups(ctx context.Context) ([]models.CombinedGroup, error)
	DeactivateCombinedGroup(ctx context.Context, id int64) error
}

type mergeStore struct {
	db *bun.DB
}

// NewMergeStore returns a new MergeStore implementation
func NewMergeStore(db *bun.DB) MergeStore {
	return &mergeStore{
		db: db,
	}
}

// MergeRooms merges two rooms and creates a combined group
func (s *mergeStore) MergeRooms(ctx context.Context, sourceRoomID, targetRoomID int64, name string, validUntil *time.Time, accessPolicy string) (*models.CombinedGroup, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Get source room
	sourceRoom := new(models.Room)
	err = tx.NewSelect().
		Model(sourceRoom).
		Where("id = ?", sourceRoomID).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("source room not found: %w", err)
	}

	// Get target room
	targetRoom := new(models.Room)
	err = tx.NewSelect().
		Model(targetRoom).
		Where("id = ?", targetRoomID).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("target room not found: %w", err)
	}

	// Get groups for source room
	var sourceGroups []models.Group
	err = tx.NewSelect().
		Model(&sourceGroups).
		Where("room_id = ?", sourceRoomID).
		Scan(ctx)

	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	// Get groups for target room
	var targetGroups []models.Group
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
	combinedGroup := &models.CombinedGroup{
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

		combinedGroupGroup := &models.CombinedGroupGroup{
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
			combinedGroupSpecialist := &models.CombinedGroupSpecialist{
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
	result := new(models.CombinedGroup)
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
func (s *mergeStore) GetCombinedGroupForRoom(ctx context.Context, roomID int64) (*models.CombinedGroup, error) {
	// First find groups associated with the room
	var groups []models.Group
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
			combinedGroup := new(models.CombinedGroup)
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
func (s *mergeStore) FindActiveCombinedGroups(ctx context.Context) ([]models.CombinedGroup, error) {
	var combinedGroups []models.CombinedGroup

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
	var activeGroups []models.CombinedGroup
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
func (s *mergeStore) DeactivateCombinedGroup(ctx context.Context, id int64) error {
	// Get the combined group
	combinedGroup := new(models.CombinedGroup)
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
