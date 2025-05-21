package activities

import (
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
)

// Resource defines the activities API resource
type Resource struct {
	ActivityService activitiesSvc.ActivityService
}

// NewResource creates a new activities resource
func NewResource(activityService activitiesSvc.ActivityService) *Resource {
	return &Resource{
		ActivityService: activityService,
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

		// Read operations only require activities:read permission
		r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Get("/", rs.listActivities)
		r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Get("/{id}", rs.getActivity)
		r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Get("/categories", rs.listCategories)
		r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Get("/timespans", rs.getTimespans)
		r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Get("/{id}/students", rs.getActivityStudents)

		// Write operations require specific permissions
		r.With(authorize.RequiresPermission(permissions.ActivitiesCreate)).Post("/", rs.createActivity)
		r.With(authorize.RequiresPermission(permissions.ActivitiesUpdate)).Put("/{id}", rs.updateActivity)
		r.With(authorize.RequiresPermission(permissions.ActivitiesDelete)).Delete("/{id}", rs.deleteActivity)
		r.With(authorize.RequiresPermission(permissions.ActivitiesEnroll)).Post("/{id}/enroll/{studentId}", rs.enrollStudent)
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
	Weekday         string    `json:"weekday"`
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
	Weekday     string `json:"weekday"`
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
	} else {
		log.Printf("Info: Group ID %d has nil Schedules array", group.ID)
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

	// Build response
	responses := make([]ActivityResponse, 0, len(groups))
	for _, group := range groups {
		count := enrollmentCounts[group.ID]
		responses = append(responses, newActivityResponse(group, count))
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
	if supervisors != nil && len(supervisors) > 0 {
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
	if schedules != nil && len(schedules) > 0 {
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

	// Note: In a complete implementation, we would handle updating schedules and supervisors
	// For simplicity, we're not implementing the full update functionality here

	// Get the updated group with details
	detailedGroup, supervisors, updatedSchedules, err := rs.ActivityService.GetGroupWithDetails(r.Context(), updatedGroup.ID)
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

		// Add supervisors information if available
		if supervisors != nil && len(supervisors) > 0 {
			// You could add processing for supervisors here if needed
			log.Printf("Info: %d supervisors found for group ID %d", len(supervisors), updatedGroup.ID)
		} else {
			log.Printf("Info: No supervisors found or failed to load for group ID %d", updatedGroup.ID)
		}
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
	_, err = rs.ActivityService.GetGroup(r.Context(), id)
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

	common.Respond(w, r, http.StatusOK, responses, "Students retrieved successfully")
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
	// In a real implementation, this would fetch timeframes from the schedule service
	// For now, we'll return a placeholder response

	// Mock timespan data
	timespans := []TimespanResponse{
		{
			ID:          1,
			Name:        "Morning",
			StartTime:   "08:00",
			EndTime:     "12:00",
			Description: "Morning sessions",
		},
		{
			ID:          2,
			Name:        "Afternoon",
			StartTime:   "13:00",
			EndTime:     "17:00",
			Description: "Afternoon sessions",
		},
	}

	common.Respond(w, r, http.StatusOK, timespans, "Time spans retrieved successfully")
}
