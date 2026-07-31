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
	"strings"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
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
	Name            string `json:"name"`
	Type            string `json:"type"` // care | activity | external
	Weekdays        []int  `json:"weekdays"`
	StartTime       string `json:"start_time"` // HH:MM
	EndTime         string `json:"end_time"`   // HH:MM
	RoomID          int64  `json:"room_id"`
	CategoryID      int64  `json:"category_id"`
	MaxParticipants *int   `json:"max_participants,omitempty"`
	// RequiredStaff is the optional manual Personalbedarf override (#1839);
	// omitted/null = derive from the Betreuungsschlüssel.
	RequiredStaff    *int   `json:"required_staff,omitempty"`
	WeekPattern      *int   `json:"week_pattern,omitempty"`
	CalendarPeriodID *int64 `json:"calendar_period_id,omitempty"`
	EducationGroupID *int64 `json:"education_group_id,omitempty"`
	// Zielgruppe (target-group) fields. TargetGroupType is one of
	// activitiesModel.TargetGroupType* ("jahrgang" | "klasse" | "gruppe" |
	// "angebot" | "none"/omitted). "gruppe" reuses EducationGroupID above
	// rather than a separate field. Cross-field validity is checked via
	// activitiesModel.Group.ValidateTargetGroup() in Bind() below.
	TargetGroupType   string  `json:"target_group_type,omitempty"`
	TargetGradeLevel  *int16  `json:"target_grade_level,omitempty"`
	TargetSchoolClass *string `json:"target_school_class,omitempty"`
	// ListKind classifies the template for printable daily lists (#1565):
	// one of activitiesModel.ListKind* ("edge_hours" | "learning_time" |
	// "activity" | "mensa") or omitted/empty for none.
	ListKind *string `json:"list_kind,omitempty"`
	// Notes is the optional durable Wochennotiz for the template; it shows on
	// every materialized instance and survives Re-Plan/Split.
	Notes           *string `json:"notes,omitempty"`
	MaterializeFrom *string `json:"materialize_from,omitempty"` // YYYY-MM-DD
	MaterializeTo   *string `json:"materialize_to,omitempty"`   // YYYY-MM-DD
	StudentIDs      []int64 `json:"student_ids,omitempty"`
	StaffIDs        []int64 `json:"staff_ids,omitempty"`
	PrimaryStaffID  *int64  `json:"primary_staff_id,omitempty"`
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
	if req.Notes != nil && len(*req.Notes) > 2000 {
		return errors.New("notes cannot exceed 2000 characters")
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
	if err := validateTemplateWorkdays(req.Weekdays); err != nil {
		return err
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
	listKind, err := normalizeTemplateListKind(req.ListKind)
	if err != nil {
		return err
	}
	req.ListKind = listKind
	return nil
}

func validateTemplateWorkdays(weekdays []int) error {
	for _, weekday := range weekdays {
		if !activitiesModel.IsValidWeekday(weekday) {
			return fmt.Errorf("invalid weekday %d (must be 1=Mon … 7=Sun)", weekday)
		}
		if weekday > activitiesModel.WeekdayFriday {
			return errors.New("timetable templates can only be scheduled from Monday to Friday")
		}
	}
	return nil
}

// normalizeTemplateListKind trims and validates an optional list_kind value
// (#1565). Empty/whitespace-only values normalize to nil ("no list kind").
func normalizeTemplateListKind(raw *string) (*string, error) {
	if raw == nil {
		return nil, nil
	}
	kind := strings.TrimSpace(*raw)
	if kind == "" {
		return nil, nil
	}
	if !activitiesModel.IsValidListKind(kind) {
		return nil, fmt.Errorf("invalid list_kind %q", kind)
	}
	return &kind, nil
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

// parsedCreateTemplate holds the request plus the values derived from cheap
// format validation (clock window, defaulted week pattern and cap).
type parsedCreateTemplate struct {
	req             *createTemplateRequest
	startTime       time.Time
	endTime         time.Time
	weekPattern     int
	maxParticipants int
}

// parseCreateTemplateRequest binds and format-validates the request. Format
// errors render precise 400 messages here rather than from the service.
func parseCreateTemplateRequest(w http.ResponseWriter, r *http.Request) (*parsedCreateTemplate, bool) {
	req := &createTemplateRequest{}
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
	return &parsedCreateTemplate{
		req:             req,
		startTime:       timing.startTime,
		endTime:         timing.endTime,
		weekPattern:     timing.weekPattern,
		maxParticipants: timing.maxParticipants,
	}, true
}

// createTemplate handles POST /api/timetable/templates.
func (rs *Resource) createTemplate(w http.ResponseWriter, r *http.Request) {
	if rs.TimetableData == nil {
		common.RenderError(w, r, common.ErrorInternalServer(
			errors.New("timetable resource not fully wired")))
		return
	}

	parsed, ok := parseCreateTemplateRequest(w, r)
	if !ok {
		return
	}

	ctx := r.Context()
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		common.RenderError(w, r, common.ErrorInternalServer(
			errors.New("no tenant in context")))
		return
	}

	gradeLevelMax, rosterValidFrom, ok := rs.templateWritePreflight(w, r, parsed.req.CalendarPeriodID)
	if !ok {
		return
	}

	result, err := rs.TimetableData.CreateTemplate(ctx, buildCreateTemplateInput(
		parsed, tenantID, gradeLevelMax, rosterValidFrom, rs.resolveStartedByStaffID(ctx),
	))
	if err != nil {
		renderCreateTemplateError(w, r, err)
		return
	}

	resp := createTemplateResponse{
		TemplateID:  result.TemplateID,
		TimeframeID: result.TimeframeID,
		ScheduleIDs: result.ScheduleIDs,
	}
	rs.materializeTemplateWindow(ctx, parsed.req, result.TemplateID, &resp)

	rs.getLogger().Info("template created",
		slog.Int64("tenant_id", tenantID),
		slog.Int64("template_id", result.TemplateID),
		slog.String("type", parsed.req.Type),
		slog.Int("weekday_count", len(parsed.req.Weekdays)),
		slog.Int("instances_created", resp.InstancesCreated),
	)
	common.Respond(w, r, http.StatusCreated, resp, "Template created")
}

// buildCreateTemplateInput maps the parsed request and resolved preconditions
// into the service input, normalizing the optional Personalbedarf override and
// series note.
func buildCreateTemplateInput(
	parsed *parsedCreateTemplate,
	tenantID int64,
	gradeLevelMax int,
	rosterValidFrom timezone.Date,
	createdBy int64,
) scheduleSvc.CreateTemplateInput {
	req := parsed.req
	var createdByPtr *int64
	if createdBy > 0 {
		c := createdBy
		createdByPtr = &c
	}
	return scheduleSvc.CreateTemplateInput{
		Name:              req.Name,
		Type:              req.Type,
		Weekdays:          req.Weekdays,
		StartTime:         parsed.startTime,
		EndTime:           parsed.endTime,
		RoomID:            req.RoomID,
		CategoryID:        req.CategoryID,
		MaxParticipants:   parsed.maxParticipants,
		RequiredStaff:     normalizeRequiredStaff(req.RequiredStaff),
		WeekPattern:       parsed.weekPattern,
		CalendarPeriodID:  req.CalendarPeriodID,
		EducationGroupID:  req.EducationGroupID,
		TargetGroupType:   req.TargetGroupType,
		TargetGradeLevel:  req.TargetGradeLevel,
		TargetSchoolClass: req.TargetSchoolClass,
		ListKind:          req.ListKind,
		Notes:             normalizeNotes(req.Notes),
		StudentIDs:        req.StudentIDs,
		StaffIDs:          req.StaffIDs,
		PrimaryStaffID:    req.PrimaryStaffID,
		CreatedBy:         createdByPtr,
		RosterValidFrom:   rosterValidFrom,
		GradeLevelMax:     gradeLevelMax,
	}
}

// renderCreateTemplateError maps a CreateTemplate failure to its HTTP response:
// the client-correctable grade-cap and education-group precheck failures become
// 400, everything else a 500.
func renderCreateTemplateError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case renderTemplateEducationGroupError(w, r, err):
	case renderTemplateTargetGradeLimit(w, r, err):
	default:
		common.RenderError(w, r, common.ErrorInternalServerWrap("create template failed", err))
	}
}

// materializeTemplateWindow best-effort materializes the requested visible
// window so the new template yields instances immediately. A materialization
// failure is logged and never fails the create — the template is already saved.
func (rs *Resource) materializeTemplateWindow(
	ctx context.Context,
	req *createTemplateRequest,
	templateID int64,
	resp *createTemplateResponse,
) {
	if req.MaterializeFrom == nil || req.MaterializeTo == nil || rs.MaterializationService == nil {
		return
	}
	from, ferr := berlinDate(*req.MaterializeFrom)
	to, terr := berlinDate(*req.MaterializeTo)
	if ferr != nil || terr != nil || to.Before(from) {
		return
	}
	mat, mErr := rs.MaterializationService.MaterializeForTenant(
		ctx, from, to, scheduleSvc.MaterializationSourceManual,
	)
	if mErr != nil {
		rs.getLogger().Warn("template create: materialize failed (template still saved)",
			slog.Int64("template_id", templateID),
			slog.String("error", mErr.Error()),
		)
		return
	}
	if mat == nil {
		return
	}
	resp.InstancesCreated = mat.InstancesCreated
	resp.MaterializedFrom = from.Format(dateLayout)
	resp.MaterializedTo = to.Format(dateLayout)
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
