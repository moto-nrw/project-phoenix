// Package facilities provides Schulhof (schoolyard) service for managing the permanent
// outdoor area with supervisor toggling capabilities.
package facilities

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/models/active"
	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/activities"
)

// SchulhofService provides operations for managing the Schulhof (schoolyard) area.
// The Schulhof is a special permanent outdoor supervision area that:
// - Always appears in the "My Supervisions" tabs
// - Can be claimed/released by supervisors at any time
// - Auto-creates necessary infrastructure (room, category, activity) on first use
type SchulhofService interface {
	// GetSchulhofStatus returns the current status of the Schulhof area including
	// room info, active group, supervisors, and student count.
	GetSchulhofStatus(ctx context.Context, staffID int64) (*SchulhofStatus, error)

	// ToggleSupervision starts or stops supervision for the given staff member.
	// action must be "start" or "stop".
	ToggleSupervision(ctx context.Context, staffID int64, action string) (*SupervisionResult, error)

	// EnsureInfrastructure ensures the Schulhof room, category, and activity group exist.
	// createdBy is the staff ID to use when creating new infrastructure.
	// Returns the activity group.
	EnsureInfrastructure(ctx context.Context, createdBy int64) (*activityModels.Group, error)

	// GetOrCreateActiveGroup returns the active Schulhof group for today,
	// creating one if it doesn't exist.
	// createdBy is the staff ID to use when creating new infrastructure.
	GetOrCreateActiveGroup(ctx context.Context, createdBy int64) (*active.Group, error)
}

// SchulhofStatus represents the current state of the Schulhof area.
type SchulhofStatus struct {
	Exists            bool             `json:"exists"`
	RoomID            *int64           `json:"room_id,omitempty"`
	RoomName          string           `json:"room_name"`
	ActivityGroupID   *int64           `json:"activity_group_id,omitempty"`
	ActiveGroupID     *int64           `json:"active_group_id,omitempty"`
	IsUserSupervising bool             `json:"is_user_supervising"`
	SupervisionID     *int64           `json:"supervision_id,omitempty"`
	SupervisorCount   int              `json:"supervisor_count"`
	StudentCount      int              `json:"student_count"`
	Supervisors       []SupervisorInfo `json:"supervisors"`
}

// SupervisorInfo contains information about a supervisor.
type SupervisorInfo struct {
	ID            int64  `json:"id"`
	StaffID       int64  `json:"staff_id"`
	Name          string `json:"name"`
	IsCurrentUser bool   `json:"is_current_user"`
}

// SupervisionResult represents the result of a supervision toggle operation.
type SupervisionResult struct {
	Action        string `json:"action"` // "started" or "stopped"
	SupervisionID *int64 `json:"supervision_id,omitempty"`
	ActiveGroupID int64  `json:"active_group_id"`
}

// schulhofService implements SchulhofService.
type schulhofService struct {
	facilityService Service
	activityService activities.ActivityService
	activeService   activeSvc.Service
	logger          *slog.Logger
}

var errSchulhofActivityNotFound = errors.New("schulhof activity not found")

// getLogger returns a nil-safe logger, falling back to slog.Default() if logger is nil
func (s *schulhofService) getLogger() *slog.Logger {
	return cmp.Or(s.logger, slog.Default())
}

// NewSchulhofService creates a new Schulhof service.
func NewSchulhofService(
	facilityService Service,
	activityService activities.ActivityService,
	activeService activeSvc.Service,
	logger *slog.Logger,
) SchulhofService {
	return &schulhofService{
		facilityService: facilityService,
		activityService: activityService,
		activeService:   activeService,
		logger:          logger,
	}
}

// GetSchulhofStatus returns the current status of the Schulhof area.
func (s *schulhofService) GetSchulhofStatus(ctx context.Context, staffID int64) (*SchulhofStatus, error) {
	status := &SchulhofStatus{
		Exists:      false,
		RoomName:    constants.SchulhofRoomName,
		Supervisors: []SupervisorInfo{},
	}

	// Step 1: Find Schulhof room
	room, err := FindCanonicalSchulhofRoom(ctx, s.facilityService)
	if err != nil {
		if errors.Is(err, ErrRoomNotFound) {
			// Room doesn't exist yet - return status with exists=false.
			s.getLogger().Info("schulhof room not found, infrastructure not yet created",
				slog.String("component", "schulhof"))
			return status, nil
		}
		return nil, fmt.Errorf("failed to look up Schulhof room: %w", err)
	}
	status.Exists = true
	status.RoomID = &room.ID

	// Step 2: Find Schulhof activity group
	activityGroup, err := s.findSchulhofActivity(ctx, room)
	if err != nil {
		if errors.Is(err, errSchulhofActivityNotFound) {
			s.getLogger().Info("schulhof activity not found",
				slog.String("component", "schulhof"))
			return status, nil
		}
		return nil, fmt.Errorf("failed to look up Schulhof activity: %w", err)
	}
	status.ActivityGroupID = &activityGroup.ID
	if err := ValidateSchulhofActivityRoom(activityGroup, room); err != nil {
		return nil, fmt.Errorf("invalid Schulhof activity infrastructure: %w", err)
	}

	// Step 3: Find today's active group for this room
	activeGroup, err := s.findTodayActiveGroup(ctx, room.ID, activityGroup.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to look up today's Schulhof group: %w", err)
	}
	if activeGroup == nil {
		// No active session today - still return status with exists=true
		return status, nil
	}
	status.ActiveGroupID = &activeGroup.ID

	// Step 4: Get supervisors for this active group
	supervisors, err := s.activeService.FindSupervisorsByActiveGroupID(ctx, activeGroup.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to look up Schulhof supervisors: %w", err)
	}
	status.SupervisorCount = len(supervisors)
	for _, sup := range supervisors {
		if sup.EndDate != nil {
			continue // Skip ended supervisions
		}
		info := SupervisorInfo{
			ID:            sup.ID,
			StaffID:       sup.StaffID,
			IsCurrentUser: sup.StaffID == staffID,
		}
		// Get supervisor name
		if sup.Staff != nil && sup.Staff.Person != nil {
			info.Name = sup.Staff.Person.FirstName + " " + sup.Staff.Person.LastName
		}
		status.Supervisors = append(status.Supervisors, info)
		if info.IsCurrentUser {
			status.IsUserSupervising = true
			status.SupervisionID = &sup.ID
		}
	}
	status.SupervisorCount = len(status.Supervisors)

	// Step 5: Count students in this active group
	visits, err := s.activeService.FindVisitsByActiveGroupID(ctx, activeGroup.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to look up Schulhof visits: %w", err)
	}
	for _, visit := range visits {
		if visit.ExitTime == nil {
			status.StudentCount++
		}
	}

	return status, nil
}

// ToggleSupervision starts or stops supervision for the given staff member.
func (s *schulhofService) ToggleSupervision(ctx context.Context, staffID int64, action string) (*SupervisionResult, error) {
	if action != "start" && action != "stop" {
		return nil, fmt.Errorf("invalid action: %s (must be 'start' or 'stop')", action)
	}

	// Ensure infrastructure exists (use staffID as creator if infrastructure needs to be created)
	_, err := s.EnsureInfrastructure(ctx, staffID)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure Schulhof infrastructure: %w", err)
	}

	// Get or create active group for today
	activeGroup, err := s.GetOrCreateActiveGroup(ctx, staffID)
	if err != nil {
		return nil, fmt.Errorf("failed to get/create active group: %w", err)
	}

	result := &SupervisionResult{
		ActiveGroupID: activeGroup.ID,
	}

	if action == "start" {
		// Claim the group as supervisor
		supervision, err := s.activeService.ClaimActiveGroup(ctx, activeGroup.ID, staffID, "supervisor")
		if err != nil {
			return nil, fmt.Errorf("failed to claim Schulhof supervision: %w", err)
		}
		result.Action = "started"
		result.SupervisionID = &supervision.ID
	} else {
		// Find and end the user's supervision
		supervisors, err := s.activeService.FindSupervisorsByActiveGroupID(ctx, activeGroup.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to find supervisors: %w", err)
		}

		var supervisionID int64
		found := false
		for _, sup := range supervisors {
			if sup.StaffID == staffID && sup.EndDate == nil {
				supervisionID = sup.ID
				found = true
				break
			}
		}

		if !found {
			return nil, fmt.Errorf("user is not currently supervising the Schulhof")
		}

		if err := s.activeService.EndSupervision(ctx, supervisionID); err != nil {
			return nil, fmt.Errorf("failed to end Schulhof supervision: %w", err)
		}
		result.Action = "stopped"
	}

	return result, nil
}

// EnsureInfrastructure ensures the Schulhof room, category, and activity group exist.
func (s *schulhofService) EnsureInfrastructure(ctx context.Context, createdBy int64) (*activityModels.Group, error) {
	// Validate or create the canonical room before returning any pre-existing
	// activity. Older versions could provision an activity against a lowercase
	// room that FindRoomByName would otherwise adopt case-insensitively.
	room, err := s.ensureSchulhofRoom(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure Schulhof room: %w", err)
	}

	// Check if activity already exists
	activityGroup, err := s.findSchulhofActivity(ctx, room)
	if err == nil && activityGroup != nil {
		if validateErr := ValidateSchulhofActivityRoom(activityGroup, room); validateErr != nil {
			return nil, fmt.Errorf("invalid Schulhof activity infrastructure: %w", validateErr)
		}
		return activityGroup, nil
	}
	if err != nil && !errors.Is(err, errSchulhofActivityNotFound) {
		return nil, fmt.Errorf("failed to look up Schulhof activity: %w", err)
	}

	s.getLogger().Info("schulhof infrastructure not found, auto-creating...",
		slog.String("component", "schulhof"))

	// Step 1: Ensure Schulhof category exists
	category, err := s.ensureSchulhofCategory(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure Schulhof category: %w", err)
	}

	// Step 2: Create the Schulhof activity group
	newActivity := &activityModels.Group{
		Name:            constants.SchulhofActivityName,
		MaxParticipants: constants.SchulhofMaxParticipants,
		IsOpen:          true, // Open activity - anyone can join
		CategoryID:      category.ID,
		PlannedRoomID:   &room.ID,
		IsSystem:        true,
	}
	// createdBy == 0 means system-created (nil) — avoids FK violation on users.staff
	if createdBy > 0 {
		newActivity.CreatedBy = &createdBy
	}

	createdActivity, err := s.activityService.CreateGroup(ctx, newActivity, []int64{}, []*activityModels.Schedule{})
	if err != nil {
		return nil, fmt.Errorf("failed to create Schulhof activity: %w", err)
	}

	s.getLogger().Info("successfully created schulhof infrastructure",
		slog.String("component", "schulhof"),
		slog.Int64("room_id", room.ID),
		slog.Int64("category_id", category.ID),
		slog.Int64("activity_id", createdActivity.ID))

	return createdActivity, nil
}

// GetOrCreateActiveGroup returns the active Schulhof group for today.
func (s *schulhofService) GetOrCreateActiveGroup(ctx context.Context, createdBy int64) (*active.Group, error) {
	// Ensure infrastructure exists
	activityGroup, err := s.EnsureInfrastructure(ctx, createdBy)
	if err != nil {
		return nil, err
	}

	// Get the room
	room, err := FindCanonicalSchulhofRoom(ctx, s.facilityService)
	if err != nil {
		return nil, fmt.Errorf("failed to find Schulhof room: %w", err)
	}
	if err := ValidateSchulhofActivityRoom(activityGroup, room); err != nil {
		return nil, fmt.Errorf("invalid Schulhof activity infrastructure: %w", err)
	}

	// Find today's active group
	activeGroup, err := s.findTodayActiveGroup(ctx, room.ID, activityGroup.ID)
	if err == nil && activeGroup != nil {
		return activeGroup, nil
	}

	// End any stale (non-today) active groups for this room before creating a new one.
	// Without this, CheckRoomConflict in CreateActiveGroup would reject the new group
	// because a leftover group from a previous day still occupies the room.
	if err := s.endStaleActiveGroups(ctx, room.ID); err != nil {
		return nil, fmt.Errorf("failed to end stale active groups: %w", err)
	}

	// Create a new active group for today. Schulhof sessions are always
	// template-backed (the well-known Schulhof activity), so GroupID is set.
	now := time.Now()
	activityGroupID := activityGroup.ID
	newActiveGroup := &active.Group{
		GroupID:   &activityGroupID,
		RoomID:    room.ID,
		StartTime: now,
	}

	if err := s.activeService.CreateActiveGroup(ctx, newActiveGroup); err != nil {
		if errors.Is(err, activeSvc.ErrRoomConflict) {
			existingGroup, findErr := s.findTodayActiveGroup(ctx, room.ID, activityGroup.ID)
			if findErr != nil {
				return nil, fmt.Errorf("failed to refetch Schulhof active group after room conflict: %w", findErr)
			}
			if existingGroup != nil {
				s.getLogger().Info("reused concurrently created schulhof active group",
					slog.String("component", "schulhof"),
					slog.Int64("active_group_id", existingGroup.ID))
				return existingGroup, nil
			}
		}
		return nil, fmt.Errorf("failed to create Schulhof active group: %w", err)
	}

	s.getLogger().Info("created schulhof active group for today",
		slog.String("component", "schulhof"),
		slog.Int64("active_group_id", newActiveGroup.ID))

	return newActiveGroup, nil
}

// findSchulhofActivity finds the dedicated system activity for the canonical
// Schulhof room. Activity names are not unique, so a normal staff activity
// with the same name must not be adopted or block provisioning.
func (s *schulhofService) findSchulhofActivity(ctx context.Context, room *facilities.Room) (*activityModels.Group, error) {
	options := base.NewQueryOptions()
	filter := base.NewFilter()
	filter.Equal("name", constants.SchulhofActivityName)
	options.Filter = filter

	groups, err := s.activityService.ListGroups(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to query Schulhof activity: %w", err)
	}

	for _, group := range groups {
		if ValidateSchulhofActivityRoom(group, room) == nil {
			return group, nil
		}
	}

	return nil, errSchulhofActivityNotFound
}

// findTodayActiveGroup finds an active group for the Schulhof room that started today.
func (s *schulhofService) findTodayActiveGroup(ctx context.Context, roomID, activityGroupID int64) (*active.Group, error) {
	// Get all active groups for this room
	activeGroups, err := s.activeService.FindActiveGroupsByRoomID(ctx, roomID)
	if err != nil {
		return nil, fmt.Errorf("failed to find active groups: %w", err)
	}

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	for _, ag := range activeGroups {
		// Check if it's for the Schulhof activity and started today and not ended.
		// Spontaneous sessions (no template, WP-B6) can never match here.
		templateID, ok := ag.TemplateID()
		if ok && templateID == activityGroupID && ag.StartTime.After(todayStart) && ag.EndTime == nil {
			return ag, nil
		}
	}

	return nil, nil
}

// endStaleActiveGroups ends any active groups for the given room that are still open
// (end_time IS NULL) but started before today. This prevents room conflict errors when
// creating a new daily Schulhof active group.
func (s *schulhofService) endStaleActiveGroups(ctx context.Context, roomID int64) error {
	activeGroups, err := s.activeService.FindActiveGroupsByRoomID(ctx, roomID)
	if err != nil {
		return fmt.Errorf("failed to find active groups: %w", err)
	}

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	for _, ag := range activeGroups {
		// Only end groups from BEFORE today. Today's groups are safe — this ensures
		// mid-day supervisor changes don't disrupt the current Schulhof session.
		if ag.EndTime == nil && !ag.StartTime.After(todayStart) {
			s.getLogger().Info("ending stale schulhof active group",
				slog.String("component", "schulhof"),
				slog.Int64("active_group_id", ag.ID),
				slog.String("started", ag.StartTime.Format("2006-01-02")))
			if err := s.activeService.EndActiveGroupSession(ctx, ag.ID); err != nil {
				return fmt.Errorf("failed to end stale active group %d: %w", ag.ID, err)
			}
		}
	}

	return nil
}

// ensureSchulhofRoom finds or creates the Schulhof room.
func (s *schulhofService) ensureSchulhofRoom(ctx context.Context) (*facilities.Room, error) {
	// Try to find existing Schulhof room
	room, err := FindCanonicalSchulhofRoom(ctx, s.facilityService)
	if err == nil && room != nil {
		s.getLogger().Info("found existing schulhof room",
			slog.String("component", "schulhof"),
			slog.Int64("room_id", room.ID))
		return room, nil
	}
	if err != nil && !errors.Is(err, ErrRoomNotFound) {
		return nil, fmt.Errorf("failed to look up Schulhof room: %w", err)
	}

	// Room not found - create it
	s.getLogger().Info("schulhof room not found, auto-creating...",
		slog.String("component", "schulhof"))

	capacity := constants.SchulhofRoomCapacity
	category := constants.SchulhofCategoryName
	color := constants.SchulhofColor

	newRoom := &facilities.Room{
		Name:     constants.SchulhofRoomName,
		Capacity: &capacity,
		Category: &category,
		Color:    &color,
		IsSystem: true,
	}

	if err := s.facilityService.CreateRoom(ctx, newRoom); err != nil {
		return nil, fmt.Errorf("failed to create Schulhof room: %w", err)
	}

	s.getLogger().Info("successfully created schulhof room",
		slog.String("component", "schulhof"),
		slog.Int64("room_id", newRoom.ID))
	return newRoom, nil
}

// ensureSchulhofCategory finds or creates the Schulhof activity category.
func (s *schulhofService) ensureSchulhofCategory(ctx context.Context) (*activityModels.Category, error) {
	// Try to find existing Schulhof category
	categories, err := s.activityService.ListCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list activity categories: %w", err)
	}

	for _, cat := range categories {
		if cat.Name == constants.SchulhofCategoryName {
			s.getLogger().Info("found existing schulhof category",
				slog.String("component", "schulhof"),
				slog.Int64("category_id", cat.ID))
			return cat, nil
		}
	}

	// Category not found - create it
	s.getLogger().Info("schulhof category not found, auto-creating...",
		slog.String("component", "schulhof"))

	newCategory := &activityModels.Category{
		Name:        constants.SchulhofCategoryName,
		Description: constants.SchulhofCategoryDescription,
		Color:       constants.SchulhofColor,
		IsSystem:    true,
	}

	createdCategory, err := s.activityService.CreateCategory(ctx, newCategory)
	if err != nil {
		return nil, fmt.Errorf("failed to create Schulhof category: %w", err)
	}

	s.getLogger().Info("successfully created schulhof category",
		slog.String("component", "schulhof"),
		slog.Int64("category_id", createdCategory.ID))
	return createdCategory, nil
}
