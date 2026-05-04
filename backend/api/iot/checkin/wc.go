package checkin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	facilitiesSvc "github.com/moto-nrw/project-phoenix/services/facilities"
)

// ensureWCRoom finds or creates the WC room
func (rs *Resource) ensureWCRoom(ctx context.Context) (*facilities.Room, error) {
	room, err := rs.FacilityService.FindToiletRoom(ctx, 0)
	if err == nil && room != nil {
		rs.getLogger().DebugContext(ctx, "found existing WC room",
			slog.Int64("room_id", room.ID),
			slog.String("room_name", room.Name),
		)
		return room, nil
	}
	if err != nil && !errors.Is(err, facilitiesSvc.ErrRoomNotFound) {
		return nil, fmt.Errorf("failed to look up WC room: %w", err)
	}

	// Room not found - create it
	rs.getLogger().InfoContext(ctx, "WC room not found, auto-creating")

	capacity := constants.WCRoomCapacity
	category := constants.WCCategoryName
	color := constants.WCColor

	newRoom := &facilities.Room{
		Name:     constants.WCRoomName,
		Capacity: &capacity,
		Category: &category,
		Color:    &color,
	}

	if err := rs.FacilityService.CreateRoom(ctx, newRoom); err != nil {
		// Retry: concurrent request may have created one of the accepted aliases
		if room, retryErr := rs.FacilityService.FindToiletRoom(ctx, 0); retryErr == nil && room != nil {
			return room, nil
		}
		return nil, fmt.Errorf("failed to auto-create WC room: %w", err)
	}

	rs.getLogger().InfoContext(ctx, "successfully auto-created WC room",
		slog.Int64("room_id", newRoom.ID),
	)
	return newRoom, nil
}

// ensureWCCategory finds or creates the WC activity category
func (rs *Resource) ensureWCCategory(ctx context.Context) (*activities.Category, error) {
	// Try to find existing WC category
	categories, err := rs.ActivitiesService.ListCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list activity categories: %w", err)
	}

	for _, cat := range categories {
		if cat.Name == constants.WCCategoryName {
			rs.getLogger().DebugContext(ctx, "found existing WC category",
				slog.Int64("category_id", cat.ID),
			)
			return cat, nil
		}
	}

	// Category not found - create it
	rs.getLogger().InfoContext(ctx, "WC category not found, auto-creating")

	newCategory := &activities.Category{
		Name:        constants.WCCategoryName,
		Description: constants.WCCategoryDescription,
		Color:       constants.WCColor,
	}

	createdCategory, err := rs.ActivitiesService.CreateCategory(ctx, newCategory)
	if err != nil {
		// Retry: concurrent request may have created it
		retryCategories, retryErr := rs.ActivitiesService.ListCategories(ctx)
		if retryErr == nil {
			for _, cat := range retryCategories {
				if cat.Name == constants.WCCategoryName {
					return cat, nil
				}
			}
		}
		return nil, fmt.Errorf("failed to auto-create WC category: %w", err)
	}

	rs.getLogger().InfoContext(ctx, "successfully auto-created WC category",
		slog.Int64("category_id", createdCategory.ID),
	)
	return createdCategory, nil
}

// wcActivityGroup finds or creates the permanent WC activity group.
// This function implements lazy initialization - it will auto-create the WC
// infrastructure (room, category, activity) on first use if not found.
func (rs *Resource) wcActivityGroup(ctx context.Context) (*activities.Group, error) {
	// Build filter for WC activity using constant
	options := base.NewQueryOptions()
	filter := base.NewFilter()
	filter.Equal("name", constants.WCActivityName)
	options.Filter = filter

	// Query activities service
	groups, err := rs.ActivitiesService.ListGroups(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to query WC activity: %w", err)
	}

	// If activity exists, return it
	if len(groups) > 0 {
		rs.getLogger().DebugContext(ctx, "found existing WC activity",
			slog.Int64("activity_id", groups[0].ID),
		)
		return groups[0], nil
	}

	// Activity not found - auto-create the entire WC infrastructure
	rs.getLogger().InfoContext(ctx, "WC activity not found, auto-creating infrastructure")

	// Step 1: Ensure WC room exists
	room, err := rs.ensureWCRoom(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure WC room: %w", err)
	}

	// Step 2: Ensure WC category exists
	category, err := rs.ensureWCCategory(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure WC category: %w", err)
	}

	// Step 3: Create the WC activity group (created_by is NULL = system-created)
	newActivity := &activities.Group{
		Name:            constants.WCActivityName,
		MaxParticipants: constants.WCMaxParticipants,
		IsOpen:          true, // Open activity - anyone can join
		CategoryID:      category.ID,
		PlannedRoomID:   &room.ID,
	}

	// CreateGroup requires supervisorIDs and schedules - pass empty slices for auto-created activity
	createdActivity, err := rs.ActivitiesService.CreateGroup(ctx, newActivity, []int64{}, []*activities.Schedule{})
	if err != nil {
		// Retry: concurrent request may have created it
		retryOptions := base.NewQueryOptions()
		retryFilter := base.NewFilter()
		retryFilter.Equal("name", constants.WCActivityName)
		retryOptions.Filter = retryFilter
		retryGroups, retryErr := rs.ActivitiesService.ListGroups(ctx, retryOptions)
		if retryErr == nil && len(retryGroups) > 0 {
			return retryGroups[0], nil
		}
		return nil, fmt.Errorf("failed to auto-create WC activity: %w", err)
	}

	rs.getLogger().InfoContext(ctx, "successfully auto-created WC infrastructure",
		slog.Int64("room_id", room.ID),
		slog.Int64("category_id", category.ID),
		slog.Int64("activity_id", createdActivity.ID),
	)

	return createdActivity, nil
}
