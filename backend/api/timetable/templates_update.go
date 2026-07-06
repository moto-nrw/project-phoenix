package timetable

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type updateTemplateRequest struct {
	Name             string  `json:"name"`
	Type             string  `json:"type"`
	Weekdays         []int   `json:"weekdays"`
	StartTime        string  `json:"start_time"`
	EndTime          string  `json:"end_time"`
	RoomID           int64   `json:"room_id"`
	CategoryID       int64   `json:"category_id"`
	MaxParticipants  *int    `json:"max_participants,omitempty"`
	WeekPattern      *int    `json:"week_pattern,omitempty"`
	CalendarPeriodID *int64  `json:"calendar_period_id,omitempty"`
	EducationGroupID *int64  `json:"education_group_id,omitempty"`
	StudentIDs       []int64 `json:"student_ids,omitempty"`
	StaffIDs         []int64 `json:"staff_ids,omitempty"`
	PrimaryStaffID   *int64  `json:"primary_staff_id,omitempty"`
}

func (req *updateTemplateRequest) Bind(_ *http.Request) error {
	if req.Name == "" {
		return errors.New("name is required")
	}
	if len(req.Name) > 255 {
		return errors.New("name cannot exceed 255 characters")
	}
	if req.RoomID <= 0 {
		return errors.New("room_id is required")
	}
	if req.CategoryID <= 0 {
		return errors.New("category_id is required")
	}
	if req.StartTime == "" || req.EndTime == "" {
		return errors.New("start_time and end_time are required")
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

func (rs *Resource) validateTemplateEducationGroup(ctx context.Context, groupID *int64) error {
	if groupID == nil {
		return nil
	}
	if *groupID <= 0 {
		return errors.New("education_group_id must be positive when set")
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return errors.New("no tenant in context")
	}
	exists, err := rs.TimetableData.EducationGroupExists(ctx, *groupID)
	if err != nil {
		return fmt.Errorf("validate education_group_id: %w", err)
	}
	if !exists {
		return errors.New("education_group_id does not reference a group in this tenant")
	}
	return nil
}

func (rs *Resource) getTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := templateIDFromRequest(w, r)
	if !ok {
		return
	}
	if rs.TimetableData == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable resource not fully wired")))
		return
	}
	templates, err := rs.loadTemplates(r.Context(), &id)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("load template failed", err))
		return
	}
	if len(templates) == 0 {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("template not found")))
		return
	}
	common.Respond(w, r, http.StatusOK, templates[0], "Template retrieved")
}

func (rs *Resource) updateTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := templateIDFromRequest(w, r)
	if !ok {
		return
	}
	if rs.TimetableData == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable resource not fully wired")))
		return
	}
	req := &updateTemplateRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if !isValidActivityType(req.Type) {
		common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("invalid type %q (must be care, activity, or external)", req.Type)))
		return
	}
	startTime, err := parseClockTime(req.StartTime)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid start_time format, expected HH:MM")))
		return
	}
	endTime, err := parseClockTime(req.EndTime)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid end_time format, expected HH:MM")))
		return
	}
	if !endTime.After(startTime) {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("end_time must be after start_time")))
		return
	}
	weekPattern := 0
	if req.WeekPattern != nil {
		weekPattern = *req.WeekPattern
	}
	if weekPattern < 0 || weekPattern > 2 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("week_pattern must be 0 (every), 1 (A), or 2 (B)")))
		return
	}
	maxParticipants := 999
	if req.MaxParticipants != nil && *req.MaxParticipants > 0 {
		maxParticipants = *req.MaxParticipants
	}
	ctx := r.Context()
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("no tenant in context")))
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
	exists, err := rs.templateExists(ctx, id)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("load template failed", err))
		return
	}
	if !exists {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("template not found")))
		return
	}
	timeframeID, err := rs.findOrCreateTimeframe(ctx, startTime, endTime, req.Name)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("resolve timeframe failed", err))
		return
	}
	if _, err := rs.TimetableData.UpdateTemplateFields(ctx, id, req.Name, req.Type, req.CategoryID, req.RoomID, req.EducationGroupID, maxParticipants); err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("update template failed", err))
		return
	}
	if err := rs.TimetableData.DeleteSchedulesByGroupID(ctx, id); err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("replace template schedules failed", err))
		return
	}
	for _, weekday := range req.Weekdays {
		tfID := timeframeID
		sched := &activitiesModel.Schedule{
			Weekday:          weekday,
			TimeframeID:      &tfID,
			ActivityGroupID:  id,
			WeekPattern:      weekPattern,
			CalendarPeriodID: req.CalendarPeriodID,
		}
		sched.SetTenantID(tenantID)
		if err := rs.TimetableData.CreateActivitySchedule(ctx, sched); err != nil {
			common.RenderError(w, r, common.ErrorInternalServerWrap("create schedule failed", err))
			return
		}
	}
	if err := rs.replaceTemplateStudents(ctx, id, req.StudentIDs, req.CalendarPeriodID, rosterValidFrom); err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("assign template students failed", err))
		return
	}
	if err := rs.replaceTemplateStaff(ctx, id, req.StaffIDs, req.PrimaryStaffID, req.CalendarPeriodID, rosterValidFrom); err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("assign template staff failed", err))
		return
	}
	templates, err := rs.loadTemplates(ctx, &id)
	if err != nil || len(templates) == 0 {
		common.RenderError(w, r, common.ErrorInternalServerWrap("reload template failed", err))
		return
	}
	common.Respond(w, r, http.StatusOK, templates[0], "Template updated")
}

func (rs *Resource) archiveTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := templateIDFromRequest(w, r)
	if !ok {
		return
	}
	if rs.TimetableData == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable resource not fully wired")))
		return
	}
	tenantID := tenant.FromContext(r.Context())
	if tenantID <= 0 {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("no tenant in context")))
		return
	}
	n, err := rs.TimetableData.ArchiveTemplate(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("archive template failed", err))
		return
	}
	if n == 0 {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("template not found")))
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]any{"id": id}, "Template archived")
}

func templateIDFromRequest(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid template id")))
		return 0, false
	}
	return id, true
}
