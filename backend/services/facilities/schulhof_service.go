// Package facilities provides Schulhof (schoolyard) service for managing the permanent
// outdoor area with supervisor toggling capabilities.
package facilities

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/models/active"
	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/activities"
	"github.com/uptrace/bun"
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
	// Returns the activity group ID.
	EnsureInfrastructure(ctx context.Context) (*activityModels.Group, error)

	// GetOrCreateActiveGroup returns the active Schulhof group for today,
	// creating one if it doesn't exist.
	GetOrCreateActiveGroup(ctx context.Context) (*active.Group, error)
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
	db              *bun.DB
}

// NewSchulhofService creates a new Schulhof service.
func NewSchulhofService(
	facilityService Service,
	activityService activities.ActivityService,
	activeService activeSvc.Service,
	db *bun.DB,
) SchulhofService {
	return &schulhofService{
		facilityService: facilityService,
		activityService: activityService,
		activeService:   activeService,
		db:              db,
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
	room, err := s.facilityService.FindRoomByName(ctx, constants.SchulhofRoomName)
	if err != nil {
		// Room doesn't exist yet - return status with exists=false
		log.Printf("%s Room not found, infrastructure not yet created", constants.SchulhofLogPrefix)
		return status, nil
	}
	status.Exists = true
	status.RoomID = &room.ID

	// Step 2: Find Schulhof activity group
	activityGroup, err := s.findSchulhofActivity(ctx)
	if err != nil {
		log.Printf("%s Activity not found: %v", constants.SchulhofLogPrefix, err)
		return status, nil
	}
	status.ActivityGroupID = &activityGroup.ID

	// Step 3: Find today's active group for this room
	activeGroup, err := s.findTodayActiveGroup(ctx, room.ID, activityGroup.ID)
	if err != nil || activeGroup == nil {
		// No active session today - still return status with exists=true
		return status, nil
	}
	status.ActiveGroupID = &activeGroup.ID

	// Step 4: Get supervisors for this active group
	supervisors, err := s.activeService.FindSupervisorsByActiveGroupID(ctx, activeGroup.ID)
	if err != nil {
		log.Printf("%s Error fetching supervisors: %v", constants.SchulhofLogPrefix, err)
	} else {
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
	}

	// Step 5: Count students in this active group
	visits, err := s.activeService.FindVisitsByActiveGroupID(ctx, activeGroup.ID)
	if err != nil {
		log.Printf("%s Error fetching visits: %v", constants.SchulhofLogPrefix, err)
	} else {
		for _, visit := range visits {
			if visit.ExitTime == nil {
				status.StudentCount++
			}
		}
	}

	return status, nil
}

// ToggleSupervision starts or stops supervision for the given staff member.
func (s *schulhofService) ToggleSupervision(ctx context.Context, staffID int64, action string) (*SupervisionResult, error) {
	if action != "start" && action != "stop" {
		return nil, fmt.Errorf("invalid action: %s (must be 'start' or 'stop')", action)
	}

	// Ensure infrastructure exists
	_, err := s.EnsureInfrastructure(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure Schulhof infrastructure: %w", err)
	}

	// Get or create active group for today
	activeGroup, err := s.GetOrCreateActiveGroup(ctx)
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
func (s *schulhofService) EnsureInfrastructure(ctx context.Context) (*activityModels.Group, error) {
	// Check if activity already exists
	activityGroup, err := s.findSchulhofActivity(ctx)
	if err == nil && activityGroup != nil {
		return activityGroup, nil
	}

	log.Printf("%s Infrastructure not found, auto-creating...", constants.SchulhofLogPrefix)

	// Step 1: Ensure Schulhof room exists
	room, err := s.ensureSchulhofRoom(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure Schulhof room: %w", err)
	}

	// Step 2: Ensure Schulhof category exists
	category, err := s.ensureSchulhofCategory(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure Schulhof category: %w", err)
	}

	// Step 3: Create the Schulhof activity group
	newActivity := &activityModels.Group{
		Name:            constants.SchulhofActivityName,
		MaxParticipants: constants.SchulhofMaxParticipants,
		IsOpen:          true, // Open activity - anyone can join
		CategoryID:      category.ID,
		PlannedRoomID:   &room.ID,
	}

	createdActivity, err := s.activityService.CreateGroup(ctx, newActivity, []int64{}, []*activityModels.Schedule{})
	if err != nil {
		return nil, fmt.Errorf("failed to create Schulhof activity: %w", err)
	}

	log.Printf("%s Successfully created infrastructure: room=%d, category=%d, activity=%d",
		constants.SchulhofLogPrefix, room.ID, category.ID, createdActivity.ID)

	return createdActivity, nil
}

// GetOrCreateActiveGroup returns the active Schulhof group for today.
func (s *schulhofService) GetOrCreateActiveGroup(ctx context.Context) (*active.Group, error) {
	// Ensure infrastructure exists
	activityGroup, err := s.EnsureInfrastructure(ctx)
	if err != nil {
		return nil, err
	}

	// Get the room
	room, err := s.facilityService.FindRoomByName(ctx, constants.SchulhofRoomName)
	if err != nil {
		return nil, fmt.Errorf("failed to find Schulhof room: %w", err)
	}

	// Find today's active group
	activeGroup, err := s.findTodayActiveGroup(ctx, room.ID, activityGroup.ID)
	if err == nil && activeGroup != nil {
		return activeGroup, nil
	}

	// Create a new active group for today
	now := time.Now()
	newActiveGroup := &active.Group{
		GroupID:   activityGroup.ID,
		RoomID:    room.ID,
		StartTime: now,
	}

	if err := s.activeService.CreateActiveGroup(ctx, newActiveGroup); err != nil {
		return nil, fmt.Errorf("failed to create Schulhof active group: %w", err)
	}

	log.Printf("%s Created active group for today: ID=%d", constants.SchulhofLogPrefix, newActiveGroup.ID)

	return newActiveGroup, nil
}

// findSchulhofActivity finds the Schulhof activity group by name.
func (s *schulhofService) findSchulhofActivity(ctx context.Context) (*activityModels.Group, error) {
	options := base.NewQueryOptions()
	filter := base.NewFilter()
	filter.Equal("name", constants.SchulhofActivityName)
	options.Filter = filter

	groups, err := s.activityService.ListGroups(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to query Schulhof activity: %w", err)
	}

	if len(groups) == 0 {
		return nil, fmt.Errorf("schulhof activity not found")
	}

	return groups[0], nil
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
		// Check if it's for the Schulhof activity and started today and not ended
		if ag.GroupID == activityGroupID && ag.StartTime.After(todayStart) && ag.EndTime == nil {
			return ag, nil
		}
	}

	return nil, nil
}

// ensureSchulhofRoom finds or creates the Schulhof room.
func (s *schulhofService) ensureSchulhofRoom(ctx context.Context) (*facilities.Room, error) {
	// Try to find existing Schulhof room
	room, err := s.facilityService.FindRoomByName(ctx, constants.SchulhofRoomName)
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

	if err := s.facilityService.CreateRoom(ctx, newRoom); err != nil {
		return nil, fmt.Errorf("failed to create Schulhof room: %w", err)
	}

	log.Printf("%s Successfully created room: ID=%d", constants.SchulhofLogPrefix, newRoom.ID)
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
			log.Printf("%s Found existing category: ID=%d", constants.SchulhofLogPrefix, cat.ID)
			return cat, nil
		}
	}

	// Category not found - create it
	log.Printf("%s Category not found, auto-creating...", constants.SchulhofLogPrefix)

	newCategory := &activityModels.Category{
		Name:        constants.SchulhofCategoryName,
		Description: constants.SchulhofCategoryDescription,
		Color:       constants.SchulhofColor,
	}

	createdCategory, err := s.activityService.CreateCategory(ctx, newCategory)
	if err != nil {
		return nil, fmt.Errorf("failed to create Schulhof category: %w", err)
	}

	log.Printf("%s Successfully created category: ID=%d", constants.SchulhofLogPrefix, createdCategory.ID)
	return createdCategory, nil
}
