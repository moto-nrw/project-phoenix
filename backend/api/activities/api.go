package activities

import (
	"context"
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
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	activitiesSvc "github.com/moto-nrw/project-phoenix/services/activities"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	usercontextSvc "github.com/moto-nrw/project-phoenix/services/usercontext"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

const (
	routeScheduleByID           = "/{id}/schedules/{scheduleId}"
	msgActivityCreatedSuccess   = "Activity created successfully"
	msgActivityUpdatedSuccess   = "Activity updated successfully"
)

// Resource defines the activities API resource
type Resource struct {
	ActivityService    activitiesSvc.ActivityService
	ScheduleService    scheduleSvc.Service
	UserService        usersSvc.PersonService
	UserContextService usercontextSvc.UserContextService
}

// NewResource creates a new activities resource
func NewResource(activityService activitiesSvc.ActivityService, scheduleService scheduleSvc.Service, userService usersSvc.PersonService, userContextService usercontextSvc.UserContextService) *Resource {
	return &Resource{
		ActivityService:    activityService,
		ScheduleService:    scheduleService,
		UserService:        userService,
		UserContextService: userContextService,
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

		// Basic Activity Group operations (Read) - All authenticated users can read
		r.Get("/", rs.listActivities)
		r.Get("/{id}", rs.getActivity)
		r.Get("/categories", rs.listCategories)
		r.Get("/timespans", rs.getTimespans)

		// Basic Activity Group operations (Write) - All authenticated users can create/update/delete
		r.Post("/", rs.createActivity)
		r.Post("/quick-create", rs.quickCreateActivity)
		r.Put("/{id}", rs.updateActivity)
		r.Delete("/{id}", rs.deleteActivity)

		// Schedule Management - All authenticated users can manage schedules
		r.Get("/{id}/schedules", rs.getActivitySchedules)
		r.Get(routeScheduleByID, rs.getActivitySchedule)
		r.Get("/schedules/available", rs.getAvailableTimeSlots)
		r.Post("/{id}/schedules", rs.createActivitySchedule)
		r.Put(routeScheduleByID, rs.updateActivitySchedule)
		r.Delete(routeScheduleByID, rs.deleteActivitySchedule)

		// Supervisor Assignment - All authenticated users can manage supervisors
		r.Get("/{id}/supervisors", rs.getActivitySupervisors)
		r.Get("/supervisors/available", rs.getAvailableSupervisors)
		r.Post("/{id}/supervisors", rs.assignSupervisor)
		r.Put("/{id}/supervisors/{supervisorId}", rs.updateSupervisorRole)
		r.Delete("/{id}/supervisors/{supervisorId}", rs.removeSupervisor)

		// Student Enrollment - All authenticated users can manage enrollments
		r.Get("/{id}/students", rs.getActivityStudents)
		r.Get("/students/{studentId}", rs.getStudentEnrollments)
		r.Get("/students/{studentId}/available", rs.getAvailableActivities)
		r.Post("/{id}/students/{studentId}", rs.enrollStudent)
		r.Delete("/{id}/students/{studentId}", rs.unenrollStudent)
		r.Put("/{id}/students", rs.updateGroupEnrollments)
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

// QuickActivityRequest represents a simplified activity creation request for mobile devices
type QuickActivityRequest struct {
	Name            string `json:"name"`
	CategoryID      int64  `json:"category_id"`
	RoomID          *int64 `json:"room_id,omitempty"`
	MaxParticipants int    `json:"max_participants"`
}

// QuickActivityResponse represents the response after creating an activity via quick-create
type QuickActivityResponse struct {
	ActivityID     int64     `json:"activity_id"`
	Name           string    `json:"name"`
	CategoryName   string    `json:"category_name"`
	RoomName       string    `json:"room_name,omitempty"`
	SupervisorName string    `json:"supervisor_name"`
	Status         string    `json:"status"`
	Message        string    `json:"message"`
	CreatedAt      time.Time `json:"created_at"`
}

// ScheduleRequest represents a schedule in activity creation/update request
type ScheduleRequest struct {
	Weekday     int    `json:"weekday"`
	TimeframeID *int64 `json:"timeframe_id,omitempty"`
}

// Bind validates the activity request
func (req *ActivityRequest) Bind(_ *http.Request) error {
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

// Bind validates the quick activity request
func (req *QuickActivityRequest) Bind(_ *http.Request) error {
	if req.Name == "" {
		return errors.New("activity name is required")
	}
	if req.CategoryID <= 0 {
		return errors.New("category ID is required")
	}
	if req.MaxParticipants <= 0 {
		return errors.New("max participants must be greater than zero")
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
		scheduleResponses := make([]ScheduleResponse, 0, len(group.Schedules))
		for _, schedule := range group.Schedules {
			if schedule == nil {
				log.Printf("Warning: Nil schedule encountered in group ID %d", group.ID)
				continue
			}
			scheduleResponses = append(scheduleResponses, newScheduleResponse(schedule))
		}
		if len(scheduleResponses) > 0 {
			response.Schedules = scheduleResponses
		}
	}

	return response
}

// =============================================================================
// HELPER METHODS - Reduce code duplication for common parsing/validation
// =============================================================================

// parseAndGetActivity parses activity ID from URL and returns the activity if it exists.
// Returns nil and false if parsing fails or activity doesn't exist (error already rendered).
func (rs *Resource) parseAndGetActivity(w http.ResponseWriter, r *http.Request) (*activities.Group, bool) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(common.MsgInvalidActivityID)))
		return nil, false
	}

	activity, err := rs.ActivityService.GetGroup(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return nil, false
	}

	return activity, true
}

// parseStudentID parses student ID from URL param "studentId".
// Returns 0 and false if parsing fails (error already rendered).
func (rs *Resource) parseStudentID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	studentID, err := common.ParseIDParam(r, "studentId")
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(common.MsgInvalidStudentID)))
		return 0, false
	}
	return studentID, true
}


// parseSupervisorID parses supervisor ID from URL param "supervisorId".
// Returns 0 and false if parsing fails (error already rendered).
func (rs *Resource) parseSupervisorID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	supervisorID, err := common.ParseIDParam(r, "supervisorId")
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid supervisor ID")))
		return 0, false
	}
	return supervisorID, true
}

// =============================================================================
// RESPONSE CONVERSION HELPERS - Reduce duplication in response creation
// =============================================================================


// newSupervisorResponse converts a supervisor model to a response object with staff details.
func newSupervisorResponse(supervisor *activities.SupervisorPlanned) SupervisorResponse {
	if supervisor == nil {
		return SupervisorResponse{}
	}
	resp := SupervisorResponse{
		ID:        supervisor.ID,
		StaffID:   supervisor.StaffID,
		IsPrimary: supervisor.IsPrimary,
	}
	if supervisor.Staff != nil && supervisor.Staff.Person != nil {
		resp.FirstName = supervisor.Staff.Person.FirstName
		resp.LastName = supervisor.Staff.Person.LastName
	}
	return resp
}

// =============================================================================
// OWNERSHIP CHECK HELPERS - Verify resources belong to activity
// =============================================================================

// checkSupervisorOwnership verifies the supervisor belongs to the specified activity.
// Returns false and renders error if ownership check fails.
func (rs *Resource) checkSupervisorOwnership(w http.ResponseWriter, r *http.Request, supervisor *activities.SupervisorPlanned, activityID int64) bool {
	if supervisor.GroupID != activityID {
		common.RenderError(w, r, ErrorForbidden(errors.New("supervisor does not belong to the specified activity")))
		return false
	}
	return true
}

// =============================================================================
// DATA RETRIEVAL HELPERS
// =============================================================================

// getEnrollmentCount returns the number of enrolled students for an activity.
func (rs *Resource) getEnrollmentCount(ctx context.Context, activityID int64) int {
	students, err := rs.ActivityService.GetEnrolledStudents(ctx, activityID)
	if err != nil || students == nil {
		return 0
	}
	return len(students)
}

// fetchActivityData retrieves activity group with details, using fallback if needed.
func (rs *Resource) fetchActivityData(ctx context.Context, id int64) (*activities.Group, []*activities.SupervisorPlanned, []*activities.Schedule, error) {
	group, supervisors, schedules, detailsErr := rs.ActivityService.GetGroupWithDetails(ctx, id)
	if detailsErr != nil {
		log.Printf("Warning: Error getting detailed group info: %v", detailsErr)
		return rs.fetchActivityDataFallback(ctx, id)
	}
	return group, supervisors, schedules, nil
}

// fetchActivityDataFallback retrieves activity data piece by piece when GetGroupWithDetails fails.
func (rs *Resource) fetchActivityDataFallback(ctx context.Context, id int64) (*activities.Group, []*activities.SupervisorPlanned, []*activities.Schedule, error) {
	group, err := rs.ActivityService.GetGroup(ctx, id)
	if err != nil {
		return nil, nil, nil, err
	}

	schedules, scheduleErr := rs.ActivityService.GetGroupSchedules(ctx, id)
	if scheduleErr != nil {
		log.Printf("Warning: Error getting schedules: %v", scheduleErr)
		schedules = []*activities.Schedule{}
	}

	supervisors, _ := rs.ActivityService.GetGroupSupervisors(ctx, id)
	return group, supervisors, schedules, nil
}

// ensureCategoryLoaded loads the category if it's missing from the group.
func (rs *Resource) ensureCategoryLoaded(ctx context.Context, group *activities.Group) {
	if group.Category == nil && group.CategoryID > 0 {
		category, catErr := rs.ActivityService.GetCategory(ctx, group.CategoryID)
		if catErr != nil {
			log.Printf("Warning: Error getting category for ID %d: %v", group.CategoryID, catErr)
		} else if category != nil {
			group.Category = category
		}
	}
}

// buildBaseActivityResponse creates the base activity response structure.
func buildBaseActivityResponse(group *activities.Group, enrollmentCount int) ActivityResponse {
	return ActivityResponse{
		ID:              group.ID,
		Name:            group.Name,
		MaxParticipants: group.MaxParticipants,
		IsOpen:          group.IsOpen,
		CategoryID:      group.CategoryID,
		PlannedRoomID:   group.PlannedRoomID,
		EnrollmentCount: enrollmentCount,
		CreatedAt:       group.CreatedAt,
		UpdatedAt:       group.UpdatedAt,
		Schedules:       []ScheduleResponse{},
	}
}

// addCategoryToResponse adds category details to the response if available.
func addCategoryToResponse(response *ActivityResponse, group *activities.Group) {
	if group.Category != nil {
		category := newCategoryResponse(group.Category)
		response.Category = &category
	}
}

// addSupervisorsToResponse adds supervisor details to the response.
func addSupervisorsToResponse(response *ActivityResponse, supervisors []*activities.SupervisorPlanned) {
	if len(supervisors) == 0 {
		return
	}

	supervisorIDs := make([]int64, 0, len(supervisors))
	supervisorDetails := make([]SupervisorResponse, 0, len(supervisors))

	for _, supervisor := range supervisors {
		if supervisor != nil {
			supervisorIDs = append(supervisorIDs, supervisor.StaffID)
			if supervisor.IsPrimary {
				response.SupervisorID = &supervisor.StaffID
			}
			supervisorDetails = append(supervisorDetails, newSupervisorResponse(supervisor))
		}
	}

	if len(supervisorIDs) > 0 {
		response.SupervisorIDs = supervisorIDs
		response.Supervisors = supervisorDetails
	}
}

// addSchedulesToResponse adds schedule details to the response.
func addSchedulesToResponse(response *ActivityResponse, schedules []*activities.Schedule) {
	if len(schedules) == 0 {
		return
	}

	responseSchedules := make([]ScheduleResponse, 0, len(schedules))
	for _, schedule := range schedules {
		if schedule != nil {
			responseSchedules = append(responseSchedules, newScheduleResponse(schedule))
		}
	}

	if len(responseSchedules) > 0 {
		response.Schedules = responseSchedules
	}
}

// updateGroupFields updates the basic fields of an activity group from a request.
func updateGroupFields(group *activities.Group, req *ActivityRequest) {
	group.Name = req.Name
	group.MaxParticipants = req.MaxParticipants
	group.IsOpen = req.IsOpen
	group.CategoryID = req.CategoryID
	group.PlannedRoomID = req.PlannedRoomID
}

// updateSupervisorsWithLogging updates group supervisors and logs any errors without failing.
func (rs *Resource) updateSupervisorsWithLogging(ctx context.Context, groupID int64, supervisorIDs []int64) {
	err := rs.ActivityService.UpdateGroupSupervisors(ctx, groupID, supervisorIDs)
	if err != nil {
		log.Printf("Warning: Failed to update supervisors for activity %d: %v", groupID, err)
	}
}

// replaceGroupSchedules removes existing schedules and adds new ones.
func (rs *Resource) replaceGroupSchedules(ctx context.Context, groupID int64, newSchedules []ScheduleRequest) {
	// Delete existing schedules
	existingSchedules, err := rs.ActivityService.GetGroupSchedules(ctx, groupID)
	if err != nil {
		log.Printf("Warning: Failed to get existing schedules: %v", err)
	} else {
		for _, schedule := range existingSchedules {
			err = rs.ActivityService.DeleteSchedule(ctx, schedule.ID)
			if err != nil {
				log.Printf("Warning: Failed to delete schedule with ID %d: %v", schedule.ID, err)
			}
		}
	}

	// Add new schedules
	for _, scheduleReq := range newSchedules {
		schedule := &activities.Schedule{
			Weekday:     scheduleReq.Weekday,
			TimeframeID: scheduleReq.TimeframeID,
		}
		_, err = rs.ActivityService.AddSchedule(ctx, groupID, schedule)
		if err != nil {
			log.Printf("Warning: Failed to add schedule (weekday=%d, timeframe=%v): %v", scheduleReq.Weekday, scheduleReq.TimeframeID, err)
		}
	}
}

// fetchUpdatedGroupData retrieves the updated group with details and handles nil checks.
func (rs *Resource) fetchUpdatedGroupData(ctx context.Context, updatedGroup *activities.Group) (*activities.Group, error) {
	detailedGroup, _, updatedSchedules, err := rs.ActivityService.GetGroupWithDetails(ctx, updatedGroup.ID)
	if err != nil {
		log.Printf("Failed to get detailed group info after update: %v", err)
		if updatedGroup != nil {
			updatedGroup.Schedules = []*activities.Schedule{}
		}
		return updatedGroup, err
	}

	// Handle schedule assignment with nil checks
	if detailedGroup != nil {
		if updatedSchedules != nil {
			updatedGroup.Schedules = updatedSchedules
		} else {
			log.Printf("Warning: updatedSchedules is nil despite no error from GetGroupWithDetails")
			updatedGroup.Schedules = []*activities.Schedule{}
		}
	} else {
		log.Printf("Warning: detailedGroup is nil despite no error from GetGroupWithDetails")
		if updatedGroup != nil {
			updatedGroup.Schedules = []*activities.Schedule{}
		}
	}

	return updatedGroup, nil
}

// buildUpdateResponse creates the final response for an activity update.
func (rs *Resource) buildUpdateResponse(ctx context.Context, group *activities.Group, activityID int64) (ActivityResponse, error) {
	if group == nil {
		log.Printf("Error: updatedGroup is nil before creating response. Returning empty response.")
		return ActivityResponse{}, errors.New("group is nil")
	}

	enrolledStudents, err := rs.ActivityService.GetEnrolledStudents(ctx, activityID)
	enrollmentCount := 0
	if err != nil {
		log.Printf("Failed to get enrolled students: %v", err)
	} else if enrolledStudents != nil {
		enrollmentCount = len(enrolledStudents)
	}

	return newActivityResponse(group, enrollmentCount), nil
}

// fetchSupervisorsBySpecialization retrieves supervisors filtered by specialization.
func (rs *Resource) fetchSupervisorsBySpecialization(ctx context.Context, specialization string) ([]SupervisorResponse, error) {
	teachers, err := rs.UserService.TeacherRepository().FindBySpecialization(ctx, specialization)
	if err != nil {
		return nil, err
	}

	supervisors := make([]SupervisorResponse, 0, len(teachers))
	for _, teacher := range teachers {
		fullTeacher, err := rs.UserService.TeacherRepository().FindWithStaffAndPerson(ctx, teacher.ID)
		if err != nil {
			log.Printf("Error fetching full teacher data for ID %d: %v", teacher.ID, err)
			continue
		}

		if fullTeacher.Staff != nil && fullTeacher.Staff.Person != nil {
			supervisors = append(supervisors, SupervisorResponse{
				ID:        teacher.ID,
				StaffID:   teacher.StaffID,
				FirstName: fullTeacher.Staff.Person.FirstName,
				LastName:  fullTeacher.Staff.Person.LastName,
				IsPrimary: false,
			})
		}
	}

	return supervisors, nil
}

// fetchAllSupervisors retrieves all staff members as potential supervisors.
func (rs *Resource) fetchAllSupervisors(ctx context.Context) ([]SupervisorResponse, error) {
	filters := make(map[string]interface{})
	staff, err := rs.UserService.StaffRepository().List(ctx, filters)
	if err != nil {
		return nil, err
	}

	supervisors := make([]SupervisorResponse, 0, len(staff))
	for _, staffMember := range staff {
		person, err := rs.UserService.Get(ctx, staffMember.PersonID)
		if err != nil {
			log.Printf("Error fetching person data for staff ID %d: %v", staffMember.ID, err)
			continue
		}

		supervisors = append(supervisors, SupervisorResponse{
			ID:        staffMember.ID,
			StaffID:   staffMember.ID,
			FirstName: person.FirstName,
			LastName:  person.LastName,
			IsPrimary: false,
		})
	}

	return supervisors, nil
}

// =============================================================================
// ACTIVITY HANDLERS
// =============================================================================

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
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Get enrollment counts
	_, enrollmentCounts, err := rs.ActivityService.GetGroupsWithEnrollmentCounts(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
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
				supervisorResponses = append(supervisorResponses, newSupervisorResponse(supervisor))
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
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(common.MsgInvalidActivityID)))
		return
	}

	// Fetch activity data with fallback handling
	group, supervisors, schedules, err := rs.fetchActivityData(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Validate group exists
	if group == nil {
		log.Printf("Error: Group is nil after GetGroup call for ID %d", id)
		common.RenderError(w, r, ErrorInternalServer(errors.New("activity not found or could not be retrieved")))
		return
	}

	// Ensure category is loaded
	rs.ensureCategoryLoaded(r.Context(), group)

	// Build response
	enrollmentCount := rs.getEnrollmentCount(r.Context(), id)
	response := buildBaseActivityResponse(group, enrollmentCount)
	addCategoryToResponse(&response, group)
	addSupervisorsToResponse(&response, supervisors)
	addSchedulesToResponse(&response, schedules)

	common.Respond(w, r, http.StatusOK, response, "Activity retrieved successfully")
}

// createActivity handles creating a new activity
func (rs *Resource) createActivity(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &ActivityRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
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
		common.RenderError(w, r, ErrorRenderer(err))
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
		}, msgActivityCreatedSuccess)
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

	common.Respond(w, r, http.StatusCreated, response, msgActivityCreatedSuccess)
}

// quickCreateActivity handles creating a new activity with mobile-optimized interface
func (rs *Resource) quickCreateActivity(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &QuickActivityRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Verify user is authenticated (JWT middleware already ensures this)
	// We just need to make sure we can get staff info if available

	// Create activity group with smart defaults
	group := &activities.Group{
		Name:            req.Name,
		MaxParticipants: req.MaxParticipants,
		IsOpen:          true, // Default to true for quick-create
		CategoryID:      req.CategoryID,
		PlannedRoomID:   req.RoomID,
	}

	// Try to get staff info to auto-assign as supervisor
	var supervisorIDs []int64
	staff, err := rs.UserContextService.GetCurrentStaff(r.Context())
	if err == nil && staff != nil {
		// If user is staff, auto-assign as primary supervisor
		supervisorIDs = []int64{staff.ID}
	}
	// If user is not staff, create activity without supervisor (can be assigned later)

	// Create the activity group with auto-assigned teacher supervision
	createdGroup, err := rs.ActivityService.CreateGroup(r.Context(), group, supervisorIDs, nil)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build enhanced response with additional context
	response := QuickActivityResponse{
		ActivityID: createdGroup.ID,
		Name:       createdGroup.Name,
		Status:     "created",
		Message:    "Activity created successfully and ready for RFID device selection",
		CreatedAt:  createdGroup.CreatedAt,
	}

	// Get category name for response
	if category, err := rs.ActivityService.GetCategory(r.Context(), req.CategoryID); err == nil && category != nil {
		response.CategoryName = category.Name
	}

	// Get room name if room was specified
	if req.RoomID != nil {
		// Assuming there's a room service available via UserService or similar
		// For now, we'll leave it empty since room service access wasn't specified
		response.RoomName = ""
	}

	// Add supervisor name to response if available
	if staff != nil && staff.Person != nil {
		response.SupervisorName = fmt.Sprintf("%s %s", staff.Person.FirstName, staff.Person.LastName)
	} else {
		// Try to get person info for non-staff users
		person, _ := rs.UserContextService.GetCurrentPerson(r.Context())
		if person != nil {
			response.SupervisorName = fmt.Sprintf("%s %s", person.FirstName, person.LastName)
		}
	}

	common.Respond(w, r, http.StatusCreated, response, msgActivityCreatedSuccess)
}

// updateActivity handles updating an activity
func (rs *Resource) updateActivity(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(common.MsgInvalidActivityID)))
		return
	}

	// Parse request
	req := &ActivityRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Get and update existing group
	existingGroup, err := rs.ActivityService.GetGroup(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	updateGroupFields(existingGroup, req)

	updatedGroup, err := rs.ActivityService.UpdateGroup(r.Context(), existingGroup)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Update supervisors and schedules
	rs.updateSupervisorsWithLogging(r.Context(), updatedGroup.ID, req.SupervisorIDs)
	rs.replaceGroupSchedules(r.Context(), updatedGroup.ID, req.Schedules)

	// Fetch updated group data with details
	finalGroup, err := rs.fetchUpdatedGroupData(r.Context(), updatedGroup)
	if err != nil {
		response := newActivityResponse(finalGroup, 0)
		common.Respond(w, r, http.StatusOK, response, msgActivityUpdatedSuccess)
		return
	}

	// Build and return response
	response, err := rs.buildUpdateResponse(r.Context(), finalGroup, id)
	if err != nil {
		common.Respond(w, r, http.StatusOK, ActivityResponse{}, "Activity updated but details could not be retrieved")
		return
	}

	common.Respond(w, r, http.StatusOK, response, msgActivityUpdatedSuccess)
}

// deleteActivity handles deleting an activity
func (rs *Resource) deleteActivity(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(common.MsgInvalidActivityID)))
		return
	}

	// Delete the activity
	if err := rs.ActivityService.DeleteGroup(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Activity deleted successfully")
}

// listCategories handles listing all activity categories
func (rs *Resource) listCategories(w http.ResponseWriter, r *http.Request) {
	// Get categories
	categories, err := rs.ActivityService.ListCategories(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
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
	activity, ok := rs.parseAndGetActivity(w, r)
	if !ok {
		return
	}

	// Get enrolled students
	students, err := rs.ActivityService.GetEnrolledStudents(r.Context(), activity.ID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
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
	studentID, ok := rs.parseStudentID(w, r)
	if !ok {
		return
	}

	// Get activities that student is enrolled in
	enrolledGroups, err := rs.ActivityService.GetStudentEnrollments(r.Context(), studentID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build activity responses
	responses := make([]ActivityResponse, 0, len(enrolledGroups))
	for _, group := range enrolledGroups {
		if group == nil {
			continue // Skip nil groups to prevent panic
		}
		responses = append(responses, newActivityResponse(group, rs.getEnrollmentCount(r.Context(), group.ID)))
	}

	common.Respond(w, r, http.StatusOK, responses, fmt.Sprintf("Activities for student ID %d retrieved successfully", studentID))
}

// getAvailableActivities handles getting activities available for a student to enroll in
func (rs *Resource) getAvailableActivities(w http.ResponseWriter, r *http.Request) {
	studentID, ok := rs.parseStudentID(w, r)
	if !ok {
		return
	}

	// Get available activities for student
	availableGroups, err := rs.ActivityService.GetAvailableGroups(r.Context(), studentID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build activity responses
	responses := make([]ActivityResponse, 0, len(availableGroups))
	for _, group := range availableGroups {
		if group == nil {
			continue // Skip nil groups to prevent panic
		}
		responses = append(responses, newActivityResponse(group, rs.getEnrollmentCount(r.Context(), group.ID)))
	}

	common.Respond(w, r, http.StatusOK, responses, fmt.Sprintf("Available activities for student ID %d retrieved successfully", studentID))
}

// We already have the enrollStudent method, no need to modify it since it follows the standard

// unenrollStudent handles removing a student from an activity
func (rs *Resource) unenrollStudent(w http.ResponseWriter, r *http.Request) {
	activity, ok := rs.parseAndGetActivity(w, r)
	if !ok {
		return
	}

	studentID, ok := rs.parseStudentID(w, r)
	if !ok {
		return
	}

	// Unenroll student
	if err := rs.ActivityService.UnenrollStudent(r.Context(), activity.ID, studentID); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, fmt.Sprintf("Student unenrolled from activity '%s' successfully", activity.Name))
}

// BatchEnrollmentRequest represents a request for updating enrollments in batch
type BatchEnrollmentRequest struct {
	StudentIDs []int64 `json:"student_ids"`
}

// Bind validates the batch enrollment request
func (req *BatchEnrollmentRequest) Bind(_ *http.Request) error {
	if req.StudentIDs == nil {
		return errors.New("student IDs are required")
	}
	return nil
}

// updateGroupEnrollments handles updating student enrollments in batch
func (rs *Resource) updateGroupEnrollments(w http.ResponseWriter, r *http.Request) {
	activity, ok := rs.parseAndGetActivity(w, r)
	if !ok {
		return
	}

	// Parse request
	var req BatchEnrollmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Validate request
	if err := req.Bind(r); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Update group enrollments
	if err := rs.ActivityService.UpdateGroupEnrollments(r.Context(), activity.ID, req.StudentIDs); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Create a simplified response
	response := map[string]interface{}{
		"activity_id":       activity.ID,
		"activity_name":     activity.Name,
		"enrollment_count":  rs.getEnrollmentCount(r.Context(), activity.ID),
		"max_participants":  activity.MaxParticipants,
		"students_enrolled": req.StudentIDs,
	}

	common.Respond(w, r, http.StatusOK, response, fmt.Sprintf("Enrollments for activity '%s' updated successfully", activity.Name))
}

// enrollStudent handles enrolling a student in an activity
func (rs *Resource) enrollStudent(w http.ResponseWriter, r *http.Request) {
	activity, ok := rs.parseAndGetActivity(w, r)
	if !ok {
		return
	}

	studentID, ok := rs.parseStudentID(w, r)
	if !ok {
		return
	}

	// Enroll student
	if err := rs.ActivityService.EnrollStudent(r.Context(), activity.ID, studentID); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Student enrolled successfully")
}

