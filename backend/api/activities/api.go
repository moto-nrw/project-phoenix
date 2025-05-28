package activities

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	activitiesSvc "github.com/moto-nrw/project-phoenix/services/activities"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// Resource defines the activities API resource
type Resource struct {
	ActivityService activitiesSvc.ActivityService
	ScheduleService scheduleSvc.Service
	UserService     usersSvc.PersonService
}

// NewResource creates a new activities resource
func NewResource(activityService activitiesSvc.ActivityService, scheduleService scheduleSvc.Service, userService usersSvc.PersonService) *Resource {
	return &Resource{
		ActivityService: activityService,
		ScheduleService: scheduleService,
		UserService:     userService,
	}
}

// Router returns a configured router for activity endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth, _ := jwt.NewTokenAuth()

	// Protected routes that require authentication and permissions
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)

		// Basic Activity Group operations (Read)
		r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Get("/", rs.listActivities)
		r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Get("/{id}", rs.getActivity)
		r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Get("/categories", rs.listCategories)
		r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Get("/timespans", rs.getTimespans)

		// Basic Activity Group operations (Write)
		r.With(authorize.RequiresPermission(permissions.ActivitiesCreate)).Post("/", rs.createActivity)
		r.With(authorize.RequiresPermission(permissions.ActivitiesUpdate)).Put("/{id}", rs.updateActivity)
		r.With(authorize.RequiresPermission(permissions.ActivitiesDelete)).Delete("/{id}", rs.deleteActivity)

		// Schedule Management
		r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Get("/{id}/schedules", rs.getActivitySchedules)
		r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Get("/{id}/schedules/{scheduleId}", rs.getActivitySchedule)
		r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Get("/schedules/available", rs.getAvailableTimeSlots)
		r.With(authorize.RequiresPermission(permissions.ActivitiesUpdate)).Post("/{id}/schedules", rs.createActivitySchedule)
		r.With(authorize.RequiresPermission(permissions.ActivitiesUpdate)).Put("/{id}/schedules/{scheduleId}", rs.updateActivitySchedule)
		r.With(authorize.RequiresPermission(permissions.ActivitiesUpdate)).Delete("/{id}/schedules/{scheduleId}", rs.deleteActivitySchedule)

		// Supervisor Assignment
		r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Get("/{id}/supervisors", rs.getActivitySupervisors)
		r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Get("/supervisors/available", rs.getAvailableSupervisors)
		r.With(authorize.RequiresPermission(permissions.ActivitiesUpdate)).Post("/{id}/supervisors", rs.assignSupervisor)
		r.With(authorize.RequiresPermission(permissions.ActivitiesUpdate)).Put("/{id}/supervisors/{supervisorId}", rs.updateSupervisorRole)
		r.With(authorize.RequiresPermission(permissions.ActivitiesUpdate)).Delete("/{id}/supervisors/{supervisorId}", rs.removeSupervisor)

		// Student Enrollment
		r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Get("/{id}/students", rs.getActivityStudents)
		r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Get("/students/{studentId}", rs.getStudentEnrollments)
		r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Get("/students/{studentId}/available", rs.getAvailableActivities)
		r.With(authorize.RequiresPermission(permissions.ActivitiesEnroll)).Post("/{id}/students/{studentId}", rs.enrollStudent)
		r.With(authorize.RequiresPermission(permissions.ActivitiesEnroll)).Delete("/{id}/students/{studentId}", rs.unenrollStudent)
		r.With(authorize.RequiresPermission(permissions.ActivitiesEnroll)).Put("/{id}/students", rs.updateGroupEnrollments)
	})

	return r
}

// CategoryResponse represents a category API response
type CategoryResponse struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Color       string    `json:"color,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SupervisorResponse represents a supervisor in activity response
type SupervisorResponse struct {
	ID        int64  `json:"id"`
	StaffID   int64  `json:"staff_id"`
	IsPrimary bool   `json:"is_primary"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

// ActivityResponse represents an activity group API response
type ActivityResponse struct {
	ID              int64                `json:"id"`
	Name            string               `json:"name"`
	MaxParticipants int                  `json:"max_participants"`
	IsOpen          bool                 `json:"is_open"`
	CategoryID      int64                `json:"category_id"`
	PlannedRoomID   *int64               `json:"planned_room_id,omitempty"`
	Category        *CategoryResponse    `json:"category,omitempty"`
	SupervisorID    *int64               `json:"supervisor_id,omitempty"`  // Primary supervisor
	SupervisorIDs   []int64              `json:"supervisor_ids,omitempty"` // All supervisors
	Supervisors     []SupervisorResponse `json:"supervisors,omitempty"`    // Detailed supervisor info
	Schedules       []ScheduleResponse   `json:"schedules,omitempty"`
	EnrollmentCount int                  `json:"enrollment_count,omitempty"`
	CreatedAt       time.Time            `json:"created_at"`
	UpdatedAt       time.Time            `json:"updated_at"`
}

// ScheduleResponse represents a schedule API response
type ScheduleResponse struct {
	ID              int64     `json:"id"`
	Weekday         int       `json:"weekday"`
	TimeframeID     *int64    `json:"timeframe_id,omitempty"`
	ActivityGroupID int64     `json:"activity_group_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// StudentResponse represents a simplified student in activity response
type StudentResponse struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// TimespanResponse represents a time span for activities
type TimespanResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	Description string `json:"description,omitempty"`
}

// ActivityRequest represents an activity creation/update request
type ActivityRequest struct {
	Name            string            `json:"name"`
	MaxParticipants int               `json:"max_participants"`
	IsOpen          bool              `json:"is_open"`
	CategoryID      int64             `json:"category_id"`
	PlannedRoomID   *int64            `json:"planned_room_id,omitempty"`
	Schedules       []ScheduleRequest `json:"schedules,omitempty"`
	SupervisorIDs   []int64           `json:"supervisor_ids,omitempty"`
}

// ScheduleRequest represents a schedule in activity creation/update request
type ScheduleRequest struct {
	Weekday     int    `json:"weekday"`
	TimeframeID *int64 `json:"timeframe_id,omitempty"`
}

// Bind validates the activity request
func (req *ActivityRequest) Bind(r *http.Request) error {
	if req.Name == "" {
		return errors.New("activity name is required")
	}
	if req.MaxParticipants <= 0 {
		return errors.New("max participants must be greater than zero")
	}
	if req.CategoryID <= 0 {
		return errors.New("category ID is required")
	}

	// Validate schedules if provided
	if len(req.Schedules) > 0 {
		for _, schedule := range req.Schedules {
			if !activities.IsValidWeekday(schedule.Weekday) {
				return errors.New("invalid weekday in schedule")
			}
		}
	}

	return nil
}

// newCategoryResponse converts a category model to a response object
func newCategoryResponse(category *activities.Category) CategoryResponse {
	// Handle nil category input
	if category == nil {
		log.Printf("Warning: Attempted to create CategoryResponse from nil category")
		return CategoryResponse{
			Name: "Unknown Category", // Provide a safe default
		}
	}

	return CategoryResponse{
		ID:          category.ID,
		Name:        category.Name,
		Description: category.Description,
		Color:       category.Color,
		CreatedAt:   category.CreatedAt,
		UpdatedAt:   category.UpdatedAt,
	}
}

// newActivityResponse converts an activity group model to a response object
func newActivityResponse(group *activities.Group, enrollmentCount int) ActivityResponse {
	// Check if group is nil to prevent panic
	if group == nil {
		log.Printf("Error: Attempted to create ActivityResponse from nil group")
		// Return empty response rather than panic
		return ActivityResponse{}
	}

	// Create response with only the direct fields that are guaranteed to be safe
	response := ActivityResponse{
		ID:              group.ID,
		Name:            group.Name,
		MaxParticipants: group.MaxParticipants,
		IsOpen:          group.IsOpen,
		CategoryID:      group.CategoryID,
		EnrollmentCount: enrollmentCount,
		CreatedAt:       group.CreatedAt,
		UpdatedAt:       group.UpdatedAt,
		Schedules:       []ScheduleResponse{},
	}

	// Safely add optional fields with nil checks
	if group.PlannedRoomID != nil {
		response.PlannedRoomID = group.PlannedRoomID
	}

	// Add category details if available - with extra nil checks
	if group.Category != nil {
		category := newCategoryResponse(group.Category)
		response.Category = &category
	}

	// Add schedules if available - with thorough nil checking
	if group.Schedules != nil {
		// Create a new slice with proper capacity
		scheduleResponses := make([]ScheduleResponse, 0, len(group.Schedules))

		// Process each schedule, skipping nil ones
		for _, schedule := range group.Schedules {
			// Skip nil schedules to prevent panic
			if schedule == nil {
				log.Printf("Warning: Nil schedule encountered in group ID %d", group.ID)
				continue
			}

			// Create schedule response with safer access to fields
			scheduleResponse := ScheduleResponse{
				ID:              schedule.ID,
				Weekday:         schedule.Weekday,
				ActivityGroupID: schedule.ActivityGroupID,
				CreatedAt:       schedule.CreatedAt,
				UpdatedAt:       schedule.UpdatedAt,
			}

			// Safely add optional fields
			if schedule.TimeframeID != nil {
				scheduleResponse.TimeframeID = schedule.TimeframeID
			}

			scheduleResponses = append(scheduleResponses, scheduleResponse)
		}

		// Only assign if we actually have schedules
		if len(scheduleResponses) > 0 {
			response.Schedules = scheduleResponses
		}
	}

	return response
}

// listActivities handles listing all activities with optional filtering
func (rs *Resource) listActivities(w http.ResponseWriter, r *http.Request) {
	// Get filter parameters
	categoryIDStr := r.URL.Query().Get("category_id")

	// Create query options with filter
	queryOptions := base.NewQueryOptions()
	filter := base.NewFilter()

	// Apply filters
	if categoryIDStr != "" {
		categoryID, err := strconv.ParseInt(categoryIDStr, 10, 64)
		if err == nil {
			filter.Equal("category_id", categoryID)
		}
	}

	// Set the filter to query options
	queryOptions.Filter = filter

	// Get activities
	groups, err := rs.ActivityService.ListGroups(r.Context(), queryOptions)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get enrollment counts
	_, enrollmentCounts, err := rs.ActivityService.GetGroupsWithEnrollmentCounts(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Build response with supervisors
	responses := make([]ActivityResponse, 0, len(groups))
	for _, group := range groups {
		count := enrollmentCounts[group.ID]
		activityResp := newActivityResponse(group, count)

		// Get supervisors for this group
		supervisors, err := rs.ActivityService.GetGroupSupervisors(r.Context(), group.ID)
		if err == nil && len(supervisors) > 0 {
			supervisorResponses := make([]SupervisorResponse, 0, len(supervisors))
			for _, supervisor := range supervisors {
				supervisorResp := SupervisorResponse{
					ID:        supervisor.ID,
					StaffID:   supervisor.StaffID,
					IsPrimary: supervisor.IsPrimary,
				}

				// Add person details if available
				if supervisor.Staff != nil && supervisor.Staff.Person != nil {
					supervisorResp.FirstName = supervisor.Staff.Person.FirstName
					supervisorResp.LastName = supervisor.Staff.Person.LastName
				}

				supervisorResponses = append(supervisorResponses, supervisorResp)
			}
			activityResp.Supervisors = supervisorResponses
		}

		responses = append(responses, activityResp)
	}

	common.Respond(w, r, http.StatusOK, responses, "Activities retrieved successfully")
}

// getActivity handles getting an activity by ID
func (rs *Resource) getActivity(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid activity ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Try to get group with full details first
	group, supervisors, schedules, detailsErr := rs.ActivityService.GetGroupWithDetails(r.Context(), id)
	if detailsErr != nil {
		log.Printf("Warning: Error getting detailed group info: %v", detailsErr)

		// Fall back to basic group info if detailed fetch fails
		group, err = rs.ActivityService.GetGroup(r.Context(), id)
		if err != nil {
			if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
				log.Printf("Error rendering error response: %v", err)
			}
			return
		}

		// Get schedules separately in the fallback case
		var scheduleErr error
		schedules, scheduleErr = rs.ActivityService.GetGroupSchedules(r.Context(), id)
		if scheduleErr != nil {
			// Log but continue without schedules
			log.Printf("Warning: Error getting schedules: %v", scheduleErr)
			schedules = []*activities.Schedule{} // Empty slice instead of nil
		}

		// Try to get supervisors separately
		supervisors, _ = rs.ActivityService.GetGroupSupervisors(r.Context(), id)
	}

	// Check if group is nil to prevent panic
	if group == nil {
		log.Printf("Error: Group is nil after GetGroup call for ID %d", id)
		if err := render.Render(w, r, ErrorInternalServer(errors.New("activity not found or could not be retrieved"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Try to load category if it's not already loaded
	if group.Category == nil && group.CategoryID > 0 {
		category, catErr := rs.ActivityService.GetCategory(r.Context(), group.CategoryID)
		if catErr != nil {
			log.Printf("Warning: Error getting category for ID %d: %v", group.CategoryID, catErr)
		} else if category != nil {
			group.Category = category
		}
	}

	// Setup enrollment count (defaulting to 0)
	enrollmentCount := 0

	// Try to get enrollment count, but continue even if it fails
	enrolledStudents, err := rs.ActivityService.GetEnrolledStudents(r.Context(), id)
	if err != nil {
		// Log error but continue - we'll just use 0 for enrollment count
		log.Printf("Error getting enrolled students: %v", err)
	} else if enrolledStudents != nil {
		enrollmentCount = len(enrolledStudents)
	}

	// Create a direct response with only necessary fields
	response := ActivityResponse{
		ID:              group.ID,
		Name:            group.Name,
		MaxParticipants: group.MaxParticipants,
		IsOpen:          group.IsOpen,
		CategoryID:      group.CategoryID,
		PlannedRoomID:   group.PlannedRoomID,
		EnrollmentCount: enrollmentCount,
		CreatedAt:       group.CreatedAt,
		UpdatedAt:       group.UpdatedAt,
		Schedules:       []ScheduleResponse{}, // Initialize with empty array
	}

	// Add category to response if available
	if group.Category != nil {
		category := newCategoryResponse(group.Category)
		response.Category = &category
	}

	// Create a response with supervisor details if available
	if len(supervisors) > 0 {
		// Extract supervisor IDs and detailed info
		supervisorIDs := make([]int64, 0, len(supervisors))
		supervisorDetails := make([]SupervisorResponse, 0, len(supervisors))

		for _, supervisor := range supervisors {
			if supervisor != nil {
				supervisorIDs = append(supervisorIDs, supervisor.StaffID)
				// If this is the primary supervisor, add it to the response
				if supervisor.IsPrimary {
					response.SupervisorID = &supervisor.StaffID
				}

				// Create detailed supervisor response
				supervisorResp := SupervisorResponse{
					ID:        supervisor.ID,
					StaffID:   supervisor.StaffID,
					IsPrimary: supervisor.IsPrimary,
				}

				// Add staff details if available
				if supervisor.Staff != nil && supervisor.Staff.Person != nil {
					supervisorResp.FirstName = supervisor.Staff.Person.FirstName
					supervisorResp.LastName = supervisor.Staff.Person.LastName
				}

				supervisorDetails = append(supervisorDetails, supervisorResp)
			}
		}

		// Add all supervisor IDs and details to the response
		if len(supervisorIDs) > 0 {
			response.SupervisorIDs = supervisorIDs
			response.Supervisors = supervisorDetails
		}
	}

	// Add schedules to response if available
	if len(schedules) > 0 {
		responseSchedules := make([]ScheduleResponse, 0, len(schedules))
		for _, schedule := range schedules {
			if schedule != nil {
				responseSchedules = append(responseSchedules, ScheduleResponse{
					ID:              schedule.ID,
					Weekday:         schedule.Weekday,
					TimeframeID:     schedule.TimeframeID,
					ActivityGroupID: schedule.ActivityGroupID,
					CreatedAt:       schedule.CreatedAt,
					UpdatedAt:       schedule.UpdatedAt,
				})
			}
		}
		if len(responseSchedules) > 0 {
			response.Schedules = responseSchedules
		}
	}

	common.Respond(w, r, http.StatusOK, response, "Activity retrieved successfully")
}

// createActivity handles creating a new activity
func (rs *Resource) createActivity(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &ActivityRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Create activity group
	group := &activities.Group{
		Name:            req.Name,
		MaxParticipants: req.MaxParticipants,
		IsOpen:          req.IsOpen,
		CategoryID:      req.CategoryID,
		PlannedRoomID:   req.PlannedRoomID,
	}

	// Prepare schedules

	schedules := make([]*activities.Schedule, 0, len(req.Schedules))
	for _, s := range req.Schedules {
		schedules = append(schedules, &activities.Schedule{
			Weekday:     s.Weekday,
			TimeframeID: s.TimeframeID,
		})
	}

	// Create the activity group with schedules and supervisors
	createdGroup, err := rs.ActivityService.CreateGroup(r.Context(), group, req.SupervisorIDs, schedules)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// EXTREMELY SIMPLIFIED APPROACH - don't try to get additional details at all
	// Just create a response with what we know is valid and return it
	if createdGroup == nil {
		// This should never happen if CreateGroup didn't return an error, but just in case
		log.Printf("Warning: CreateGroup returned nil group without error")
		common.Respond(w, r, http.StatusCreated, ActivityResponse{
			Name:       req.Name, // Use the original request data as fallback
			CategoryID: req.CategoryID,
			Schedules:  []ScheduleResponse{},
		}, "Activity created successfully")
		return
	}

	// Create a direct response with minimal fields
	response := ActivityResponse{
		ID:              createdGroup.ID,
		Name:            createdGroup.Name,
		MaxParticipants: createdGroup.MaxParticipants,
		IsOpen:          createdGroup.IsOpen,
		CategoryID:      createdGroup.CategoryID,
		PlannedRoomID:   createdGroup.PlannedRoomID,
		CreatedAt:       createdGroup.CreatedAt,
		UpdatedAt:       createdGroup.UpdatedAt,
		EnrollmentCount: 0,
		Schedules:       []ScheduleResponse{}, // Always use empty slice, not nil
	}

	common.Respond(w, r, http.StatusCreated, response, "Activity created successfully")
}

// updateActivity handles updating an activity
func (rs *Resource) updateActivity(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid activity ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse request
	req := &ActivityRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Get existing group
	existingGroup, err := rs.ActivityService.GetGroup(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Update fields
	existingGroup.Name = req.Name
	existingGroup.MaxParticipants = req.MaxParticipants
	existingGroup.IsOpen = req.IsOpen
	existingGroup.CategoryID = req.CategoryID
	existingGroup.PlannedRoomID = req.PlannedRoomID

	// Update the group
	updatedGroup, err := rs.ActivityService.UpdateGroup(r.Context(), existingGroup)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Handle supervisor updates - always process since frontend always sends this field
	if true { // Always process supervisor updates

		// First, remove all existing supervisors for this group
		existingSupervisors, err := rs.ActivityService.GetGroupSupervisors(r.Context(), updatedGroup.ID)
		if err != nil {
			log.Printf("Warning: Failed to get existing supervisors: %v", err)
		} else {
			for _, supervisor := range existingSupervisors {
				err = rs.ActivityService.DeleteSupervisor(r.Context(), supervisor.ID)
				if err != nil {
					log.Printf("Warning: Failed to delete supervisor with ID %d: %v", supervisor.ID, err)
				}
			}
		}

		// Then add the new supervisors
		for i, staffID := range req.SupervisorIDs {
			isPrimary := i == 0 // First supervisor is primary
			_, err = rs.ActivityService.AddSupervisor(r.Context(), updatedGroup.ID, staffID, isPrimary)
			if err != nil {
				log.Printf("Warning: Failed to add supervisor %d to activity %d: %v", staffID, updatedGroup.ID, err)
			}
		}
	}

	// Handle schedule updates - similar to supervisor handling
	if true { // Always process schedule updates
		// First, get existing schedules
		existingSchedules, err := rs.ActivityService.GetGroupSchedules(r.Context(), updatedGroup.ID)
		if err != nil {
			log.Printf("Warning: Failed to get existing schedules: %v", err)
		} else {
			// Remove all existing schedules
			for _, schedule := range existingSchedules {
				err = rs.ActivityService.DeleteSchedule(r.Context(), schedule.ID)
				if err != nil {
					log.Printf("Warning: Failed to delete schedule with ID %d: %v", schedule.ID, err)
				}
			}
		}

		// Add the new schedules
		for _, scheduleReq := range req.Schedules {
			schedule := &activities.Schedule{
				Weekday:     scheduleReq.Weekday,
				TimeframeID: scheduleReq.TimeframeID,
			}
			_, err = rs.ActivityService.AddSchedule(r.Context(), updatedGroup.ID, schedule)
			if err != nil {
				log.Printf("Warning: Failed to add schedule (weekday=%d, timeframe=%v): %v", scheduleReq.Weekday, scheduleReq.TimeframeID, err)
				// Don't fail the whole update, just log the warning
			}
		}
	}

	// Get the updated group with details
	detailedGroup, _, updatedSchedules, err := rs.ActivityService.GetGroupWithDetails(r.Context(), updatedGroup.ID)
	if err != nil {
		log.Printf("Failed to get detailed group info after update: %v", err)
		// Return response with basic info if we can't get detailed info
		if updatedGroup != nil {
			// Ensure we have a valid empty schedules array rather than nil
			updatedGroup.Schedules = []*activities.Schedule{} // Empty slice instead of nil
			response := newActivityResponse(updatedGroup, 0)
			common.Respond(w, r, http.StatusOK, response, "Activity updated successfully")
		} else {
			log.Printf("Error: updatedGroup is nil after update")
			common.Respond(w, r, http.StatusOK, ActivityResponse{}, "Activity updated but details could not be retrieved")
		}
		return
	}

	// If we successfully got detailed info, use that instead
	if detailedGroup != nil {
		// Add schedules to group, never use nil
		if updatedSchedules != nil {
			updatedGroup.Schedules = updatedSchedules
		} else {
			log.Printf("Warning: updatedSchedules is nil despite no error from GetGroupWithDetails")
			updatedGroup.Schedules = []*activities.Schedule{} // Empty slice instead of nil
		}

		// Note: supervisors are loaded but not currently processed
	} else {
		log.Printf("Warning: detailedGroup is nil despite no error from GetGroupWithDetails")
		// If detailedGroup is nil but updatedGroup is not, make sure it has valid schedules
		if updatedGroup != nil {
			updatedGroup.Schedules = []*activities.Schedule{} // Empty slice instead of nil
		}
	}

	// Final safety check to ensure updatedGroup is not nil
	if updatedGroup == nil {
		log.Printf("Error: updatedGroup is nil before creating response. Returning empty response.")
		common.Respond(w, r, http.StatusOK, ActivityResponse{}, "Activity updated but details could not be retrieved")
		return
	}

	// Get enrollment count
	enrolledStudents, err := rs.ActivityService.GetEnrolledStudents(r.Context(), id)
	if err != nil {
		log.Printf("Failed to get enrolled students: %v", err)
		response := newActivityResponse(updatedGroup, 0)
		common.Respond(w, r, http.StatusOK, response, "Activity updated successfully")
		return
	}

	response := newActivityResponse(updatedGroup, len(enrolledStudents))
	common.Respond(w, r, http.StatusOK, response, "Activity updated successfully")
}

// deleteActivity handles deleting an activity
func (rs *Resource) deleteActivity(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid activity ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Delete the activity
	if err := rs.ActivityService.DeleteGroup(r.Context(), id); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Activity deleted successfully")
}

// listCategories handles listing all activity categories
func (rs *Resource) listCategories(w http.ResponseWriter, r *http.Request) {
	// Get categories
	categories, err := rs.ActivityService.ListCategories(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Build response
	responses := make([]CategoryResponse, 0, len(categories))
	for _, category := range categories {
		responses = append(responses, newCategoryResponse(category))
	}

	common.Respond(w, r, http.StatusOK, responses, "Categories retrieved successfully")
}

// STUDENT ENROLLMENT HANDLERS

// getActivityStudents handles getting students enrolled in an activity
func (rs *Resource) getActivityStudents(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid activity ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Check if activity exists
	activity, err := rs.ActivityService.GetGroup(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get enrolled students
	students, err := rs.ActivityService.GetEnrolledStudents(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Build simplified student responses
	responses := make([]StudentResponse, 0, len(students))
	for _, student := range students {
		// Skip nil students to prevent panic
		if student == nil {
			continue
		}

		// Create a basic response with the ID
		studentResp := StudentResponse{
			ID: student.ID,
			// Default name values if no person data
			FirstName: "Student",
			LastName:  fmt.Sprintf("%d", student.ID),
		}

		// Check if student has person data
		if student.Person != nil {
			person := student.Person
			studentResp.FirstName = person.FirstName
			studentResp.LastName = person.LastName
		}

		responses = append(responses, studentResp)
	}

	common.Respond(w, r, http.StatusOK, responses, fmt.Sprintf("Students enrolled in activity '%s' retrieved successfully", activity.Name))
}

// getStudentEnrollments handles getting activities that a student is enrolled in
func (rs *Resource) getStudentEnrollments(w http.ResponseWriter, r *http.Request) {
	// Parse student ID from URL
	studentID, err := strconv.ParseInt(chi.URLParam(r, "studentId"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid student ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get activities that student is enrolled in
	enrolledGroups, err := rs.ActivityService.GetStudentEnrollments(r.Context(), studentID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Build activity responses
	responses := make([]ActivityResponse, 0, len(enrolledGroups))
	for _, group := range enrolledGroups {
		if group == nil {
			continue // Skip nil groups to prevent panic
		}

		// Get enrollment count
		// This could be optimized by fetching all counts at once in a real implementation
		students, err := rs.ActivityService.GetEnrolledStudents(r.Context(), group.ID)
		enrollmentCount := 0
		if err == nil && students != nil {
			enrollmentCount = len(students)
		}

		// Create response
		responses = append(responses, newActivityResponse(group, enrollmentCount))
	}

	common.Respond(w, r, http.StatusOK, responses, fmt.Sprintf("Activities for student ID %d retrieved successfully", studentID))
}

// getAvailableActivities handles getting activities available for a student to enroll in
func (rs *Resource) getAvailableActivities(w http.ResponseWriter, r *http.Request) {
	// Parse student ID from URL
	studentID, err := strconv.ParseInt(chi.URLParam(r, "studentId"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid student ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get available activities for student
	availableGroups, err := rs.ActivityService.GetAvailableGroups(r.Context(), studentID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Build activity responses
	responses := make([]ActivityResponse, 0, len(availableGroups))
	for _, group := range availableGroups {
		if group == nil {
			continue // Skip nil groups to prevent panic
		}

		// Get enrollment count
		// This could be optimized by fetching all counts at once in a real implementation
		students, err := rs.ActivityService.GetEnrolledStudents(r.Context(), group.ID)
		enrollmentCount := 0
		if err == nil && students != nil {
			enrollmentCount = len(students)
		}

		// Create response
		responses = append(responses, newActivityResponse(group, enrollmentCount))
	}

	common.Respond(w, r, http.StatusOK, responses, fmt.Sprintf("Available activities for student ID %d retrieved successfully", studentID))
}

// We already have the enrollStudent method, no need to modify it since it follows the standard

// unenrollStudent handles removing a student from an activity
func (rs *Resource) unenrollStudent(w http.ResponseWriter, r *http.Request) {
	// Parse IDs from URL
	activityID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid activity ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	studentID, err := strconv.ParseInt(chi.URLParam(r, "studentId"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid student ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Check if activity exists
	activity, err := rs.ActivityService.GetGroup(r.Context(), activityID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Unenroll student
	if err := rs.ActivityService.UnenrollStudent(r.Context(), activityID, studentID); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, nil, fmt.Sprintf("Student unenrolled from activity '%s' successfully", activity.Name))
}

// BatchEnrollmentRequest represents a request for updating enrollments in batch
type BatchEnrollmentRequest struct {
	StudentIDs []int64 `json:"student_ids"`
}

// Bind validates the batch enrollment request
func (req *BatchEnrollmentRequest) Bind(r *http.Request) error {
	if req.StudentIDs == nil {
		return errors.New("student IDs are required")
	}
	return nil
}

// updateGroupEnrollments handles updating student enrollments in batch
func (rs *Resource) updateGroupEnrollments(w http.ResponseWriter, r *http.Request) {
	// Parse activity ID from URL
	activityID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid activity ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse request
	var req BatchEnrollmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Validate request
	if err := req.Bind(r); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Check if activity exists
	activity, err := rs.ActivityService.GetGroup(r.Context(), activityID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Update group enrollments
	if err := rs.ActivityService.UpdateGroupEnrollments(r.Context(), activityID, req.StudentIDs); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get updated enrollment count for response
	students, err := rs.ActivityService.GetEnrolledStudents(r.Context(), activityID)
	enrollmentCount := 0
	if err == nil && students != nil {
		enrollmentCount = len(students)
	}

	// Create a simplified response
	response := map[string]interface{}{
		"activity_id":       activityID,
		"activity_name":     activity.Name,
		"enrollment_count":  enrollmentCount,
		"max_participants":  activity.MaxParticipants,
		"students_enrolled": req.StudentIDs,
	}

	common.Respond(w, r, http.StatusOK, response, fmt.Sprintf("Enrollments for activity '%s' updated successfully", activity.Name))
}

// enrollStudent handles enrolling a student in an activity
func (rs *Resource) enrollStudent(w http.ResponseWriter, r *http.Request) {
	// Parse IDs from URL
	activityID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid activity ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	studentID, err := strconv.ParseInt(chi.URLParam(r, "studentId"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid student ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Enroll student
	if err := rs.ActivityService.EnrollStudent(r.Context(), activityID, studentID); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Student enrolled successfully")
}

// getTimespans handles retrieving all available time spans for activities
func (rs *Resource) getTimespans(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Fetch active timeframes from the schedule service
	timeframes, err := rs.ScheduleService.FindActiveTimeframes(ctx)
	if err != nil {
		log.Printf("Error fetching timeframes: %v", err)
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to retrieve timeframes")
		return
	}

	// Convert timeframes to TimespanResponse format
	timespans := make([]TimespanResponse, len(timeframes))
	for i, tf := range timeframes {
		timespans[i] = TimespanResponse{
			ID:          tf.ID,
			Name:        tf.Description, // Use description as name for now
			StartTime:   tf.StartTime.Format("15:04"),
			EndTime:     formatEndTime(tf.EndTime),
			Description: tf.Description,
		}
	}

	common.Respond(w, r, http.StatusOK, timespans, "Time spans retrieved successfully")
}

// formatEndTime safely formats the end time, handling nil values
func formatEndTime(endTime *time.Time) string {
	if endTime == nil {
		return ""
	}
	return endTime.Format("15:04")
}

// SCHEDULE MANAGEMENT HANDLERS

// getActivitySchedules retrieves all schedules for a specific activity
func (rs *Resource) getActivitySchedules(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid activity ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Check if activity exists
	activity, err := rs.ActivityService.GetGroup(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get schedules for the activity
	schedules, err := rs.ActivityService.GetGroupSchedules(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Convert to response objects
	responses := make([]ScheduleResponse, 0, len(schedules))
	for _, schedule := range schedules {
		if schedule == nil {
			continue // Skip nil schedules to prevent panic
		}

		responses = append(responses, ScheduleResponse{
			ID:              schedule.ID,
			Weekday:         schedule.Weekday,
			TimeframeID:     schedule.TimeframeID,
			ActivityGroupID: schedule.ActivityGroupID,
			CreatedAt:       schedule.CreatedAt,
			UpdatedAt:       schedule.UpdatedAt,
		})
	}

	common.Respond(w, r, http.StatusOK, responses, fmt.Sprintf("Schedules for activity '%s' retrieved successfully", activity.Name))
}

// getActivitySchedule retrieves a specific schedule by ID
func (rs *Resource) getActivitySchedule(w http.ResponseWriter, r *http.Request) {
	// Parse activity ID from URL
	activityID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid activity ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse schedule ID from URL
	scheduleID, err := strconv.ParseInt(chi.URLParam(r, "scheduleId"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid schedule ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Check if activity exists
	_, err = rs.ActivityService.GetGroup(r.Context(), activityID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get schedule
	schedule, err := rs.ActivityService.GetSchedule(r.Context(), scheduleID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Check if schedule belongs to the specified activity
	if schedule.ActivityGroupID != activityID {
		if err := render.Render(w, r, ErrorForbidden(errors.New("schedule does not belong to the specified activity"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Convert to response object
	response := ScheduleResponse{
		ID:              schedule.ID,
		Weekday:         schedule.Weekday,
		TimeframeID:     schedule.TimeframeID,
		ActivityGroupID: schedule.ActivityGroupID,
		CreatedAt:       schedule.CreatedAt,
		UpdatedAt:       schedule.UpdatedAt,
	}

	common.Respond(w, r, http.StatusOK, response, "Schedule retrieved successfully")
}

// getAvailableTimeSlots retrieves available time slots for scheduling
func (rs *Resource) getAvailableTimeSlots(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Query parameters for filtering
	weekday := r.URL.Query().Get("weekday")
	roomIDStr := r.URL.Query().Get("room_id")
	durationStr := r.URL.Query().Get("duration") // Duration in minutes

	// Validate weekday if provided
	if weekday != "" {
		// Parse weekday as integer
		weekdayInt, err := strconv.Atoi(weekday)
		if err != nil || !activities.IsValidWeekday(weekdayInt) {
			if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid weekday"))); err != nil {
				log.Printf("Error rendering error response: %v", err)
			}
			return
		}
	}

	// Parse room ID if provided (currently unused)
	if roomIDStr != "" {
		_, err := strconv.ParseInt(roomIDStr, 10, 64)
		if err != nil {
			if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid room ID"))); err != nil {
				log.Printf("Error rendering error response: %v", err)
			}
			return
		}
		// Room ID validation complete but not used for filtering yet
	}

	// Parse duration or use default (2 hours)
	duration := 2 * time.Hour // Default duration
	if durationStr != "" {
		minutes, err := strconv.Atoi(durationStr)
		if err != nil || minutes <= 0 {
			if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid duration"))); err != nil {
				log.Printf("Error rendering error response: %v", err)
			}
			return
		}
		duration = time.Duration(minutes) * time.Minute
	}

	// Set date range for the next 7 days
	startDate := time.Now().Truncate(24 * time.Hour)
	endDate := startDate.AddDate(0, 0, 7)

	// Find available time slots using the schedule service
	availableSlots, err := rs.ScheduleService.FindAvailableSlots(ctx, startDate, endDate, duration)
	if err != nil {
		log.Printf("Error finding available time slots: %v", err)
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to retrieve available time slots")
		return
	}

	// Convert available slots to TimespanResponse format and apply filters
	var timespans []TimespanResponse
	for _, slot := range availableSlots {
		// Apply weekday filter if specified
		if weekday != "" {
			slotWeekday := slot.StartTime.Weekday().String()
			expectedWeekday := convertWeekdayToString(weekday)
			if slotWeekday != expectedWeekday {
				continue
			}
		}

		// Note: room filter is specified but room availability checking not implemented

		timespans = append(timespans, TimespanResponse{
			ID:          slot.ID,
			Name:        generateSlotName(slot.StartTime, slot.EndTime),
			StartTime:   slot.StartTime.Format("15:04"),
			EndTime:     formatEndTime(slot.EndTime),
			Description: fmt.Sprintf("Available slot: %s", slot.StartTime.Format("Monday, Jan 2")),
		})
	}

	common.Respond(w, r, http.StatusOK, timespans, "Available time slots retrieved successfully")
}

// convertWeekdayToString converts weekday abbreviation to full weekday name
func convertWeekdayToString(weekday string) string {
	weekdayMap := map[string]string{
		"MON": "Monday",
		"TUE": "Tuesday",
		"WED": "Wednesday",
		"THU": "Thursday",
		"FRI": "Friday",
		"SAT": "Saturday",
		"SUN": "Sunday",
	}
	if fullName, exists := weekdayMap[weekday]; exists {
		return fullName
	}
	return weekday
}

// generateSlotName creates a descriptive name for the time slot
func generateSlotName(startTime time.Time, endTime *time.Time) string {
	if endTime == nil {
		return fmt.Sprintf("From %s", startTime.Format("15:04"))
	}
	return fmt.Sprintf("%s - %s", startTime.Format("15:04"), endTime.Format("15:04"))
}

// createActivitySchedule adds a new schedule to an activity
func (rs *Resource) createActivitySchedule(w http.ResponseWriter, r *http.Request) {
	// Parse activity ID from URL
	activityID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid activity ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse request
	var req ScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Validate request
	if !activities.IsValidWeekday(req.Weekday) {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid weekday"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Check if activity exists
	_, err = rs.ActivityService.GetGroup(r.Context(), activityID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Create schedule
	schedule := &activities.Schedule{
		ActivityGroupID: activityID,
		Weekday:         req.Weekday,
		TimeframeID:     req.TimeframeID,
	}

	createdSchedule, err := rs.ActivityService.AddSchedule(r.Context(), activityID, schedule)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Convert to response
	response := ScheduleResponse{
		ID:              createdSchedule.ID,
		Weekday:         createdSchedule.Weekday,
		TimeframeID:     createdSchedule.TimeframeID,
		ActivityGroupID: createdSchedule.ActivityGroupID,
		CreatedAt:       createdSchedule.CreatedAt,
		UpdatedAt:       createdSchedule.UpdatedAt,
	}

	common.Respond(w, r, http.StatusCreated, response, "Schedule created successfully")
}

// updateActivitySchedule updates an existing schedule
func (rs *Resource) updateActivitySchedule(w http.ResponseWriter, r *http.Request) {
	// Parse activity ID from URL
	activityID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid activity ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse schedule ID from URL
	scheduleID, err := strconv.ParseInt(chi.URLParam(r, "scheduleId"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid schedule ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse request
	var req ScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Validate request
	if !activities.IsValidWeekday(req.Weekday) {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid weekday"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get existing schedule
	existingSchedule, err := rs.ActivityService.GetSchedule(r.Context(), scheduleID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Check if schedule belongs to the specified activity
	if existingSchedule.ActivityGroupID != activityID {
		if err := render.Render(w, r, ErrorForbidden(errors.New("schedule does not belong to the specified activity"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Update schedule fields
	existingSchedule.Weekday = req.Weekday
	existingSchedule.TimeframeID = req.TimeframeID

	// Update schedule
	updatedSchedule, err := rs.ActivityService.UpdateSchedule(r.Context(), existingSchedule)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Convert to response
	response := ScheduleResponse{
		ID:              updatedSchedule.ID,
		Weekday:         updatedSchedule.Weekday,
		TimeframeID:     updatedSchedule.TimeframeID,
		ActivityGroupID: updatedSchedule.ActivityGroupID,
		CreatedAt:       updatedSchedule.CreatedAt,
		UpdatedAt:       updatedSchedule.UpdatedAt,
	}

	common.Respond(w, r, http.StatusOK, response, "Schedule updated successfully")
}

// deleteActivitySchedule deletes a schedule
func (rs *Resource) deleteActivitySchedule(w http.ResponseWriter, r *http.Request) {
	// Parse activity ID from URL
	activityID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid activity ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse schedule ID from URL
	scheduleID, err := strconv.ParseInt(chi.URLParam(r, "scheduleId"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid schedule ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get schedule to verify ownership
	schedule, err := rs.ActivityService.GetSchedule(r.Context(), scheduleID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Check if schedule belongs to the specified activity
	if schedule.ActivityGroupID != activityID {
		if err := render.Render(w, r, ErrorForbidden(errors.New("schedule does not belong to the specified activity"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Delete schedule
	if err := rs.ActivityService.DeleteSchedule(r.Context(), scheduleID); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Schedule deleted successfully")
}

// SUPERVISOR ASSIGNMENT HANDLERS

// getActivitySupervisors retrieves all supervisors for a specific activity
func (rs *Resource) getActivitySupervisors(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid activity ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Check if activity exists
	activity, err := rs.ActivityService.GetGroup(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get supervisors for the activity
	supervisors, err := rs.ActivityService.GetGroupSupervisors(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Convert to response objects
	responses := make([]SupervisorResponse, 0, len(supervisors))
	for _, supervisor := range supervisors {
		if supervisor == nil {
			continue // Skip nil supervisors to prevent panic
		}

		// Create basic supervisor response
		supervisorResp := SupervisorResponse{
			ID:        supervisor.ID,
			StaffID:   supervisor.StaffID,
			IsPrimary: supervisor.IsPrimary,
		}

		// Add staff details if available
		if supervisor.Staff != nil && supervisor.Staff.Person != nil {
			supervisorResp.FirstName = supervisor.Staff.Person.FirstName
			supervisorResp.LastName = supervisor.Staff.Person.LastName
		}

		responses = append(responses, supervisorResp)
	}

	common.Respond(w, r, http.StatusOK, responses, fmt.Sprintf("Supervisors for activity '%s' retrieved successfully", activity.Name))
}

// getAvailableSupervisors retrieves available supervisors for assignment
func (rs *Resource) getAvailableSupervisors(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Query parameters for filtering
	specialization := r.URL.Query().Get("specialization")

	var supervisors []SupervisorResponse

	if specialization != "" {
		// Get teachers with specific specialization
		teachers, err := rs.UserService.TeacherRepository().FindBySpecialization(ctx, specialization)
		if err != nil {
			log.Printf("Error fetching teachers by specialization: %v", err)
			common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to retrieve teachers")
			return
		}

		// Convert teachers to supervisor responses
		for _, teacher := range teachers {
			// Get the full teacher with staff and person data
			fullTeacher, err := rs.UserService.TeacherRepository().FindWithStaffAndPerson(ctx, teacher.ID)
			if err != nil {
				log.Printf("Error fetching full teacher data for ID %d: %v", teacher.ID, err)
				continue // Skip this teacher if we can't get full data
			}

			if fullTeacher.Staff != nil && fullTeacher.Staff.Person != nil {
				supervisors = append(supervisors, SupervisorResponse{
					ID:        teacher.ID,
					StaffID:   teacher.StaffID,
					FirstName: fullTeacher.Staff.Person.FirstName,
					LastName:  fullTeacher.Staff.Person.LastName,
					IsPrimary: false, // Default to false for available supervisors
				})
			}
		}
	} else {
		// Get all staff members who could potentially be supervisors
		filters := make(map[string]interface{})
		staff, err := rs.UserService.StaffRepository().List(ctx, filters)
		if err != nil {
			log.Printf("Error fetching staff: %v", err)
			common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to retrieve staff")
			return
		}

		// Convert staff to supervisor responses
		for _, staffMember := range staff {
			// Get person data for each staff member
			person, err := rs.UserService.Get(ctx, staffMember.PersonID)
			if err != nil {
				log.Printf("Error fetching person data for staff ID %d: %v", staffMember.ID, err)
				continue // Skip this staff member if we can't get person data
			}

			supervisors = append(supervisors, SupervisorResponse{
				ID:        staffMember.ID,
				StaffID:   staffMember.ID,
				FirstName: person.FirstName,
				LastName:  person.LastName,
				IsPrimary: false, // Default to false for available supervisors
			})
		}
	}

	common.Respond(w, r, http.StatusOK, supervisors, "Available supervisors retrieved successfully")
}

// SupervisorRequest represents a supervisor assignment request
type SupervisorRequest struct {
	StaffID   int64 `json:"staff_id"`
	IsPrimary bool  `json:"is_primary"`
}

// Bind validates the supervisor request
func (req *SupervisorRequest) Bind(r *http.Request) error {
	if req.StaffID <= 0 {
		return errors.New("staff ID is required and must be greater than 0")
	}
	return nil
}

// assignSupervisor assigns a supervisor to an activity
func (rs *Resource) assignSupervisor(w http.ResponseWriter, r *http.Request) {
	// Parse activity ID from URL
	activityID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid activity ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse request
	var req SupervisorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Validate request
	if err := req.Bind(r); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Check if activity exists
	_, err = rs.ActivityService.GetGroup(r.Context(), activityID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Assign supervisor
	supervisor, err := rs.ActivityService.AddSupervisor(r.Context(), activityID, req.StaffID, req.IsPrimary)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Create response
	response := SupervisorResponse{
		ID:        supervisor.ID,
		StaffID:   supervisor.StaffID,
		IsPrimary: supervisor.IsPrimary,
	}

	// Add staff details if available
	if supervisor.Staff != nil && supervisor.Staff.Person != nil {
		response.FirstName = supervisor.Staff.Person.FirstName
		response.LastName = supervisor.Staff.Person.LastName
	}

	common.Respond(w, r, http.StatusCreated, response, "Supervisor assigned successfully")
}

// updateSupervisorRole updates a supervisor's role (primary/non-primary)
func (rs *Resource) updateSupervisorRole(w http.ResponseWriter, r *http.Request) {
	// Parse activity ID from URL
	activityID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid activity ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse supervisor ID from URL
	supervisorID, err := strconv.ParseInt(chi.URLParam(r, "supervisorId"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid supervisor ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse request
	var req SupervisorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get existing supervisor
	supervisor, err := rs.ActivityService.GetSupervisor(r.Context(), supervisorID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Check if supervisor belongs to the specified activity
	if supervisor.GroupID != activityID {
		if err := render.Render(w, r, ErrorForbidden(errors.New("supervisor does not belong to the specified activity"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// If making this supervisor primary, use the service method to handle it properly
	if req.IsPrimary && !supervisor.IsPrimary {
		if err := rs.ActivityService.SetPrimarySupervisor(r.Context(), supervisorID); err != nil {
			if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
				log.Printf("Error rendering error response: %v", err)
			}
			return
		}
	} else if supervisor.IsPrimary != req.IsPrimary {
		// Only update if the primary status is changing
		supervisor.IsPrimary = req.IsPrimary
		if _, err := rs.ActivityService.UpdateSupervisor(r.Context(), supervisor); err != nil {
			if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
				log.Printf("Error rendering error response: %v", err)
			}
			return
		}
	}

	// Get the updated supervisor
	updatedSupervisor, err := rs.ActivityService.GetSupervisor(r.Context(), supervisorID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Create response
	response := SupervisorResponse{
		ID:        updatedSupervisor.ID,
		StaffID:   updatedSupervisor.StaffID,
		IsPrimary: updatedSupervisor.IsPrimary,
	}

	// Add staff details if available
	if updatedSupervisor.Staff != nil && updatedSupervisor.Staff.Person != nil {
		response.FirstName = updatedSupervisor.Staff.Person.FirstName
		response.LastName = updatedSupervisor.Staff.Person.LastName
	}

	common.Respond(w, r, http.StatusOK, response, "Supervisor role updated successfully")
}

// removeSupervisor removes a supervisor from an activity
func (rs *Resource) removeSupervisor(w http.ResponseWriter, r *http.Request) {
	// Parse activity ID from URL
	activityID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid activity ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse supervisor ID from URL
	supervisorID, err := strconv.ParseInt(chi.URLParam(r, "supervisorId"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid supervisor ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get supervisor to verify ownership
	supervisor, err := rs.ActivityService.GetSupervisor(r.Context(), supervisorID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Check if supervisor belongs to the specified activity
	if supervisor.GroupID != activityID {
		if err := render.Render(w, r, ErrorForbidden(errors.New("supervisor does not belong to the specified activity"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Delete supervisor
	if err := rs.ActivityService.DeleteSupervisor(r.Context(), supervisorID); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Supervisor removed successfully")
}
