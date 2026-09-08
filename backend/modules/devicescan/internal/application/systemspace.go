package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
)

// systemSpace describes one lazily provisioned system area (WC, Schulhof):
// a room, an activity category, and a permanent open activity. The
// per-space room lookup keeps its own error policy in findRoom, so the
// shared bootstrap below stays behavior-identical per space. Error strings
// and log lines are built from the label to match the historic copies.
type systemSpace struct {
	label string

	// findRoom returns nil, nil when the room does not exist yet.
	findRoom    func(ctx context.Context, s *Service) (*facilities.Room, error)
	logRoomName bool

	roomName     string
	roomCapacity int

	categoryName string
	categoryDesc string
	color        string

	// roomColorless leaves the provisioned room's color unset. Set for the
	// Schulhof (#2405): its color is admin-configurable, and an unset color
	// is what makes the documented orange default apply.
	roomColorless bool

	// releaseRoomOnCreate marks the provisioned room as a permanently
	// released open room. Set for the Schulhof only (#3064); the toilets are
	// short-stay infrastructure, not a destination a child chooses. Applied
	// on creation alone, so an administrator's deactivation survives.
	releaseRoomOnCreate bool

	activityName    string
	maxParticipants int
	selectActivity  func(candidates []ports.Activity, room *facilities.Room) *ports.Activity
}

var schulhofSpace = systemSpace{
	label: "Schulhof",
	findRoom: func(ctx context.Context, s *Service) (*facilities.Room, error) {
		room, err := s.findCanonicalSchulhofRoom(ctx)
		if errors.Is(err, facilities.ErrRoomNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("failed to look up Schulhof room: %w", err)
		}
		return room, nil
	},
	roomName:            facilities.SchulhofRoomName,
	roomCapacity:        facilities.SchulhofRoomCapacity,
	categoryName:        facilities.SchulhofCategoryName,
	categoryDesc:        facilities.SchulhofCategoryDescription,
	color:               facilities.SchulhofColor,
	roomColorless:       true,
	releaseRoomOnCreate: true,
	activityName:        facilities.SchulhofActivityName,
	maxParticipants:     facilities.SchulhofMaxParticipants,
	selectActivity: func(candidates []ports.Activity, room *facilities.Room) *ports.Activity {
		for i := range candidates {
			if validateSchulhofActivityRoom(&candidates[i], room) == nil {
				return &candidates[i]
			}
		}
		return nil
	},
}

// wcSpace looks the room up through the toilet alias lookup and, unlike
// the Schulhof, aborts on unexpected lookup errors instead of falling through
// to provisioning, so alias collisions surface instead of spawning duplicates.
var wcSpace = systemSpace{
	label:       "WC",
	logRoomName: true,
	findRoom: func(ctx context.Context, s *Service) (*facilities.Room, error) {
		room, err := s.rooms.FindToiletRoom(ctx, 0)
		if err != nil {
			if errors.Is(err, facilities.ErrRoomNotFound) {
				return nil, nil
			}
			return nil, fmt.Errorf("failed to look up WC room: %w", err)
		}
		return &room, nil
	},
	roomName:        facilities.WCRoomName,
	roomCapacity:    facilities.WCRoomCapacity,
	categoryName:    facilities.WCCategoryName,
	categoryDesc:    facilities.WCCategoryDescription,
	color:           facilities.WCColor,
	activityName:    facilities.WCActivityName,
	maxParticipants: facilities.WCMaxParticipants,
}

// findCanonicalSchulhofRoom returns the tenant's Schulhof only when it is
// the reserved system room: a room that merely matches the name
// case-insensitively or lacks the system flag is a configuration conflict.
func (s *Service) findCanonicalSchulhofRoom(ctx context.Context) (*facilities.Room, error) {
	room, err := s.rooms.FindRoomByName(ctx, facilities.SchulhofRoomName)
	if err != nil {
		return nil, err
	}
	if room.Name != facilities.SchulhofRoomName {
		return nil, fmt.Errorf("non-canonical room name %q conflicts with reserved name %q", room.Name, facilities.SchulhofRoomName)
	}
	if !room.IsSystem {
		return nil, fmt.Errorf("reserved room %q is not marked as a system room", facilities.SchulhofRoomName)
	}
	return &room, nil
}

// validateSchulhofActivityRoom checks that the activity is the dedicated
// system activity of the canonical yard.
func validateSchulhofActivityRoom(activity *ports.Activity, room *facilities.Room) error {
	if activity == nil {
		return errors.New("schulhof activity is nil")
	}
	if room == nil {
		return errors.New("schulhof room is nil")
	}
	if activity.Name != facilities.SchulhofActivityName {
		return fmt.Errorf("activity name %q does not match reserved name %q", activity.Name, facilities.SchulhofActivityName)
	}
	if !activity.IsSystem {
		return fmt.Errorf("reserved activity %q is not marked as a system activity", facilities.SchulhofActivityName)
	}
	if activity.PlannedRoomID == nil || *activity.PlannedRoomID != room.ID {
		return fmt.Errorf("reserved activity %q is not assigned to canonical Schulhof room %d", facilities.SchulhofActivityName, room.ID)
	}
	return nil
}

// schulhofActivity finds or provisions the permanent Schulhof activity,
// validating the yard infrastructure it is bound to.
func (s *Service) schulhofActivity(ctx context.Context) (*ports.Activity, error) {
	activity, err := s.systemActivity(ctx, schulhofSpace)
	if err != nil {
		return nil, err
	}
	room, err := s.findCanonicalSchulhofRoom(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to validate Schulhof room: %w", err)
	}
	if err := validateSchulhofActivityRoom(activity, room); err != nil {
		return nil, fmt.Errorf("invalid Schulhof activity infrastructure: %w", err)
	}
	return activity, nil
}

// wcActivity finds or provisions the permanent WC activity.
func (s *Service) wcActivity(ctx context.Context) (*ports.Activity, error) {
	return s.systemActivity(ctx, wcSpace)
}

// ensureSystemRoom finds or creates the space's room.
func (s *Service) ensureSystemRoom(ctx context.Context, sp systemSpace) (*facilities.Room, error) {
	room, err := sp.findRoom(ctx, s)
	if err != nil {
		return nil, err
	}
	if room != nil {
		attrs := []any{slog.Int64("room_id", room.ID)}
		if sp.logRoomName {
			attrs = append(attrs, slog.String("room_name", room.Name))
		}
		s.logger.DebugContext(ctx, "found existing "+sp.label+" room", attrs...)
		return room, nil
	}

	s.logger.InfoContext(ctx, sp.label+" room not found, auto-creating")
	capacity := sp.roomCapacity
	category := sp.categoryName
	input := facilities.CreateRoom{
		Name: sp.roomName, Capacity: &capacity, Category: &category, IsSystem: true, IsOpenRoom: sp.releaseRoomOnCreate,
	}
	if !sp.roomColorless {
		color := sp.color
		input.Color = &color
	}
	created, err := s.rooms.CreateRoom(ctx, input)
	if err != nil {
		// Retry: a concurrent request may have created it.
		if room, retryErr := sp.findRoom(ctx, s); retryErr == nil && room != nil {
			return room, nil
		}
		return nil, fmt.Errorf("failed to auto-create %s room: %w", sp.label, err)
	}
	s.logger.InfoContext(ctx, "successfully auto-created "+sp.label+" room",
		slog.Int64("room_id", created.ID),
	)
	return &created, nil
}

// ensureSystemCategory finds or creates the space's activity category.
func (s *Service) ensureSystemCategory(ctx context.Context, sp systemSpace) (*ports.Category, error) {
	categories, err := s.activities.ListCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list activity categories: %w", err)
	}
	for i := range categories {
		if categories[i].Name == sp.categoryName {
			s.logger.DebugContext(ctx, "found existing "+sp.label+" category",
				slog.Int64("category_id", categories[i].ID),
			)
			return &categories[i], nil
		}
	}

	s.logger.InfoContext(ctx, sp.label+" category not found, auto-creating")
	created, err := s.activities.CreateCategory(ctx, ports.NewCategory{
		Name: sp.categoryName, Description: sp.categoryDesc, Color: sp.color, IsSystem: true,
	})
	if err != nil {
		// Retry: a concurrent request may have created it.
		if retry, retryErr := s.activities.ListCategories(ctx); retryErr == nil {
			for i := range retry {
				if retry[i].Name == sp.categoryName {
					return &retry[i], nil
				}
			}
		}
		return nil, fmt.Errorf("failed to auto-create %s category: %w", sp.label, err)
	}
	s.logger.InfoContext(ctx, "successfully auto-created "+sp.label+" category",
		slog.Int64("category_id", created.ID),
	)
	return &created, nil
}

// systemActivity finds or creates the space's permanent activity, lazily
// provisioning room, category and activity on first use.
func (s *Service) systemActivity(ctx context.Context, sp systemSpace) (*ports.Activity, error) {
	if s.activities == nil {
		return nil, errors.New("activity catalog is not configured")
	}
	var room *facilities.Room
	var err error
	if sp.selectActivity != nil {
		// Selectors such as Schulhof need the canonical room to distinguish
		// the dedicated system activity from normal activities of the same name.
		room, err = s.ensureSystemRoom(ctx, sp)
		if err != nil {
			return nil, fmt.Errorf("failed to ensure %s room: %w", sp.label, err)
		}
	}

	candidates, err := s.activities.ListByName(ctx, sp.activityName)
	if err != nil {
		return nil, fmt.Errorf("failed to query %s activity: %w", sp.label, err)
	}
	selectExisting := func(candidates []ports.Activity) *ports.Activity {
		if sp.selectActivity != nil {
			return sp.selectActivity(candidates, room)
		}
		if len(candidates) > 0 {
			return &candidates[0]
		}
		return nil
	}
	if existing := selectExisting(candidates); existing != nil {
		s.logger.DebugContext(ctx, "found existing "+sp.label+" activity",
			slog.Int64("activity_id", existing.ID),
		)
		return existing, nil
	}

	s.logger.InfoContext(ctx, sp.label+" activity not found, auto-creating infrastructure")
	if room == nil {
		room, err = s.ensureSystemRoom(ctx, sp)
		if err != nil {
			return nil, fmt.Errorf("failed to ensure %s room: %w", sp.label, err)
		}
	}
	category, err := s.ensureSystemCategory(ctx, sp)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure %s category: %w", sp.label, err)
	}

	// created_by stays unset: the activity is system-created.
	created, err := s.activities.CreateActivity(ctx, ports.NewActivity{
		Name: sp.activityName, MaxParticipants: sp.maxParticipants, IsOpen: true,
		CategoryID: category.ID, PlannedRoomID: &room.ID, IsSystem: true,
	})
	if err != nil {
		// Retry: a concurrent request may have created it.
		if retry, retryErr := s.activities.ListByName(ctx, sp.activityName); retryErr == nil {
			if existing := selectExisting(retry); existing != nil {
				return existing, nil
			}
		}
		return nil, fmt.Errorf("failed to auto-create %s activity: %w", sp.label, err)
	}
	s.logger.InfoContext(ctx, "successfully auto-created "+sp.label+" infrastructure",
		slog.Int64("room_id", room.ID),
		slog.Int64("category_id", category.ID),
		slog.Int64("activity_id", created.ID),
	)
	return &created, nil
}
