package checkin

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/activities"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/facilities"
)

// ensureSchulhofRoom finds or creates the Schulhof room
func (rs *Resource) ensureSchulhofRoom(ctx context.Context) (*facilities.Room, error) {
	// Try to find existing Schulhof room
	room, err := rs.FacilityService.FindRoomByName(ctx, activities.SchulhofRoomName)
	if err == nil && room != nil {
		return room, nil
	}

	// Room not found - create it
	capacity := activities.SchulhofRoomCapacity
	category := activities.SchulhofCategoryName
	color := activities.SchulhofColor

	newRoom := &facilities.Room{
		Name:     activities.SchulhofRoomName,
		Capacity: &capacity,
		Category: &category,
		Color:    &color,
	}

	if err := rs.FacilityService.CreateRoom(ctx, newRoom); err != nil {
		return nil, fmt.Errorf("failed to auto-create Schulhof room: %w", err)
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
		if cat.Name == activities.SchulhofCategoryName {
			return cat, nil
		}
	}

	// Category not found - create it
	newCategory := &activities.Category{
		Name:        activities.SchulhofCategoryName,
		Description: activities.SchulhofCategoryDescription,
		Color:       activities.SchulhofColor,
	}

	createdCategory, err := rs.ActivitiesService.CreateCategory(ctx, newCategory)
	if err != nil {
		return nil, fmt.Errorf("failed to auto-create Schulhof category: %w", err)
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
	filter.Equal("group.name", activities.SchulhofActivityName)
	options.Filter = filter

	// Query activities service
	groups, err := rs.ActivitiesService.ListGroups(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to query Schulhof activity: %w", err)
	}

	// If activity exists, return it
	if len(groups) > 0 {
		return groups[0], nil
	}

	// Activity not found - auto-create the entire Schulhof infrastructure
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
		Name:            activities.SchulhofActivityName,
		MaxParticipants: activities.SchulhofMaxParticipants,
		IsOpen:          true, // Open activity - anyone can join
		CategoryID:      category.ID,
		PlannedRoomID:   &room.ID,
	}

	// CreateGroup requires supervisorIDs and schedules - pass empty slices for auto-created activity
	createdActivity, err := rs.ActivitiesService.CreateGroup(ctx, newActivity, []int64{}, []*activities.Schedule{})
	if err != nil {
		return nil, fmt.Errorf("failed to auto-create Schulhof activity: %w", err)
	}

	return createdActivity, nil
}
