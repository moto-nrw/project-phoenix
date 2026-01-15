package activities

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/models/activities"
	activitiesSvc "github.com/moto-nrw/project-phoenix/services/activities"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	usercontextSvc "github.com/moto-nrw/project-phoenix/services/usercontext"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

const (
	routeScheduleByID         = "/{id}/schedules/{scheduleId}"
	msgActivityCreatedSuccess = "Activity created successfully"
	msgActivityUpdatedSuccess = "Activity updated successfully"
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

// newCategoryResponse converts a category model to a response object
func newCategoryResponse(category *activities.Category) CategoryResponse {
	// Handle nil category input
	if category == nil {
		logger.Logger.Warn("attempted to create CategoryResponse from nil category")
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
		logger.Logger.Error("attempted to create ActivityResponse from nil group")
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
				logger.Logger.WithField("group_id", group.ID).Warn("nil schedule encountered in group")
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
		logger.Logger.WithField("error", detailsErr).Warn("error getting detailed group info, using fallback")
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
		logger.Logger.WithField("error", scheduleErr).Warn("error getting schedules")
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
			logger.Logger.WithFields(map[string]interface{}{
				"category_id": group.CategoryID,
				"error":       catErr,
			}).Warn("error getting category")
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
		logger.Logger.WithFields(map[string]interface{}{
			"activity_id": groupID,
			"error":       err,
		}).Warn("failed to update supervisors for activity")
	}
}

// replaceGroupSchedules removes existing schedules and adds new ones.
func (rs *Resource) replaceGroupSchedules(ctx context.Context, groupID int64, newSchedules []ScheduleRequest) {
	// Delete existing schedules
	existingSchedules, err := rs.ActivityService.GetGroupSchedules(ctx, groupID)
	if err != nil {
		logger.Logger.WithField("error", err).Warn("failed to get existing schedules")
	} else {
		for _, schedule := range existingSchedules {
			err = rs.ActivityService.DeleteSchedule(ctx, schedule.ID)
			if err != nil {
				logger.Logger.WithFields(map[string]interface{}{
					"schedule_id": schedule.ID,
					"error":       err,
				}).Warn("failed to delete schedule")
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
			logger.Logger.WithFields(map[string]interface{}{
				"weekday":      scheduleReq.Weekday,
				"timeframe_id": scheduleReq.TimeframeID,
				"error":        err,
			}).Warn("failed to add schedule")
		}
	}
}

// fetchUpdatedGroupData retrieves the updated group with details and handles nil checks.
func (rs *Resource) fetchUpdatedGroupData(ctx context.Context, updatedGroup *activities.Group) (*activities.Group, error) {
	detailedGroup, _, updatedSchedules, err := rs.ActivityService.GetGroupWithDetails(ctx, updatedGroup.ID)
	if err != nil {
		logger.Logger.WithField("error", err).Warn("failed to get detailed group info after update")
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
			logger.Logger.Warn("updatedSchedules is nil despite no error from GetGroupWithDetails")
			updatedGroup.Schedules = []*activities.Schedule{}
		}
	} else {
		logger.Logger.Warn("detailedGroup is nil despite no error from GetGroupWithDetails")
		if updatedGroup != nil {
			updatedGroup.Schedules = []*activities.Schedule{}
		}
	}

	return updatedGroup, nil
}

// buildUpdateResponse creates the final response for an activity update.
func (rs *Resource) buildUpdateResponse(ctx context.Context, group *activities.Group, activityID int64) (ActivityResponse, error) {
	if group == nil {
		logger.Logger.Error("updatedGroup is nil before creating response, returning empty response")
		return ActivityResponse{}, errors.New("group is nil")
	}

	enrolledStudents, err := rs.ActivityService.GetEnrolledStudents(ctx, activityID)
	enrollmentCount := 0
	if err != nil {
		logger.Logger.WithField("error", err).Warn("failed to get enrolled students")
	} else if enrolledStudents != nil {
		enrollmentCount = len(enrolledStudents)
	}

	return newActivityResponse(group, enrollmentCount), nil
}

// fetchSupervisorsBySpecialization retrieves supervisors filtered by specialization.
func (rs *Resource) fetchSupervisorsBySpecialization(ctx context.Context, specialization string) ([]SupervisorResponse, error) {
	teachers, err := rs.UserService.GetTeachersBySpecialization(ctx, specialization)
	if err != nil {
		return nil, err
	}

	supervisors := make([]SupervisorResponse, 0, len(teachers))
	for _, teacher := range teachers {
		fullTeacher, err := rs.UserService.GetTeacherWithDetails(ctx, teacher.ID)
		if err != nil {
			logger.Logger.WithFields(map[string]interface{}{
				"teacher_id": teacher.ID,
				"error":      err,
			}).Warn("error fetching full teacher data")
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
	staff, err := rs.UserService.ListStaff(ctx, nil)
	if err != nil {
		return nil, err
	}

	supervisors := make([]SupervisorResponse, 0, len(staff))
	for _, staffMember := range staff {
		person, err := rs.UserService.Get(ctx, staffMember.PersonID)
		if err != nil {
			logger.Logger.WithFields(map[string]interface{}{
				"staff_id": staffMember.ID,
				"error":    err,
			}).Warn("error fetching person data for staff")
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
