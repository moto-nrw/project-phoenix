package httpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
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
func (rs *Resource) checkScheduleOwnership(w http.ResponseWriter, r *http.Request, schedule *activities.Schedule, activityID int64) bool {
	if schedule.ActivityGroupID != activityID {
		common.RenderError(w, r, common.ErrorForbidden(errors.New("schedule does not belong to the specified activity")))
		return false
	}
	return true
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

// getAvailableTimeSlots retrieves available time slots for scheduling
func (rs *Resource) getAvailableTimeSlots(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get query parameters
	weekday := r.URL.Query().Get("weekday")
	roomIDStr := r.URL.Query().Get("room_id")
	durationStr := r.URL.Query().Get("duration")

	// Validate query parameters
	if err := parseAndValidateWeekday(weekday); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	if err := parseAndValidateRoomID(roomIDStr); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	duration, err := parseDurationWithDefault(durationStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Set date range for the next 7 days
	startDate := timezone.Today()
	endDate := startDate.AddDate(0, 0, 7)

	// Find available time slots
	availableSlots, err := rs.ScheduleService.FindAvailableSlots(ctx, startDate, endDate, duration)
	if err != nil {
		slog.Default().ErrorContext(ctx, "Error finding available time slots", slog.String("error", err.Error()))
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

// createActivitySchedule adds a new schedule to an activity
func (rs *Resource) createActivitySchedule(w http.ResponseWriter, r *http.Request) {
	activity, ok := rs.parseAndGetActivity(w, r)
	if !ok {
		return
	}

	// Parse request
	var req ScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Validate request
	if !activities.IsValidWeekday(req.Weekday) {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidWeekday)))
		return
	}

	// Create schedule
	schedule := &activities.Schedule{
		ActivityGroupID: activity.ID,
		Weekday:         req.Weekday,
		TimeframeID:     req.TimeframeID,
	}

	tenantID := tenant.FromContext(r.Context())
	var createdSchedule *activities.Schedule
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		createdSchedule, txErr = rs.ActivityService.AddSchedule(ctx, activity.ID, schedule)
		return txErr
	}); err != nil {
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
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Validate request
	if !activities.IsValidWeekday(req.Weekday) {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidWeekday)))
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
	tenantID := tenant.FromContext(r.Context())
	var updatedSchedule *activities.Schedule
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		updatedSchedule, txErr = rs.ActivityService.UpdateSchedule(ctx, existingSchedule)
		return txErr
	}); err != nil {
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
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.ActivityService.DeleteSchedule(ctx, scheduleID)
	}); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Schedule deleted successfully")
}
