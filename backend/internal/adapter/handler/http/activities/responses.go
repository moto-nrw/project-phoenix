package activities

import (
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/activities"
)

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
