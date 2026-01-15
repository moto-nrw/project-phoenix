package activities

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/logging"
	"github.com/moto-nrw/project-phoenix/models/activities"
)

// =============================================================================
// SCHEDULE MANAGEMENT HANDLERS
// =============================================================================

// parseScheduleID parses schedule ID from URL param "scheduleId".
// Returns 0 and false if parsing fails (error already rendered).
func (rs *Resource) parseScheduleID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	scheduleID, err := common.ParseIDParam(r, "scheduleId")
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid schedule ID")))
		return 0, false
	}
	return scheduleID, true
}

// checkScheduleOwnership verifies the schedule belongs to the specified activity.
// Returns false and renders error if ownership check fails.
func (rs *Resource) checkScheduleOwnership(w http.ResponseWriter, r *http.Request, schedule *activities.Schedule, activityID int64) bool {
	if schedule.ActivityGroupID != activityID {
		common.RenderError(w, r, ErrorForbidden(errors.New("schedule does not belong to the specified activity")))
		return false
	}
	return true
}

// newScheduleResponse converts a schedule model to a response object.
func newScheduleResponse(schedule *activities.Schedule) ScheduleResponse {
	if schedule == nil {
		return ScheduleResponse{}
	}
	return ScheduleResponse{
		ID:              schedule.ID,
		Weekday:         schedule.Weekday,
		TimeframeID:     schedule.TimeframeID,
		ActivityGroupID: schedule.ActivityGroupID,
		CreatedAt:       schedule.CreatedAt,
		UpdatedAt:       schedule.UpdatedAt,
	}
}

// formatEndTime safely formats the end time, handling nil values
func formatEndTime(endTime *time.Time) string {
	if endTime == nil {
		return ""
	}
	return endTime.Format("15:04")
}

// parseAndValidateWeekday validates the weekday query parameter.
func parseAndValidateWeekday(weekday string) error {
	if weekday == "" {
		return nil
	}

	weekdayInt, err := strconv.Atoi(weekday)
	if err != nil || !activities.IsValidWeekday(weekdayInt) {
		return errors.New(common.MsgInvalidWeekday)
	}
	return nil
}

// parseAndValidateRoomID validates the room_id query parameter.
func parseAndValidateRoomID(roomIDStr string) error {
	if roomIDStr == "" {
		return nil
	}

	_, err := strconv.ParseInt(roomIDStr, 10, 64)
	if err != nil {
		return errors.New("invalid room ID")
	}
	return nil
}

// parseDurationWithDefault parses duration string or returns default (2 hours).
func parseDurationWithDefault(durationStr string) (time.Duration, error) {
	if durationStr == "" {
		return 2 * time.Hour, nil
	}

	minutes, err := strconv.Atoi(durationStr)
	if err != nil || minutes <= 0 {
		return 0, errors.New("invalid duration")
	}
	return time.Duration(minutes) * time.Minute, nil
}

// convertWeekdayToString converts weekday abbreviation to full weekday name
func convertWeekdayToString(weekday string) string {
	weekdayMap := map[string]string{
		"MON": "Monday",
		"TUE": "Tuesday",
		"WED": "Wednesday",
		"THU": "Thursday",
		"FRI": "Friday",
		"SAT": "Saturday",
		"SUN": "Sunday",
	}
	if fullName, exists := weekdayMap[weekday]; exists {
		return fullName
	}
	return weekday
}

// generateSlotName creates a descriptive name for the time slot
func generateSlotName(startTime time.Time, endTime *time.Time) string {
	if endTime == nil {
		return fmt.Sprintf("From %s", startTime.Format("15:04"))
	}
	return fmt.Sprintf("%s - %s", startTime.Format("15:04"), endTime.Format("15:04"))
}

// getTimespans handles retrieving all available time spans for activities
func (rs *Resource) getTimespans(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Fetch active timeframes from the schedule service
	timeframes, err := rs.ScheduleService.FindActiveTimeframes(ctx)
	if err != nil {
		logging.Logger.WithError(err).Error("Error fetching timeframes")
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

// getAvailableTimeSlots retrieves available time slots for scheduling
func (rs *Resource) getAvailableTimeSlots(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get query parameters
	weekday := r.URL.Query().Get("weekday")
	roomIDStr := r.URL.Query().Get("room_id")
	durationStr := r.URL.Query().Get("duration")

	// Validate query parameters
	if err := parseAndValidateWeekday(weekday); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	if err := parseAndValidateRoomID(roomIDStr); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	duration, err := parseDurationWithDefault(durationStr)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Set date range for the next 7 days
	startDate := time.Now().Truncate(24 * time.Hour)
	endDate := startDate.AddDate(0, 0, 7)

	// Find available time slots
	availableSlots, err := rs.ScheduleService.FindAvailableSlots(ctx, startDate, endDate, duration)
	if err != nil {
		logging.Logger.WithError(err).Error("Error finding available time slots")
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to retrieve available time slots")
		return
	}

	// Convert and filter slots
	var timespans []TimespanResponse
	for _, slot := range availableSlots {
		// Apply weekday filter if specified
		if weekday != "" {
			slotWeekday := slot.StartTime.Weekday().String()
			expectedWeekday := convertWeekdayToString(weekday)
			if slotWeekday != expectedWeekday {
				continue
			}
		}

		timespans = append(timespans, TimespanResponse{
			ID:          slot.ID,
			Name:        generateSlotName(slot.StartTime, slot.EndTime),
			StartTime:   slot.StartTime.Format("15:04"),
			EndTime:     formatEndTime(slot.EndTime),
			Description: fmt.Sprintf("Available slot: %s", slot.StartTime.Format("Monday, Jan 2")),
		})
	}

	common.Respond(w, r, http.StatusOK, timespans, "Available time slots retrieved successfully")
}

// createActivitySchedule adds a new schedule to an activity
func (rs *Resource) createActivitySchedule(w http.ResponseWriter, r *http.Request) {
	activity, ok := rs.parseAndGetActivity(w, r)
	if !ok {
		return
	}

	// Parse request
	var req ScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Validate request
	if !activities.IsValidWeekday(req.Weekday) {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(common.MsgInvalidWeekday)))
		return
	}

	// Create schedule
	schedule := &activities.Schedule{
		ActivityGroupID: activity.ID,
		Weekday:         req.Weekday,
		TimeframeID:     req.TimeframeID,
	}

	createdSchedule, err := rs.ActivityService.AddSchedule(r.Context(), activity.ID, schedule)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, newScheduleResponse(createdSchedule), "Schedule created successfully")
}

// updateActivitySchedule updates an existing schedule
func (rs *Resource) updateActivitySchedule(w http.ResponseWriter, r *http.Request) {
	activity, ok := rs.parseAndGetActivity(w, r)
	if !ok {
		return
	}

	scheduleID, ok := rs.parseScheduleID(w, r)
	if !ok {
		return
	}

	// Parse request
	var req ScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Validate request
	if !activities.IsValidWeekday(req.Weekday) {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(common.MsgInvalidWeekday)))
		return
	}

	// Get existing schedule
	existingSchedule, err := rs.ActivityService.GetSchedule(r.Context(), scheduleID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Check if schedule belongs to the specified activity
	if !rs.checkScheduleOwnership(w, r, existingSchedule, activity.ID) {
		return
	}

	// Update schedule fields
	existingSchedule.Weekday = req.Weekday
	existingSchedule.TimeframeID = req.TimeframeID

	// Update schedule
	updatedSchedule, err := rs.ActivityService.UpdateSchedule(r.Context(), existingSchedule)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newScheduleResponse(updatedSchedule), "Schedule updated successfully")
}

// deleteActivitySchedule deletes a schedule
func (rs *Resource) deleteActivitySchedule(w http.ResponseWriter, r *http.Request) {
	activity, ok := rs.parseAndGetActivity(w, r)
	if !ok {
		return
	}

	scheduleID, ok := rs.parseScheduleID(w, r)
	if !ok {
		return
	}

	// Get schedule to verify ownership
	schedule, err := rs.ActivityService.GetSchedule(r.Context(), scheduleID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Check if schedule belongs to the specified activity
	if !rs.checkScheduleOwnership(w, r, schedule, activity.ID) {
		return
	}

	// Delete schedule
	if err := rs.ActivityService.DeleteSchedule(r.Context(), scheduleID); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Schedule deleted successfully")
}
