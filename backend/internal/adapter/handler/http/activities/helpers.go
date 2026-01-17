package activities

import (
	"context"
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/activities"
)

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

// checkSupervisorOwnership verifies the supervisor belongs to the specified activity.
// Returns false and renders error if ownership check fails.
func (rs *Resource) checkSupervisorOwnership(w http.ResponseWriter, r *http.Request, supervisor *activities.SupervisorPlanned, activityID int64) bool {
	if supervisor.GroupID != activityID {
		common.RenderError(w, r, ErrorForbidden(errors.New("supervisor does not belong to the specified activity")))
		return false
	}
	return true
}

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
