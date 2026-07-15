// Package facilities provides WC (toilet/bathroom) service for managing the
// permanent WC room with auto-creation capabilities.
package facilities

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/constants"
	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/services/activities"
)

// WCService provides operations for managing the WC (toilet) system room.
// The WC room is a special permanent room that:
// - Is auto-created when the checkout.wc_enabled setting is toggled on
// - Cannot be deleted or renamed (protected as a system room)
// - Auto-creates necessary infrastructure (room, category, activity) on first use
type WCService interface {
	// EnsureInfrastructure ensures the WC room, category, and activity group exist.
	// Creates them if they don't exist (idempotent). CreatedBy is nil (system-created).
	// Returns the activity group.
	EnsureInfrastructure(ctx context.Context) (*activityModels.Group, error)
}

// wcService implements WCService.
type wcService struct {
	facilityService Service
	activityService activities.ActivityService
	logger          *slog.Logger
}

var errWCActivityNotFound = errors.New("WC activity not found")

// getLogger returns a nil-safe logger, falling back to slog.Default() if logger is nil.
func (s *wcService) getLogger() *slog.Logger {
	return cmp.Or(s.logger, slog.Default())
}

// NewWCService creates a new WC service.
func NewWCService(
	facilityService Service,
	activityService activities.ActivityService,
	logger *slog.Logger,
) WCService {
	return &wcService{
		facilityService: facilityService,
		activityService: activityService,
		logger:          logger,
	}
}

// EnsureInfrastructure ensures the WC room, category, and activity group exist.
func (s *wcService) EnsureInfrastructure(ctx context.Context) (*activityModels.Group, error) {
	// Check if activity already exists
	activityGroup, err := s.findWCActivity(ctx)
	if err == nil && activityGroup != nil {
		return activityGroup, nil
	}
	if err != nil && !errors.Is(err, errWCActivityNotFound) {
		return nil, fmt.Errorf("failed to look up WC activity: %w", err)
	}

	s.getLogger().Info("WC infrastructure not found, auto-creating...",
		slog.String("component", "wc"))

	// Step 1: Ensure WC room exists
	room, err := s.ensureWCRoom(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure WC room: %w", err)
	}

	// Step 2: Ensure WC category exists
	category, err := s.ensureWCCategory(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure WC category: %w", err)
	}

	// Step 3: Create the WC activity group (CreatedBy = nil = system-created)
	newActivity := &activityModels.Group{
		Name:            constants.WCActivityName,
		MaxParticipants: constants.WCMaxParticipants,
		IsOpen:          true, // Open activity - anyone can join
		CategoryID:      category.ID,
		PlannedRoomID:   &room.ID,
		IsSystem:        true,
	}

	createdActivity, err := s.activityService.CreateGroup(ctx, newActivity, []int64{}, []*activityModels.Schedule{})
	if err != nil {
		// Retry: concurrent request may have created it
		retryGroup, retryErr := s.findWCActivity(ctx)
		if retryErr == nil && retryGroup != nil {
			return retryGroup, nil
		}
		return nil, fmt.Errorf("failed to create WC activity: %w", err)
	}

	s.getLogger().Info("successfully created WC infrastructure",
		slog.String("component", "wc"),
		slog.Int64("room_id", room.ID),
		slog.Int64("category_id", category.ID),
		slog.Int64("activity_id", createdActivity.ID))

	return createdActivity, nil
}

// findWCActivity finds the WC activity group by name.
func (s *wcService) findWCActivity(ctx context.Context) (*activityModels.Group, error) {
	options := base.NewQueryOptions()
	filter := base.NewFilter()
	filter.Equal("name", constants.WCActivityName)
	options.Filter = filter

	groups, err := s.activityService.ListGroups(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to query WC activity: %w", err)
	}

	if len(groups) == 0 {
		return nil, errWCActivityNotFound
	}

	return groups[0], nil
}

// ensureWCRoom finds or creates the WC room.
func (s *wcService) ensureWCRoom(ctx context.Context) (*facilities.Room, error) {
	room, err := s.facilityService.FindToiletRoom(ctx, 0)
	if err == nil && room != nil {
		s.getLogger().Info("found existing WC room",
			slog.String("component", "wc"),
			slog.Int64("room_id", room.ID),
			slog.String("room_name", room.Name))
		return room, nil
	}
	if err != nil && !errors.Is(err, ErrRoomNotFound) {
		return nil, fmt.Errorf("failed to look up WC room: %w", err)
	}

	// Room not found - create it
	s.getLogger().Info("WC room not found, auto-creating...",
		slog.String("component", "wc"))

	capacity := constants.WCRoomCapacity
	category := constants.WCCategoryName
	color := constants.WCColor

	newRoom := &facilities.Room{
		Name:     constants.WCRoomName,
		Capacity: &capacity,
		Category: &category,
		Color:    &color,
		IsSystem: true,
	}

	if err := s.facilityService.CreateRoom(ctx, newRoom); err != nil {
		// Retry: concurrent request may have created one of the accepted aliases
		if room, retryErr := s.facilityService.FindToiletRoom(ctx, 0); retryErr == nil && room != nil {
			return room, nil
		}
		return nil, fmt.Errorf("failed to create WC room: %w", err)
	}

	s.getLogger().Info("successfully created WC room",
		slog.String("component", "wc"),
		slog.Int64("room_id", newRoom.ID))
	return newRoom, nil
}

// ensureWCCategory finds or creates the WC activity category.
func (s *wcService) ensureWCCategory(ctx context.Context) (*activityModels.Category, error) {
	// Try to find existing WC category
	categories, err := s.activityService.ListCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list activity categories: %w", err)
	}

	for _, cat := range categories {
		if cat.Name == constants.WCCategoryName {
			s.getLogger().Info("found existing WC category",
				slog.String("component", "wc"),
				slog.Int64("category_id", cat.ID))
			return cat, nil
		}
	}

	// Category not found - create it
	s.getLogger().Info("WC category not found, auto-creating...",
		slog.String("component", "wc"))

	newCategory := &activityModels.Category{
		Name:        constants.WCCategoryName,
		Description: constants.WCCategoryDescription,
		Color:       constants.WCColor,
		IsSystem:    true,
	}

	createdCategory, err := s.activityService.CreateCategory(ctx, newCategory)
	if err != nil {
		// Retry: concurrent request may have created it
		retryCategories, retryErr := s.activityService.ListCategories(ctx)
		if retryErr == nil {
			for _, cat := range retryCategories {
				if cat.Name == constants.WCCategoryName {
					return cat, nil
				}
			}
		}
		return nil, fmt.Errorf("failed to create WC category: %w", err)
	}

	s.getLogger().Info("successfully created WC category",
		slog.String("component", "wc"),
		slog.Int64("category_id", createdCategory.ID))
	return createdCategory, nil
}
