// Package facilities provides Schulhof (schoolyard) service for managing the permanent
// outdoor area with supervisor toggling capabilities.
package facilities

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/models/active"
	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/activities"
)

// SchulhofService provides the read model and infrastructure bootstrap for the
// Schulhof (schoolyard) area. Since #2161 the Schulhof is a regular plannable
// room: sessions there are started via the generic timetable/spontaneous flows
// and supervision uses the generic claim/end-supervision endpoints. This
// service only answers "what is going on in the Schulhof right now" and
// auto-creates the reserved infrastructure (room, category, activity) that the
// IoT checkin path and the kiosk checkout button depend on.
type SchulhofService interface {
	// GetSchulhofStatus returns the current status of the Schulhof area including
	// room info, the caller's newest supervised group (or otherwise the newest
	// open group), supervisors, and student count.
	GetSchulhofStatus(ctx context.Context, staffID int64) (*SchulhofStatus, error)

	// EnsureInfrastructure ensures the Schulhof room, category, and activity group exist.
	// createdBy is the staff ID to use when creating new infrastructure.
	// Returns the activity group.
	EnsureInfrastructure(ctx context.Context, createdBy int64) (*activityModels.Group, error)
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

	// Step 2: Find the Schulhof activity group. Purely informational since
	// #2161 — sessions in the room are no longer required to be backed by the
	// system activity, so a missing activity does not end the lookup.
	activityGroup, err := s.findSchulhofActivity(ctx, room)
	switch {
	case err == nil:
		status.ActivityGroupID = &activityGroup.ID
	case errors.Is(err, errSchulhofActivityNotFound):
		s.getLogger().Info("schulhof activity not found",
			slog.String("component", "schulhof"))
	default:
		return nil, fmt.Errorf("failed to look up Schulhof activity: %w", err)
	}

	// Step 3: Prefer the caller's newest supervised group, then fall back to the
	// newest open group in the room. Planned timetable blocks, spontaneous
	// sessions, and IoT fallback groups all count (#2161). The caller preference
	// keeps an existing supervisor's session manageable when a later parallel
	// session starts in the same room.
	activeGroup, supervisors, err := s.findPreferredOpenActiveGroup(ctx, room.ID, staffID)
	if err != nil {
		return nil, fmt.Errorf("failed to look up open Schulhof group: %w", err)
	}
	if activeGroup == nil {
		// No open session - still return status with exists=true
		return status, nil
	}
	status.ActiveGroupID = &activeGroup.ID

	// Step 4: Map the active supervisors loaded while selecting the group.
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

// findPreferredOpenActiveGroup returns the caller's newest actively supervised
// group in the room, or the newest open group when the caller supervises none.
// It returns the selected group's active supervisors with the group so status
// rendering does not need a second lookup.
func (s *schulhofService) findPreferredOpenActiveGroup(
	ctx context.Context,
	roomID int64,
	staffID int64,
) (*active.Group, []*active.GroupSupervisor, error) {
	activeGroups, err := s.activeService.FindActiveGroupsByRoomID(ctx, roomID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find active groups: %w", err)
	}

	openGroups := make([]*active.Group, 0, len(activeGroups))
	openGroupIDs := make([]int64, 0, len(activeGroups))
	var newestOpen *active.Group
	for _, ag := range activeGroups {
		if ag.EndTime != nil {
			continue
		}
		openGroups = append(openGroups, ag)
		openGroupIDs = append(openGroupIDs, ag.ID)
		if newerActiveGroup(ag, newestOpen) {
			newestOpen = ag
		}
	}
	if newestOpen == nil {
		return nil, nil, nil
	}

	allSupervisors, err := s.activeService.FindSupervisorsByActiveGroupIDs(ctx, openGroupIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find active supervisors: %w", err)
	}

	supervisorsByGroup := make(map[int64][]*active.GroupSupervisor, len(openGroups))
	for _, supervisor := range allSupervisors {
		supervisorsByGroup[supervisor.GroupID] = append(supervisorsByGroup[supervisor.GroupID], supervisor)
	}

	var newestSupervised *active.Group
	for _, group := range openGroups {
		for _, supervisor := range supervisorsByGroup[group.ID] {
			if supervisor.StaffID == staffID && newerActiveGroup(group, newestSupervised) {
				newestSupervised = group
				break
			}
		}
	}

	selected := newestOpen
	if newestSupervised != nil {
		selected = newestSupervised
	}
	return selected, supervisorsByGroup[selected.ID], nil
}

func newerActiveGroup(candidate, current *active.Group) bool {
	return current == nil ||
		candidate.StartTime.After(current.StartTime) ||
		(candidate.StartTime.Equal(current.StartTime) && candidate.ID > current.ID)
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
