package checkin

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
)

// systemSpace describes one lazily-bootstrapped system area (WC, Schulhof):
// a room, an activity category, and a permanent open activity group. The
// per-space room lookup keeps its own error policy inside findRoom, so the
// shared ensure/bootstrap flow below stays behavior-identical per space.
// All error strings and log lines are built from the label to match the
// previous per-space copies byte for byte.
type systemSpace struct {
	label string // "WC" / "Schulhof" — used in log + error text

	// findRoom returns (nil, nil) when the room does not exist yet.
	findRoom    func(ctx context.Context, rs *Resource) (*facilities.Room, error)
	logRoomName bool // WC logs room_name on found (alias set)

	roomName     string
	roomCapacity int

	categoryName string
	categoryDesc string
	color        string

	activityName    string
	maxParticipants int
	selectActivity  func(groups []*activities.Group, room *facilities.Room) *activities.Group
}

// ensureSystemRoom finds or creates the space's room.
func (rs *Resource) ensureSystemRoom(ctx context.Context, sp systemSpace) (*facilities.Room, error) {
	room, err := sp.findRoom(ctx, rs)
	if err != nil {
		return nil, err
	}
	if room != nil {
		attrs := []any{slog.Int64("room_id", room.ID)}
		if sp.logRoomName {
			attrs = append(attrs, slog.String("room_name", room.Name))
		}
		rs.getLogger().DebugContext(ctx, "found existing "+sp.label+" room", attrs...)
		return room, nil
	}

	// Room not found - create it
	rs.getLogger().InfoContext(ctx, sp.label+" room not found, auto-creating")

	capacity := sp.roomCapacity
	category := sp.categoryName
	color := sp.color

	newRoom := &facilities.Room{
		Name:     sp.roomName,
		Capacity: &capacity,
		Category: &category,
		Color:    &color,
		IsSystem: true,
	}

	if err := rs.FacilityService.CreateRoom(ctx, newRoom); err != nil {
		// Retry: concurrent request may have created it
		if room, retryErr := sp.findRoom(ctx, rs); retryErr == nil && room != nil {
			return room, nil
		}
		return nil, fmt.Errorf("failed to auto-create %s room: %w", sp.label, err)
	}

	rs.getLogger().InfoContext(ctx, "successfully auto-created "+sp.label+" room",
		slog.Int64("room_id", newRoom.ID),
	)
	return newRoom, nil
}

// ensureSystemCategory finds or creates the space's activity category.
func (rs *Resource) ensureSystemCategory(ctx context.Context, sp systemSpace) (*activities.Category, error) {
	categories, err := rs.ActivitiesService.ListCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list activity categories: %w", err)
	}

	for _, cat := range categories {
		if cat.Name == sp.categoryName {
			rs.getLogger().DebugContext(ctx, "found existing "+sp.label+" category",
				slog.Int64("category_id", cat.ID),
			)
			return cat, nil
		}
	}

	// Category not found - create it
	rs.getLogger().InfoContext(ctx, sp.label+" category not found, auto-creating")

	newCategory := &activities.Category{
		Name:        sp.categoryName,
		Description: sp.categoryDesc,
		Color:       sp.color,
		IsSystem:    true,
	}

	createdCategory, err := rs.ActivitiesService.CreateCategory(ctx, newCategory)
	if err != nil {
		// Retry: concurrent request may have created it
		retryCategories, retryErr := rs.ActivitiesService.ListCategories(ctx)
		if retryErr == nil {
			for _, cat := range retryCategories {
				if cat.Name == sp.categoryName {
					return cat, nil
				}
			}
		}
		return nil, fmt.Errorf("failed to auto-create %s category: %w", sp.label, err)
	}

	rs.getLogger().InfoContext(ctx, "successfully auto-created "+sp.label+" category",
		slog.Int64("category_id", createdCategory.ID),
	)
	return createdCategory, nil
}

// systemActivityGroup finds or creates the space's permanent activity group,
// lazily bootstrapping room + category + activity on first use.
func (rs *Resource) systemActivityGroup(ctx context.Context, sp systemSpace) (*activities.Group, error) {
	var room *facilities.Room
	var err error
	if sp.selectActivity != nil {
		// Selectors such as Schulhof need the canonical room to distinguish the
		// dedicated system activity from normal activities with the same name.
		room, err = rs.ensureSystemRoom(ctx, sp)
		if err != nil {
			return nil, fmt.Errorf("failed to ensure %s room: %w", sp.label, err)
		}
	}

	// WithTableAlias("group") is applied in the repository, so use the
	// unqualified field name.
	options := base.NewQueryOptions()
	filter := base.NewFilter()
	filter.Equal("name", sp.activityName)
	options.Filter = filter

	groups, err := rs.ActivitiesService.ListGroups(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to query %s activity: %w", sp.label, err)
	}

	selectExisting := func(candidates []*activities.Group) *activities.Group {
		if sp.selectActivity != nil {
			return sp.selectActivity(candidates, room)
		}
		if len(candidates) > 0 {
			return candidates[0]
		}
		return nil
	}

	if existingActivity := selectExisting(groups); existingActivity != nil {
		rs.getLogger().DebugContext(ctx, "found existing "+sp.label+" activity",
			slog.Int64("activity_id", existingActivity.ID),
		)
		return existingActivity, nil
	}

	// Activity not found - auto-create the entire infrastructure
	rs.getLogger().InfoContext(ctx, sp.label+" activity not found, auto-creating infrastructure")

	if room == nil {
		room, err = rs.ensureSystemRoom(ctx, sp)
		if err != nil {
			return nil, fmt.Errorf("failed to ensure %s room: %w", sp.label, err)
		}
	}

	category, err := rs.ensureSystemCategory(ctx, sp)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure %s category: %w", sp.label, err)
	}

	// created_by is NULL = system-created
	newActivity := &activities.Group{
		Name:            sp.activityName,
		MaxParticipants: sp.maxParticipants,
		IsOpen:          true, // Open activity - anyone can join
		CategoryID:      category.ID,
		PlannedRoomID:   &room.ID,
		IsSystem:        true,
	}

	// CreateGroup requires supervisorIDs and schedules - pass empty slices for auto-created activity
	createdActivity, err := rs.ActivitiesService.CreateGroup(ctx, newActivity, []int64{}, []*activities.Schedule{})
	if err != nil {
		// Retry: concurrent request may have created it
		retryOptions := base.NewQueryOptions()
		retryFilter := base.NewFilter()
		retryFilter.Equal("name", sp.activityName)
		retryOptions.Filter = retryFilter
		retryGroups, retryErr := rs.ActivitiesService.ListGroups(ctx, retryOptions)
		if retryErr == nil {
			if existingActivity := selectExisting(retryGroups); existingActivity != nil {
				return existingActivity, nil
			}
		}
		return nil, fmt.Errorf("failed to auto-create %s activity: %w", sp.label, err)
	}

	rs.getLogger().InfoContext(ctx, "successfully auto-created "+sp.label+" infrastructure",
		slog.Int64("room_id", room.ID),
		slog.Int64("category_id", category.ID),
		slog.Int64("activity_id", createdActivity.ID),
	)

	return createdActivity, nil
}
