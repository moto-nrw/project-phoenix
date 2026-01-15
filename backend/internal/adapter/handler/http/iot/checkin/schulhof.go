package checkin

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/activities"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/facilities"
)

// ensureSchulhofRoom finds or creates the Schulhof room
func (rs *Resource) ensureSchulhofRoom(ctx context.Context) (*facilities.Room, error) {
	// Try to find existing Schulhof room
	room, err := rs.FacilityService.FindRoomByName(ctx, constants.SchulhofRoomName)
	if err == nil && room != nil {
		if logger.Logger != nil {
			logger.Logger.WithField("room_id", room.ID).Debug("Schulhof: Found existing room")
		}
		return room, nil
	}

	// Room not found - create it
	if logger.Logger != nil {
		logger.Logger.Debug("Schulhof: Room not found, auto-creating...")
	}

	capacity := constants.SchulhofRoomCapacity
	category := constants.SchulhofCategoryName
	color := constants.SchulhofColor

	newRoom := &facilities.Room{
		Name:     constants.SchulhofRoomName,
		Capacity: &capacity,
		Category: &category,
		Color:    &color,
	}

	if err := rs.FacilityService.CreateRoom(ctx, newRoom); err != nil {
		return nil, fmt.Errorf("failed to auto-create Schulhof room: %w", err)
	}

	if logger.Logger != nil {
		logger.Logger.WithField("room_id", newRoom.ID).Info("Schulhof: Successfully auto-created room")
	}
	return newRoom, nil
}

// ensureSchulhofCategory finds or creates the Schulhof activity category
func (rs *Resource) ensureSchulhofCategory(ctx context.Context) (*activities.Category, error) {
	// Try to find existing Schulhof category
	categories, err := rs.ActivitiesService.ListCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list activity categories: %w", err)
	}

	for _, cat := range categories {
		if cat.Name == constants.SchulhofCategoryName {
			if logger.Logger != nil {
				logger.Logger.WithField("category_id", cat.ID).Debug("Schulhof: Found existing category")
			}
			return cat, nil
		}
	}

	// Category not found - create it
	if logger.Logger != nil {
		logger.Logger.Debug("Schulhof: Category not found, auto-creating...")
	}

	newCategory := &activities.Category{
		Name:        constants.SchulhofCategoryName,
		Description: constants.SchulhofCategoryDescription,
		Color:       constants.SchulhofColor,
	}

	createdCategory, err := rs.ActivitiesService.CreateCategory(ctx, newCategory)
	if err != nil {
		return nil, fmt.Errorf("failed to auto-create Schulhof category: %w", err)
	}

	if logger.Logger != nil {
		logger.Logger.WithField("category_id", createdCategory.ID).Info("Schulhof: Successfully auto-created category")
	}
	return createdCategory, nil
}

// schulhofActivityGroup finds or creates the permanent Schulhof activity group.
// This function implements lazy initialization - it will auto-create the Schulhof
// infrastructure (room, category, activity) on first use if not found.
func (rs *Resource) schulhofActivityGroup(ctx context.Context) (*activities.Group, error) {
	// Build filter for Schulhof activity using constant
	// Use qualified column name to avoid ambiguity with category.name
	options := base.NewQueryOptions()
	filter := base.NewFilter()
	filter.Equal("group.name", constants.SchulhofActivityName)
	options.Filter = filter

	// Query activities service
	groups, err := rs.ActivitiesService.ListGroups(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to query Schulhof activity: %w", err)
	}

	// If activity exists, return it
	if len(groups) > 0 {
		if logger.Logger != nil {
			logger.Logger.WithField("activity_id", groups[0].ID).Debug("Schulhof: Found existing activity")
		}
		return groups[0], nil
	}

	// Activity not found - auto-create the entire Schulhof infrastructure
	if logger.Logger != nil {
		logger.Logger.Debug("Schulhof: Activity not found, auto-creating infrastructure...")
	}

	// Step 1: Ensure Schulhof room exists
	room, err := rs.ensureSchulhofRoom(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure Schulhof room: %w", err)
	}

	// Step 2: Ensure Schulhof category exists
	category, err := rs.ensureSchulhofCategory(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure Schulhof category: %w", err)
	}

	// Step 3: Create the Schulhof activity group
	newActivity := &activities.Group{
		Name:            constants.SchulhofActivityName,
		MaxParticipants: constants.SchulhofMaxParticipants,
		IsOpen:          true, // Open activity - anyone can join
		CategoryID:      category.ID,
		PlannedRoomID:   &room.ID,
	}

	// CreateGroup requires supervisorIDs and schedules - pass empty slices for auto-created activity
	createdActivity, err := rs.ActivitiesService.CreateGroup(ctx, newActivity, []int64{}, []*activities.Schedule{})
	if err != nil {
		return nil, fmt.Errorf("failed to auto-create Schulhof activity: %w", err)
	}

	if logger.Logger != nil {
		logger.Logger.WithFields(map[string]interface{}{
			"room_id":     room.ID,
			"category_id": category.ID,
			"activity_id": createdActivity.ID,
		}).Info("Schulhof: Successfully auto-created infrastructure")
	}

	return createdActivity, nil
}
