package timetable

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
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
	TargetGroupType   string                  `json:"target_group_type,omitempty"`
	TargetGradeLevel  *int16                  `json:"target_grade_level,omitempty"`
	TargetSchoolClass *string                 `json:"target_school_class,omitempty"`
	Targets           []templateTargetRequest `json:"targets,omitempty"`
	// ListKind classifies the template for printable daily lists (#1565);
	// omitted/null/empty clears it.
	ListKind *string `json:"list_kind,omitempty"`
	// Notes is the durable Wochennotiz for the template; omitted/null clears it.
	Notes          *string `json:"notes,omitempty"`
	StudentIDs     []int64 `json:"student_ids,omitempty"`
	StaffIDs       []int64 `json:"staff_ids,omitempty"`
	PrimaryStaffID *int64  `json:"primary_staff_id,omitempty"`
}

func (req *updateTemplateRequest) Bind(_ *http.Request) error {
	if req.Name == "" {
		return errors.New("name is required")
	}
	if len(req.Name) > 255 {
		return errors.New("name cannot exceed 255 characters")
	}
	if req.Notes != nil && len(*req.Notes) > 2000 {
		return errors.New("notes cannot exceed 2000 characters")
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
	if err := validateTemplateWeekdays(req.Weekdays); err != nil {
		return err
	}
	target := &activitiesModel.Group{
		TargetGroupType:   req.TargetGroupType,
		TargetGradeLevel:  req.TargetGradeLevel,
		TargetSchoolClass: req.TargetSchoolClass,
		EducationGroupID:  req.EducationGroupID,
	}
	if err := target.ValidateTargetGroup(); err != nil {
		if len(req.Targets) == 0 {
			return err
		}
	}
	req.TargetGroupType = target.TargetGroupType
	req.TargetSchoolClass = target.TargetSchoolClass
	if err := validateTargetRequests(req.TargetGroupType, req.Targets); err != nil {
		return err
	}
	listKind, err := normalizeTemplateListKind(req.ListKind)
	if err != nil {
		return err
	}
	req.ListKind = listKind
	return nil
}

// parsedUpdateTemplate holds the request plus values derived from cheap format
// validation (clock window, defaulted week pattern and cap).
type parsedUpdateTemplate struct {
	req             *updateTemplateRequest
	startTime       time.Time
	endTime         time.Time
	weekPattern     int
	maxParticipants int
}

// parseUpdateTemplateRequest binds and format-validates the request. Format
// errors render precise 400 messages here rather than from the service.
func parseUpdateTemplateRequest(w http.ResponseWriter, r *http.Request) (*parsedUpdateTemplate, bool) {
	req := &updateTemplateRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return nil, false
	}
	if !isValidActivityType(req.Type) {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			fmt.Errorf("invalid type %q (must be care, activity, or external)", req.Type)))
		return nil, false
	}
	timing, ok := parseTemplateTiming(w, r, req.StartTime, req.EndTime, req.WeekPattern, req.MaxParticipants)
	if !ok {
		return nil, false
	}
	return &parsedUpdateTemplate{
		req:             req,
		startTime:       timing.startTime,
		endTime:         timing.endTime,
		weekPattern:     timing.weekPattern,
		maxParticipants: timing.maxParticipants,
	}, true
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
	parsed, ok := parseUpdateTemplateRequest(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("no tenant in context")))
		return
	}
	templates, err := rs.loadTemplates(ctx, &id)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("load template failed", err))
		return
	}
	if len(templates) == 0 {
		if hasWeekendTemplateWeekday(parsed.req.Weekdays) {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("timetable templates can only be scheduled from Monday to Friday")))
			return
		}
		common.RenderError(w, r, common.ErrorNotFound(errors.New("template not found")))
		return
	}
	if err := validateLegacyTemplateWorkdays(templates[0].Schedules, parsed.req.Weekdays); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	gradeLevelMax, rosterValidFrom, ok := rs.templateWritePreflight(w, r, parsed.req.CalendarPeriodID, nil)
	if !ok {
		return
	}
	if err := rs.TimetableData.ValidateTemplateEducationGroup(ctx, parsed.req.EducationGroupID); err != nil {
		renderTemplateEducationGroupError(w, r, err)
		return
	}
	timeframeID, err := rs.TimetableData.FindOrCreateTimeframe(ctx, parsed.startTime, parsed.endTime, parsed.req.Name)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("resolve timeframe failed", err))
		return
	}
	updateErr := rs.TimetableData.UpdateTemplate(ctx, buildUpdateTemplateInput(id, parsed, timeframeID, gradeLevelMax, rosterValidFrom))
	if updateErr != nil {
		// findOrCreateTimeframe runs before the recurrence service. Tenant
		// middleware commits 4xx responses unless explicitly marked, so every
		// failed update must roll back a timeframe created by this request.
		tenant.MarkRollback(ctx)
		renderUpdateTemplateError(w, r, updateErr)
		return
	}
	templates, err = rs.loadTemplates(ctx, &id)
	if err != nil || len(templates) == 0 {
		common.RenderError(w, r, common.ErrorInternalServerWrap("reload template failed", err))
		return
	}
	common.Respond(w, r, http.StatusOK, templates[0], "Template updated")
}

// validateLegacyTemplateWorkdays permits an update to retain a weekend
// schedule that already exists, so administrators can edit or normalize old
// templates. A PUT can never introduce a new weekend weekday.
func validateLegacyTemplateWorkdays(existing []templateScheduleResponse, requested []int) error {
	legacy := make(map[int]struct{})
	for _, schedule := range existing {
		if schedule.Weekday > activitiesModel.WeekdayFriday {
			legacy[schedule.Weekday] = struct{}{}
		}
	}
	for _, weekday := range requested {
		if weekday > activitiesModel.WeekdayFriday {
			if _, ok := legacy[weekday]; !ok {
				return errors.New("timetable templates can only be scheduled from Monday to Friday")
			}
		}
	}
	return nil
}

func hasWeekendTemplateWeekday(weekdays []int) bool {
	for _, weekday := range weekdays {
		if weekday > activitiesModel.WeekdayFriday {
			return true
		}
	}
	return false
}

// buildUpdateTemplateInput maps the parsed request and resolved preconditions
// into the service input.
func buildUpdateTemplateInput(
	id int64,
	parsed *parsedUpdateTemplate,
	timeframeID int64,
	gradeLevelMax int,
	rosterValidFrom timezone.Date,
) scheduleSvc.TemplateUpdateInput {
	req := parsed.req
	return scheduleSvc.TemplateUpdateInput{
		TemplateID: id,
		Fields: activitiesModel.TemplateFieldsUpdate{
			Name:              req.Name,
			Type:              req.Type,
			CategoryID:        req.CategoryID,
			RoomID:            req.RoomID,
			EducationGroupID:  req.EducationGroupID,
			MaxParticipants:   parsed.maxParticipants,
			RequiredStaff:     normalizeRequiredStaff(req.RequiredStaff),
			CalendarPeriodID:  req.CalendarPeriodID,
			TargetGroupType:   req.TargetGroupType,
			TargetGradeLevel:  req.TargetGradeLevel,
			TargetSchoolClass: req.TargetSchoolClass,
			ListKind:          req.ListKind,
			Notes:             normalizeNotes(req.Notes),
		},
		Weekdays:         req.Weekdays,
		TimeframeID:      timeframeID,
		WeekPattern:      parsed.weekPattern,
		CalendarPeriodID: req.CalendarPeriodID,
		RosterValidFrom:  rosterValidFrom,
		StudentIDs:       req.StudentIDs,
		StaffIDs:         req.StaffIDs,
		PrimaryStaffID:   req.PrimaryStaffID,
		Targets:          targetModels(req.Targets),
		GradeLevelMax:    gradeLevelMax,
	}
}

// renderUpdateTemplateError maps an UpdateTemplate failure to its HTTP response.
// A capped/archived segment surfaces as the same 404 as the preflight lookup;
// the care-offering, roster-rebase, and grade-cap conflicts each carry their
// own status and code; everything else is a 500.
func renderUpdateTemplateError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, scheduleSvc.ErrTemplateSegmentNotEditable):
		common.RenderError(w, r, common.ErrorNotFound(errors.New("template not found")))
	case errors.Is(err, scheduleSvc.ErrCategoryNotAssignable):
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("category is archived or unavailable")))
	case errors.Is(err, scheduleSvc.ErrTemplateWeekendWeekday):
		common.RenderError(w, r, common.ErrorInvalidRequest(scheduleSvc.ErrTemplateWeekendWeekday))
	case renderTemplateCareOfferingConflict(w, r, err):
	case renderTemplateRosterRebaseConflict(w, r, err):
	case renderTemplateTargetGradeLimit(w, r, err):
	default:
		common.RenderError(w, r, common.ErrorInternalServerWrap("update template failed", err))
	}
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
