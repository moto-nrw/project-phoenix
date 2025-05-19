package activities

import (
	"errors"
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

// ActivityResponse represents an activity group API response
type ActivityResponse struct {
	ID              int64              `json:"id"`
	Name            string             `json:"name"`
	MaxParticipants int                `json:"max_participants"`
	IsOpen          bool               `json:"is_open"`
	CategoryID      int64              `json:"category_id"`
	PlannedRoomID   *int64             `json:"planned_room_id,omitempty"`
	Category        *CategoryResponse  `json:"category,omitempty"`
	Schedules       []ScheduleResponse `json:"schedules,omitempty"`
	EnrollmentCount int                `json:"enrollment_count,omitempty"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
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
	}

	// Add category details if available
	if group.Category != nil {
		category := newCategoryResponse(group.Category)
		response.Category = &category
	}

	// Add schedules if available
	if len(group.Schedules) > 0 {
		response.Schedules = make([]ScheduleResponse, 0, len(group.Schedules))
		for _, schedule := range group.Schedules {
			response.Schedules = append(response.Schedules, ScheduleResponse{
				ID:              schedule.ID,
				Weekday:         schedule.Weekday,
				TimeframeID:     schedule.TimeframeID,
				ActivityGroupID: schedule.ActivityGroupID,
				CreatedAt:       schedule.CreatedAt,
				UpdatedAt:       schedule.UpdatedAt,
			})
		}
	}

	return response
}

// listActivities handles listing all activities with optional filtering
func (rs *Resource) listActivities(w http.ResponseWriter, r *http.Request) {
	// Get filter parameters
	categoryIDStr := r.URL.Query().Get("category_id")

	// Create filters map
	filters := make(map[string]interface{})

	// Apply filters
	if categoryIDStr != "" {
		categoryID, err := strconv.ParseInt(categoryIDStr, 10, 64)
		if err == nil {
			filters["category_id"] = categoryID
		}
	}

	// Get activities
	groups, err := rs.ActivityService.ListGroups(r.Context(), filters)
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

	// Get activity group with details
	group, _, schedules, err := rs.ActivityService.GetGroupWithDetails(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get enrollment count
	enrolledStudents, err := rs.ActivityService.GetEnrolledStudents(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Prepare response
	group.Schedules = schedules
	response := newActivityResponse(group, len(enrolledStudents))

	// Add supervisor details if needed
	// For brevity, we're not including detailed supervisor info in this implementation

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

	// Get the created group with details
	createdGroup, _, createdSchedules, err := rs.ActivityService.GetGroupWithDetails(r.Context(), createdGroup.ID)
	if err != nil {
		// Still return the basic group info if we can't get detailed info
		createdGroup.Schedules = schedules
		response := newActivityResponse(createdGroup, 0)
		common.Respond(w, r, http.StatusCreated, response, "Activity created successfully")
		return
	}

	// Add schedules to group
	createdGroup.Schedules = createdSchedules

	response := newActivityResponse(createdGroup, 0)
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
	updatedGroup, _, updatedSchedules, err := rs.ActivityService.GetGroupWithDetails(r.Context(), updatedGroup.ID)
	if err != nil {
		response := newActivityResponse(updatedGroup, 0)
		common.Respond(w, r, http.StatusOK, response, "Activity updated successfully")
		return
	}

	// Add schedules to group
	updatedGroup.Schedules = updatedSchedules

	// Get enrollment count
	enrolledStudents, err := rs.ActivityService.GetEnrolledStudents(r.Context(), id)
	if err != nil {
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
		// Check if person data is loaded
		if student.Person != nil {
			responses = append(responses, StudentResponse{
				ID:        student.ID,
				FirstName: student.Person.FirstName,
				LastName:  student.Person.LastName,
			})
		} else {
			// Fallback if person data isn't loaded
			responses = append(responses, StudentResponse{
				ID: student.ID,
			})
		}
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
