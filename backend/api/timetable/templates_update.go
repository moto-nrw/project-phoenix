package timetable

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type updateTemplateRequest struct {
	Name            string `json:"name"`
	Type            string `json:"type"`
	Weekdays        []int  `json:"weekdays"`
	StartTime       string `json:"start_time"`
	EndTime         string `json:"end_time"`
	RoomID          int64  `json:"room_id"`
	CategoryID      int64  `json:"category_id"`
	MaxParticipants *int   `json:"max_participants,omitempty"`
	// RequiredStaff is the optional manual Personalbedarf override (#1839);
	// omitted/null clears the override (derive from the Betreuungsschlüssel).
	RequiredStaff    *int   `json:"required_staff,omitempty"`
	WeekPattern      *int   `json:"week_pattern,omitempty"`
	CalendarPeriodID *int64 `json:"calendar_period_id,omitempty"`
	EducationGroupID *int64 `json:"education_group_id,omitempty"`
	// Zielgruppe fields — see createTemplateRequest for the full contract.
	TargetGroupType   string  `json:"target_group_type,omitempty"`
	TargetGradeLevel  *int16  `json:"target_grade_level,omitempty"`
	TargetSchoolClass *string `json:"target_school_class,omitempty"`
	StudentIDs        []int64 `json:"student_ids,omitempty"`
	StaffIDs          []int64 `json:"staff_ids,omitempty"`
	PrimaryStaffID    *int64  `json:"primary_staff_id,omitempty"`
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
	target := &activitiesModel.Group{
		TargetGroupType:   req.TargetGroupType,
		TargetGradeLevel:  req.TargetGradeLevel,
		TargetSchoolClass: req.TargetSchoolClass,
		EducationGroupID:  req.EducationGroupID,
	}
	if err := target.ValidateTargetGroup(); err != nil {
		return err
	}
	req.TargetGroupType = target.TargetGroupType
	req.TargetSchoolClass = target.TargetSchoolClass
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
	gradeLevelMax, err := rs.resolveTemplateGradeLevelMax(ctx)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap(
			"resolve template grade level limit failed", err))
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
	fieldsUpdate := activitiesModel.TemplateFieldsUpdate{
		Name:              req.Name,
		Type:              req.Type,
		CategoryID:        req.CategoryID,
		RoomID:            req.RoomID,
		EducationGroupID:  req.EducationGroupID,
		MaxParticipants:   maxParticipants,
		RequiredStaff:     normalizeRequiredStaff(req.RequiredStaff),
		CalendarPeriodID:  req.CalendarPeriodID,
		TargetGroupType:   req.TargetGroupType,
		TargetGradeLevel:  req.TargetGradeLevel,
		TargetSchoolClass: req.TargetSchoolClass,
	}
	updateErr := rs.TimetableData.UpdateTemplate(ctx, scheduleSvc.TemplateUpdateInput{
		TemplateID:       id,
		Fields:           fieldsUpdate,
		Weekdays:         req.Weekdays,
		TimeframeID:      timeframeID,
		WeekPattern:      weekPattern,
		CalendarPeriodID: req.CalendarPeriodID,
		RosterValidFrom:  rosterValidFrom,
		StudentIDs:       req.StudentIDs,
		StaffIDs:         req.StaffIDs,
		PrimaryStaffID:   req.PrimaryStaffID,
		GradeLevelMax:    gradeLevelMax,
	})
	if updateErr != nil {
		// findOrCreateTimeframe runs before the recurrence service. Tenant
		// middleware commits 4xx responses unless explicitly marked, so every
		// failed update must roll back a timeframe created by this request.
		tenant.MarkRollback(ctx)
	}
	if errors.Is(updateErr, scheduleSvc.ErrTemplateSegmentNotEditable) {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("template not found")))
		return
	} else if renderTemplateCareOfferingConflict(w, r, updateErr) {
		return
	} else if renderTemplateRosterRebaseConflict(w, r, updateErr) {
		return
	} else if renderTemplateTargetGradeLimit(w, r, updateErr) {
		return
	} else if updateErr != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("update template failed", updateErr))
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
		if renderTemplateCareOfferingConflict(w, r, err) {
			return
		}
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
	return common.ParsePositiveInt64IDWithError(w, r, "id", "invalid template id")
}
