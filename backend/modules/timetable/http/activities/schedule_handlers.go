package httpadapter

import (
	"errors"
	"fmt"
	"net/http"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// parseScheduleID parses schedule ID from URL param "scheduleId".
// Returns 0 and false if parsing fails (error already rendered).
func (rs *Resource) parseScheduleID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	scheduleID, err := common.ParseIDParam(r, "scheduleId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid schedule ID")))
		return 0, false
	}
	return scheduleID, true
}

// =============================================================================
// OWNERSHIP CHECK HELPERS - Verify resources belong to activity
// =============================================================================

// checkScheduleOwnership verifies the schedule belongs to the specified activity.
// Returns false and renders error if ownership check fails.
func (rs *Resource) checkScheduleOwnership(w http.ResponseWriter, r *http.Request, schedule *activityContract.Schedule, activityID int64) bool {
	if schedule.ActivityGroupID != activityID {
		common.RenderError(w, r, common.ErrorForbidden(errors.New("schedule does not belong to the specified activity")))
		return false
	}
	return true
}

// SCHEDULE MANAGEMENT HANDLERS

// getActivitySchedules retrieves all schedules for a specific activity
func (rs *Resource) getActivitySchedules(w http.ResponseWriter, r *http.Request) {
	activity, ok := rs.parseAndGetActivity(w, r)
	if !ok {
		return
	}

	// Get schedules for the activity
	schedules, err := rs.ActivityService.GetGroupSchedules(r.Context(), activity.ID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Convert to response objects
	responses := make([]ScheduleResponse, 0, len(schedules))
	for _, schedule := range schedules {
		if schedule == nil {
			continue // Skip nil schedules to prevent panic
		}
		responses = append(responses, newScheduleResponse(schedule))
	}

	common.Respond(w, r, http.StatusOK, responses, fmt.Sprintf("Schedules for activity '%s' retrieved successfully", activity.Name))
}

// getActivitySchedule retrieves a specific schedule by ID
func (rs *Resource) getActivitySchedule(w http.ResponseWriter, r *http.Request) {
	activity, ok := rs.parseAndGetActivity(w, r)
	if !ok {
		return
	}

	scheduleID, ok := rs.parseScheduleID(w, r)
	if !ok {
		return
	}

	// Get schedule
	schedule, err := rs.ActivityService.GetSchedule(r.Context(), scheduleID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Check if schedule belongs to the specified activity
	if !rs.checkScheduleOwnership(w, r, schedule, activity.ID) {
		return
	}

	common.Respond(w, r, http.StatusOK, newScheduleResponse(schedule), "Schedule retrieved successfully")
}
