// Package timetable — POST /api/timetable/templates handler.
//
// Creates a recurring template (activities.groups with is_template=true)
// plus its weekday schedules and an underlying timeframe in one shot.
// This is the "Wiederkehrende Aktivität anlegen" path from the admin
// weekly planner — it hides the three-table coordination behind a single
// HTTP call so admins can model "Mensa Mo–Fr 12:00–12:50 in Mensa-Saal"
// without leaving the timetable view.
//
// Spontaneous one-off rows belong on the device (PyrePortal); see
// instances_create.go for that flow.
//
// Permission: SchedulesManage. Optionally triggers materialization for a
// caller-supplied window so the resulting instances appear immediately on
// the planner grid.
package timetable

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// createTemplateRequest is the JSON body accepted by POST /templates.
//
// Weekdays follow ISO 8601 (Mo=1 … Su=7). The handler creates one
// schedule row per weekday all linked to the same timeframe. WeekPattern
// matches activities.schedules.week_pattern: 0=every week, 1=A, 2=B.
//
// MaxParticipants is optional and defaults to 999 — the timetable use
// case is dominated by care/Mensa-style activities where the cap is the
// room capacity, not a numerical roster limit.
type createTemplateRequest struct {
	Name             string  `json:"name"`
	Type             string  `json:"type"` // care | activity | external
	Weekdays         []int   `json:"weekdays"`
	StartTime        string  `json:"start_time"` // HH:MM
	EndTime          string  `json:"end_time"`   // HH:MM
	RoomID           int64   `json:"room_id"`
	CategoryID       int64   `json:"category_id"`
	MaxParticipants  *int    `json:"max_participants,omitempty"`
	WeekPattern      *int    `json:"week_pattern,omitempty"`
	CalendarPeriodID *int64  `json:"calendar_period_id,omitempty"`
	EducationGroupID *int64  `json:"education_group_id,omitempty"`
	MaterializeFrom  *string `json:"materialize_from,omitempty"` // YYYY-MM-DD
	MaterializeTo    *string `json:"materialize_to,omitempty"`   // YYYY-MM-DD
	StudentIDs       []int64 `json:"student_ids,omitempty"`
	StaffIDs         []int64 `json:"staff_ids,omitempty"`
	PrimaryStaffID   *int64  `json:"primary_staff_id,omitempty"`
}

// Bind enforces presence, but defers format/business validation to the
// handler so error messages are precise.
func (req *createTemplateRequest) Bind(_ *http.Request) error {
	if req.Name == "" {
		return errors.New("name is required")
	}
	if len(req.Name) > 255 {
		return errors.New("name cannot exceed 255 characters")
	}
	if req.StartTime == "" {
		return errors.New("start_time is required (HH:MM)")
	}
	if req.EndTime == "" {
		return errors.New("end_time is required (HH:MM)")
	}
	if req.RoomID <= 0 {
		return errors.New("room_id is required")
	}
	if req.CategoryID <= 0 {
		return errors.New("category_id is required")
	}
	if len(req.Weekdays) == 0 {
		return errors.New("at least one weekday is required")
	}
	for _, w := range req.Weekdays {
		if !activitiesModel.IsValidWeekday(w) {
			return fmt.Errorf("invalid weekday %d (must be 1=Mon … 7=Sun)", w)
		}
	}
	return nil
}

// createTemplateResponse describes what the caller needs to update the
// planner UI: the new template id, the schedule ids, and (if materialize
// was requested) the count of fresh instances on the grid.
type createTemplateResponse struct {
	TemplateID       int64   `json:"template_id"`
	TimeframeID      int64   `json:"timeframe_id"`
	ScheduleIDs      []int64 `json:"schedule_ids"`
	InstancesCreated int     `json:"instances_created,omitempty"`
	MaterializedFrom string  `json:"materialized_from,omitempty"`
	MaterializedTo   string  `json:"materialized_to,omitempty"`
}

// createTemplate handles POST /api/timetable/templates.
func (rs *Resource) createTemplate(w http.ResponseWriter, r *http.Request) {
	if rs.timetableData == nil {
		common.RenderError(w, r, common.ErrorInternalServer(
			errors.New("timetable resource not fully wired")))
		return
	}

	req := &createTemplateRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	if !isValidActivityType(req.Type) {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			fmt.Errorf("invalid type %q (must be care, activity, or external)", req.Type)))
		return
	}

	startTime, err := parseClockTime(req.StartTime)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("invalid start_time format, expected HH:MM")))
		return
	}
	endTime, err := parseClockTime(req.EndTime)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("invalid end_time format, expected HH:MM")))
		return
	}
	if !endTime.After(startTime) {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("end_time must be after start_time")))
		return
	}

	weekPattern := 0
	if req.WeekPattern != nil {
		weekPattern = *req.WeekPattern
	}
	if weekPattern < 0 || weekPattern > 2 {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("week_pattern must be 0 (every), 1 (A), or 2 (B)")))
		return
	}

	maxParticipants := 999
	if req.MaxParticipants != nil && *req.MaxParticipants > 0 {
		maxParticipants = *req.MaxParticipants
	}

	ctx := r.Context()
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		common.RenderError(w, r, common.ErrorInternalServer(
			errors.New("no tenant in context")))
		return
	}
	rosterValidFrom, err := rs.templateRosterValidFrom(ctx, req.CalendarPeriodID)
	if err != nil {
		renderTemplatePeriodLookupError(w, r, err)
		return
	}
	if err := rs.validateTemplateEducationGroup(ctx, req.EducationGroupID); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// 1. Find or create timeframe matching the requested clock window.
	//    The find-or-create rule lives in the schedule service (shared with
	//    the WP-B3 template split) so it exists in exactly one place.
	timeframeID, err := rs.findOrCreateTimeframe(ctx, startTime, endTime, req.Name)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap(
			"resolve timeframe failed", err))
		return
	}

	// 2. Create the template (activities.groups with is_template=true).
	createdBy := rs.resolveStartedByStaffID(ctx)
	var createdByPtr *int64
	if createdBy > 0 {
		c := createdBy
		createdByPtr = &c
	}

	roomIDCopy := req.RoomID
	group := &activitiesModel.Group{
		Name:             req.Name,
		MaxParticipants:  maxParticipants,
		IsOpen:           true,
		CategoryID:       req.CategoryID,
		PlannedRoomID:    &roomIDCopy,
		Type:             req.Type,
		EducationGroupID: req.EducationGroupID,
		IsTemplate:       true,
		CreatedBy:        createdByPtr,
	}
	group.SetTenantID(tenantID)
	if err := rs.timetableData.CreateActivityGroup(ctx, group); err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap(
			"create template failed", err))
		return
	}

	// 3. One schedule row per requested weekday, all linked to the same
	//    timeframe + week_pattern. Each row drives its own materialization
	//    candidate.
	scheduleIDs := make([]int64, 0, len(req.Weekdays))
	for _, weekday := range req.Weekdays {
		tfID := timeframeID
		sched := &activitiesModel.Schedule{
			Weekday:          weekday,
			TimeframeID:      &tfID,
			ActivityGroupID:  group.ID,
			WeekPattern:      weekPattern,
			CalendarPeriodID: req.CalendarPeriodID,
		}
		sched.SetTenantID(tenantID)
		if err := rs.timetableData.CreateActivitySchedule(ctx, sched); err != nil {
			common.RenderError(w, r, common.ErrorInternalServerWrap(
				"create schedule failed", err))
			return
		}
		scheduleIDs = append(scheduleIDs, sched.ID)
	}

	if err := rs.replaceTemplateStudents(ctx, group.ID, req.StudentIDs, req.CalendarPeriodID, rosterValidFrom); err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap(
			"assign template students failed", err))
		return
	}
	if err := rs.replaceTemplateStaff(ctx, group.ID, req.StaffIDs, req.PrimaryStaffID, req.CalendarPeriodID, rosterValidFrom); err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap(
			"assign template staff failed", err))
		return
	}

	resp := createTemplateResponse{
		TemplateID:  group.ID,
		TimeframeID: timeframeID,
		ScheduleIDs: scheduleIDs,
	}

	// 4. Optionally materialize the visible window so the new template
	//    yields instances that show up on the grid immediately.
	if req.MaterializeFrom != nil && req.MaterializeTo != nil &&
		rs.materializationService != nil {
		from, ferr := berlinDate(*req.MaterializeFrom)
		to, terr := berlinDate(*req.MaterializeTo)
		if ferr == nil && terr == nil && !to.Before(from) {
			mat, mErr := rs.materializationService.MaterializeForTenant(
				ctx, from, to, scheduleSvc.MaterializationSourceManual,
			)
			if mErr != nil {
				rs.getLogger().Warn("template create: materialize failed (template still saved)",
					slog.Int64("template_id", group.ID),
					slog.String("error", mErr.Error()),
				)
			} else if mat != nil {
				resp.InstancesCreated = mat.InstancesCreated
				resp.MaterializedFrom = from.Format(dateLayout)
				resp.MaterializedTo = to.Format(dateLayout)
			}
		}
	}

	rs.getLogger().Info("template created",
		slog.Int64("tenant_id", tenantID),
		slog.Int64("template_id", group.ID),
		slog.String("type", req.Type),
		slog.Int("weekday_count", len(req.Weekdays)),
		slog.Int("instances_created", resp.InstancesCreated),
	)
	common.Respond(w, r, http.StatusCreated, resp, "Template created")
}

// findOrCreateTimeframe returns the id of an existing schedule.timeframes
// row matching [start, end] or inserts a fresh one. Description is set to
// the template name on first creation as a debug hint, but is informational
// only — lookups go by time window.
func (rs *Resource) findOrCreateTimeframe(ctx context.Context, start, end time.Time, descHint string) (int64, error) {
	existing, err := rs.timetableData.GetTimeframesByTimeRange(ctx, start, end)
	if err == nil {
		for _, tf := range existing {
			if tf == nil {
				continue
			}
			// Match exact clock times; FindByTimeRange may return overlapping
			// windows depending on impl, so be precise. Do not use
			// time.Time.Equal here: schedule.timeframes stores SQL TIME, and
			// drivers may decode TIME with a different date anchor than the
			// handler's parseClockTime uses.
			if sameClockTime(tf.StartTime, start) && tf.EndTime != nil && sameClockTime(*tf.EndTime, end) {
				return tf.ID, nil
			}
		}
	}

	endCopy := end
	tf := &scheduleModel.Timeframe{
		StartTime:   start,
		EndTime:     &endCopy,
		IsActive:    true,
		Description: fmt.Sprintf("auto: %s", descHint),
	}
	tf.SetTenantID(tenant.FromContext(ctx))
	if err := rs.timetableData.CreateTimeframe(ctx, tf); err != nil {
		return 0, fmt.Errorf("create timeframe: %w", err)
	}
	return tf.ID, nil
}

func sameClockTime(a, b time.Time) bool {
	return a.Hour() == b.Hour() &&
		a.Minute() == b.Minute() &&
		a.Second() == b.Second() &&
		a.Nanosecond() == b.Nanosecond()
}

// isValidActivityType matches the constants in models/activities/group.go.
// Kept local to the handler so the imports stay narrow.
func isValidActivityType(t string) bool {
	switch t {
	case activitiesModel.GroupTypeCare,
		activitiesModel.GroupTypeActivity,
		activitiesModel.GroupTypeExternal:
		return true
	}
	return false
}
