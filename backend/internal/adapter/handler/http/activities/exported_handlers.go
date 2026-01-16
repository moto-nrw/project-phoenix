package activities

import "net/http"

// =============================================================================
// HANDLER ACCESSOR METHODS (for testing)
// =============================================================================

// ListActivitiesHandler returns the list activities handler
func (rs *Resource) ListActivitiesHandler() http.HandlerFunc { return rs.listActivities }

// GetActivityHandler returns the get activity handler
func (rs *Resource) GetActivityHandler() http.HandlerFunc { return rs.getActivity }

// CreateActivityHandler returns the create activity handler
func (rs *Resource) CreateActivityHandler() http.HandlerFunc { return rs.createActivity }

// QuickCreateActivityHandler returns the quick create activity handler
func (rs *Resource) QuickCreateActivityHandler() http.HandlerFunc { return rs.quickCreateActivity }

// UpdateActivityHandler returns the update activity handler
func (rs *Resource) UpdateActivityHandler() http.HandlerFunc { return rs.updateActivity }

// DeleteActivityHandler returns the delete activity handler
func (rs *Resource) DeleteActivityHandler() http.HandlerFunc { return rs.deleteActivity }

// ListCategoriesHandler returns the list categories handler
func (rs *Resource) ListCategoriesHandler() http.HandlerFunc { return rs.listCategories }

// GetTimespansHandler returns the get timespans handler
func (rs *Resource) GetTimespansHandler() http.HandlerFunc { return rs.getTimespans }

// GetActivitySchedulesHandler returns the get activity schedules handler
func (rs *Resource) GetActivitySchedulesHandler() http.HandlerFunc { return rs.getActivitySchedules }

// GetActivityScheduleHandler returns the get specific schedule handler
func (rs *Resource) GetActivityScheduleHandler() http.HandlerFunc { return rs.getActivitySchedule }

// GetAvailableTimeSlotsHandler returns the get available time slots handler
func (rs *Resource) GetAvailableTimeSlotsHandler() http.HandlerFunc { return rs.getAvailableTimeSlots }

// CreateActivityScheduleHandler returns the create schedule handler
func (rs *Resource) CreateActivityScheduleHandler() http.HandlerFunc {
	return rs.createActivitySchedule
}

// UpdateActivityScheduleHandler returns the update schedule handler
func (rs *Resource) UpdateActivityScheduleHandler() http.HandlerFunc {
	return rs.updateActivitySchedule
}

// DeleteActivityScheduleHandler returns the delete schedule handler
func (rs *Resource) DeleteActivityScheduleHandler() http.HandlerFunc {
	return rs.deleteActivitySchedule
}

// GetActivitySupervisorsHandler returns the get supervisors handler
func (rs *Resource) GetActivitySupervisorsHandler() http.HandlerFunc {
	return rs.getActivitySupervisors
}

// GetAvailableSupervisorsHandler returns the get available supervisors handler
func (rs *Resource) GetAvailableSupervisorsHandler() http.HandlerFunc {
	return rs.getAvailableSupervisors
}

// AssignSupervisorHandler returns the assign supervisor handler
func (rs *Resource) AssignSupervisorHandler() http.HandlerFunc { return rs.assignSupervisor }

// UpdateSupervisorRoleHandler returns the update supervisor role handler
func (rs *Resource) UpdateSupervisorRoleHandler() http.HandlerFunc { return rs.updateSupervisorRole }

// RemoveSupervisorHandler returns the remove supervisor handler
func (rs *Resource) RemoveSupervisorHandler() http.HandlerFunc { return rs.removeSupervisor }

// GetActivityStudentsHandler returns the get enrolled students handler
func (rs *Resource) GetActivityStudentsHandler() http.HandlerFunc { return rs.getActivityStudents }

// GetStudentEnrollmentsHandler returns the get student enrollments handler
func (rs *Resource) GetStudentEnrollmentsHandler() http.HandlerFunc { return rs.getStudentEnrollments }

// GetAvailableActivitiesHandler returns the get available activities handler
func (rs *Resource) GetAvailableActivitiesHandler() http.HandlerFunc {
	return rs.getAvailableActivities
}

// EnrollStudentHandler returns the enroll student handler
func (rs *Resource) EnrollStudentHandler() http.HandlerFunc { return rs.enrollStudent }

// UnenrollStudentHandler returns the unenroll student handler
func (rs *Resource) UnenrollStudentHandler() http.HandlerFunc { return rs.unenrollStudent }

// UpdateGroupEnrollmentsHandler returns the batch enrollment handler
func (rs *Resource) UpdateGroupEnrollmentsHandler() http.HandlerFunc {
	return rs.updateGroupEnrollments
}
