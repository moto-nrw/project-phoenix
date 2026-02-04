package checkin

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/auth/device"
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
		rs.getLogger().DebugContext(ctx, "found existing Schulhof room",
			slog.Int64("room_id", room.ID),
		)
		return room, nil
	}

	// Room not found - create it
	rs.getLogger().InfoContext(ctx, "Schulhof room not found, auto-creating")

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

	rs.getLogger().InfoContext(ctx, "successfully auto-created Schulhof room",
		slog.Int64("room_id", newRoom.ID),
	)
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
			rs.getLogger().DebugContext(ctx, "found existing Schulhof category",
				slog.Int64("category_id", cat.ID),
			)
			return cat, nil
		}
	}

	// Category not found - create it
	rs.getLogger().InfoContext(ctx, "Schulhof category not found, auto-creating")

	newCategory := &activities.Category{
		Name:        constants.SchulhofCategoryName,
		Description: constants.SchulhofCategoryDescription,
		Color:       constants.SchulhofColor,
	}

	createdCategory, err := rs.ActivitiesService.CreateCategory(ctx, newCategory)
	if err != nil {
		return nil, fmt.Errorf("failed to auto-create Schulhof category: %w", err)
	}

	rs.getLogger().InfoContext(ctx, "successfully auto-created Schulhof category",
		slog.Int64("category_id", createdCategory.ID),
	)
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
		rs.getLogger().DebugContext(ctx, "found existing Schulhof activity",
			slog.Int64("activity_id", groups[0].ID),
		)
		return groups[0], nil
	}

	// Activity not found - auto-create the entire Schulhof infrastructure
	rs.getLogger().InfoContext(ctx, "Schulhof activity not found, auto-creating infrastructure")

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
	// Get staff ID from device authentication context - required for created_by FK
	staffCtx := device.StaffFromCtx(ctx)
	if staffCtx == nil {
		return nil, fmt.Errorf("schulhof activity auto-create requires staff context (scan staff RFID first)")
	}

	newActivity := &activities.Group{
		Name:            constants.SchulhofActivityName,
		MaxParticipants: constants.SchulhofMaxParticipants,
		IsOpen:          true, // Open activity - anyone can join
		CategoryID:      category.ID,
		PlannedRoomID:   &room.ID,
		CreatedBy:       staffCtx.ID,
	}

	// CreateGroup requires supervisorIDs and schedules - pass empty slices for auto-created activity
	createdActivity, err := rs.ActivitiesService.CreateGroup(ctx, newActivity, []int64{}, []*activities.Schedule{})
	if err != nil {
		return nil, fmt.Errorf("failed to auto-create Schulhof activity: %w", err)
	}

	rs.getLogger().InfoContext(ctx, "successfully auto-created Schulhof infrastructure",
		slog.Int64("room_id", room.ID),
		slog.Int64("category_id", category.ID),
		slog.Int64("activity_id", createdActivity.ID),
	)

	return createdActivity, nil
}
