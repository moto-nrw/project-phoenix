package activities

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// =============================================================================
// ACTIVITY CRUD HANDLERS
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
