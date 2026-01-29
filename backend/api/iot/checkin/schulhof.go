package checkin

import (
	"context"
	"fmt"
	"log"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
)

// ensureSchulhofRoom finds or creates the Schulhof room
func (rs *Resource) ensureSchulhofRoom(ctx context.Context) (*facilities.Room, error) {
	// Try to find existing Schulhof room
	room, err := rs.FacilityService.FindRoomByName(ctx, constants.SchulhofRoomName)
	if err == nil && room != nil {
		log.Printf("%s Found existing room: ID=%d", constants.SchulhofLogPrefix, room.ID)
		return room, nil
	}

	// Room not found - create it
	log.Printf("%s Room not found, auto-creating...", constants.SchulhofLogPrefix)

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

	log.Printf("%s Successfully auto-created room: ID=%d", constants.SchulhofLogPrefix, newRoom.ID)
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
			log.Printf("%s Found existing category: ID=%d", constants.SchulhofLogPrefix, cat.ID)
			return cat, nil
		}
	}

	// Category not found - create it
	log.Printf("%s Category not found, auto-creating...", constants.SchulhofLogPrefix)

	newCategory := &activities.Category{
		Name:        constants.SchulhofCategoryName,
		Description: constants.SchulhofCategoryDescription,
		Color:       constants.SchulhofColor,
	}

	createdCategory, err := rs.ActivitiesService.CreateCategory(ctx, newCategory)
	if err != nil {
		return nil, fmt.Errorf("failed to auto-create Schulhof category: %w", err)
	}

	log.Printf("%s Successfully auto-created category: ID=%d", constants.SchulhofLogPrefix, createdCategory.ID)
	return createdCategory, nil
}

// schulhofActivityGroup finds or creates the permanent Schulhof activity group.
// This function implements lazy initialization - it will auto-create the Schulhof
// infrastructure (room, category, activity) on first use if not found.
func (rs *Resource) schulhofActivityGroup(ctx context.Context) (*activities.Group, error) {
	// Build filter for Schulhof activity using constant
	// WithTableAlias("group") is applied in the repository, so use unqualified field name
	options := base.NewQueryOptions()
	filter := base.NewFilter()
	filter.Equal("name", constants.SchulhofActivityName)
	options.Filter = filter

	// Query activities service
	groups, err := rs.ActivitiesService.ListGroups(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to query Schulhof activity: %w", err)
	}

	// If activity exists, return it
	if len(groups) > 0 {
		log.Printf("%s Found existing activity: ID=%d", constants.SchulhofLogPrefix, groups[0].ID)
		return groups[0], nil
	}

	// Activity not found - auto-create the entire Schulhof infrastructure
	log.Printf("%s Activity not found, auto-creating infrastructure...", constants.SchulhofLogPrefix)

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

	log.Printf("%s Successfully auto-created infrastructure: room=%d, category=%d, activity=%d",
		constants.SchulhofLogPrefix, room.ID, category.ID, createdActivity.ID)

	return createdActivity, nil
}
