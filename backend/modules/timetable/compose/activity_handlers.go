package compose

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/models/activities"
	activitiesSvc "github.com/moto-nrw/project-phoenix/services/activities"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// fetchActivityData retrieves activity group with details, using fallback if needed.
func (rs *Resource) fetchActivityData(ctx context.Context, id int64) (*activities.Group, []*activities.SupervisorPlanned, []*activities.Schedule, error) {
	group, supervisors, schedules, detailsErr := rs.ActivityService.GetGroupWithDetails(ctx, id)
	if detailsErr != nil {
		slog.Default().WarnContext(ctx, "Error getting detailed group info",
			slog.String("error", detailsErr.Error()),
			slog.Int64("group_id", id),
		)
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
		slog.Default().WarnContext(ctx, "Error getting schedules",
			slog.String("error", scheduleErr.Error()),
			slog.Int64("group_id", id),
		)
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
			slog.Default().WarnContext(ctx, "Error getting category",
				slog.Int64("category_id", group.CategoryID),
				slog.String("error", catErr.Error()),
			)
		} else if category != nil {
			group.Category = category
		}
	}
}

// buildBaseActivityResponse creates the base activity response structure.
func buildBaseActivityResponse(group *activities.Group, enrollmentCount int) ActivityResponse {
	response := ActivityResponse{
		ID:              group.ID,
		Name:            group.Name,
		MaxParticipants: group.ParticipantLimit(),
		IsOpen:          group.IsOpen,
		CategoryID:      group.CategoryID,
		PlannedRoomID:   group.PlannedRoomID,
		CreatedBy:       group.CreatedBy,
		EnrollmentCount: enrollmentCount,
		CreatedAt:       group.CreatedAt,
		UpdatedAt:       group.UpdatedAt,
		Schedules:       []ScheduleResponse{},
	}

	// Add creator name if available
	if group.CreatedByStaff != nil && group.CreatedByStaff.Person != nil {
		response.CreatedByName = group.CreatedByStaff.Person.FirstName + " " + group.CreatedByStaff.Person.LastName
	}

	return response
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

// fetchUpdatedGroupData retrieves the updated group with details and handles nil checks.
func (rs *Resource) fetchUpdatedGroupData(ctx context.Context, updatedGroup *activities.Group) (*activities.Group, error) {
	detailedGroup, _, updatedSchedules, err := rs.ActivityService.GetGroupWithDetails(ctx, updatedGroup.ID)
	if err != nil {
		slog.Default().ErrorContext(ctx, "Failed to get detailed group info after update", slog.String("error", err.Error()))
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
			slog.Default().Warn("updatedSchedules is nil despite no error from GetGroupWithDetails")
			updatedGroup.Schedules = []*activities.Schedule{}
		}
	} else {
		slog.Default().Warn("detailedGroup is nil despite no error from GetGroupWithDetails")
		if updatedGroup != nil {
			updatedGroup.Schedules = []*activities.Schedule{}
		}
	}

	return updatedGroup, nil
}

// buildUpdateResponse creates the final response for an activity update.
func (rs *Resource) buildUpdateResponse(ctx context.Context, group *activities.Group, activityID int64) (ActivityResponse, error) {
	if group == nil {
		slog.Default().Error("updatedGroup is nil before creating response")
		return ActivityResponse{}, errors.New("group is nil")
	}

	enrolledStudents, err := rs.ActivityService.GetEnrolledStudents(ctx, activityID)
	enrollmentCount := 0
	if err != nil {
		slog.Default().ErrorContext(ctx, "Failed to get enrolled students", slog.String("error", err.Error()))
	} else if enrolledStudents != nil {
		enrollmentCount = len(enrolledStudents)
	}

	return newActivityResponse(group, enrollmentCount), nil
}

// =============================================================================
// ACTIVITY HANDLERS
// =============================================================================

// listActivities handles listing all activities with optional filtering
func (rs *Resource) listActivities(w http.ResponseWriter, r *http.Request) {
	// Get filter parameters
	categoryIDStr := r.URL.Query().Get("category_id")

	query := &activities.GroupListQuery{}

	// Apply filters
	if categoryIDStr != "" {
		categoryID, err := strconv.ParseInt(categoryIDStr, 10, 64)
		if err == nil {
			query.CategoryID = &categoryID
		}
	}

	// Hide auto-provisioned system activities (Schulhof Freispiel, WC)
	// unless the caller explicitly opts in — they are IoT infrastructure,
	// not joinable activities (issue #923).
	if r.URL.Query().Get("include_system") != "true" {
		isSystem := false
		query.IsSystem = &isSystem
	}

	// Get activities
	groups, err := rs.ActivityService.ListGroups(r.Context(), query)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Get enrollment counts
	_, enrollmentCounts, err := rs.ActivityService.GetGroupsWithEnrollmentCounts(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Batch fetch supervisors for all groups (avoids N+1 query problem)
	groupIDs := make([]int64, len(groups))
	for i, group := range groups {
		groupIDs[i] = group.ID
	}
	supervisorMap, err := rs.ActivityService.GetSupervisorsForGroups(r.Context(), groupIDs)
	if err != nil {
		slog.Default().ErrorContext(r.Context(), "Error loading supervisors", slog.String("error", err.Error()))
		supervisorMap = make(map[int64][]*activities.SupervisorPlanned)
	}

	// Build response with supervisors
	responses := make([]ActivityResponse, 0, len(groups))
	for _, group := range groups {
		count := enrollmentCounts[group.ID]
		activityResp := newActivityResponse(group, count)

		// Get supervisors from map (O(1) lookup)
		if supervisors := supervisorMap[group.ID]; len(supervisors) > 0 {
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
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidActivityID)))
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
		slog.Default().Error("Group is nil after GetGroup call", slog.Int64("group_id", id))
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("activity not found or could not be retrieved")))
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
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Get current staff ID - required for created_by
	staffID, _, err := rs.getStaffIDAndManagePermission(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorForbidden(errors.New("only staff members can create activities")))
		return
	}

	// Create activity group with creator
	group := &activities.Group{
		Name:            req.Name,
		MaxParticipants: req.MaxParticipants,
		IsOpen:          req.IsOpen,
		CategoryID:      req.CategoryID,
		PlannedRoomID:   req.PlannedRoomID,
		CreatedBy:       &staffID,
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
	var createdGroup *activities.Group
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		createdGroup, txErr = rs.ActivityService.CreateGroup(ctx, group, req.SupervisorIDs, schedules)
		return txErr
	}); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// EXTREMELY SIMPLIFIED APPROACH - don't try to get additional details at all
	// Just create a response with what we know is valid and return it
	if createdGroup == nil {
		// This should never happen if CreateGroup didn't return an error, but just in case
		slog.Default().Warn("CreateGroup returned nil group without error")
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
		MaxParticipants: createdGroup.ParticipantLimit(),
		IsOpen:          createdGroup.IsOpen,
		CategoryID:      createdGroup.CategoryID,
		PlannedRoomID:   createdGroup.PlannedRoomID,
		CreatedBy:       createdGroup.CreatedBy,
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
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Get current staff - required for created_by
	staff, err := rs.UserContextService.GetCurrentStaff(r.Context())
	if err != nil || staff == nil {
		common.RenderError(w, r, common.ErrorForbidden(errors.New("only staff members can create activities")))
		return
	}

	// Create activity group with smart defaults and creator
	group := &activities.Group{
		Name:            req.Name,
		MaxParticipants: req.MaxParticipants,
		IsOpen:          true, // Default to true for quick-create
		CategoryID:      req.CategoryID,
		PlannedRoomID:   req.RoomID,
		CreatedBy:       &staff.ID,
	}

	// Auto-assign creator as primary supervisor
	supervisorIDs := []int64{staff.ID}

	// Create the activity group with auto-assigned teacher supervision
	var createdGroup *activities.Group
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		createdGroup, txErr = rs.ActivityService.CreateGroup(ctx, group, supervisorIDs, nil)
		return txErr
	}); err != nil {
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
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidActivityID)))
		return
	}

	// Get staff ID and check for manage permission
	staffID, hasManagePermission, err := rs.getStaffIDAndManagePermission(r)
	if err != nil && !hasManagePermission {
		common.RenderError(w, r, common.ErrorForbidden(errors.New("only staff members can update activities")))
		return
	}

	// Parse request
	req := &ActivityRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Get and update existing group
	existingGroup, err := rs.ActivityService.GetGroup(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	updateGroupFields(existingGroup, req)

	// Convert schedule requests to model schedules for the service call.
	newSchedules := make([]*activities.Schedule, 0, len(req.Schedules))
	for _, scheduleReq := range req.Schedules {
		newSchedules = append(newSchedules, &activities.Schedule{
			Weekday:     scheduleReq.Weekday,
			TimeframeID: scheduleReq.TimeframeID,
		})
	}

	// Group fields + supervisors + schedules update as one unit: any failure
	// rolls back the whole transaction and propagates (issue #575 B10 —
	// previously supervisor/schedule failures were logged at Warn and the
	// partial state committed with a 200).
	var updatedGroup *activities.Group
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		updatedGroup, txErr = rs.ActivityService.UpdateGroupWithDetails(ctx, existingGroup, staffID, hasManagePermission, req.SupervisorIDs, newSchedules)
		return txErr
	}); err != nil {
		// Check for ownership error
		if errors.Is(err, activitiesSvc.ErrNotOwner) {
			common.RenderError(w, r, common.ErrorForbidden(err))
			return
		}
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

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
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidActivityID)))
		return
	}

	// Get staff ID and check for manage permission
	staffID, hasManagePermission, err := rs.getStaffIDAndManagePermission(r)
	if err != nil && !hasManagePermission {
		common.RenderError(w, r, common.ErrorForbidden(errors.New("only staff members can delete activities")))
		return
	}

	// Delete the activity with ownership check
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.ActivityService.DeleteGroup(ctx, id, staffID, hasManagePermission)
	}); err != nil {
		// Check for ownership error
		if errors.Is(err, activitiesSvc.ErrNotOwner) {
			common.RenderError(w, r, common.ErrorForbidden(err))
			return
		}
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Activity deleted successfully")
}

// getTimespans handles retrieving all available time spans for activities
func (rs *Resource) getTimespans(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Fetch active timeframes from the schedule service
	timeframes, err := rs.ScheduleService.FindActiveTimeframes(ctx)
	if err != nil {
		slog.Default().ErrorContext(ctx, "Error fetching timeframes", slog.String("error", err.Error()))
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
