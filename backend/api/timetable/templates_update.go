package timetable

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type updateTemplateRequest struct {
	Name            string        `json:"name"`
	Type            string        `json:"type"`
	Weekdays        []int         `json:"weekdays"`
	StartTime       string        `json:"start_time"`
	EndTime         string        `json:"end_time"`
	RoomID          int64         `json:"room_id"`
	CategoryID      int64         `json:"category_id"`
	PlanningTrackID nullableInt64 `json:"planning_track_id"`
	MaxParticipants *int          `json:"max_participants,omitempty"`
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
	// Offering-source rule (#2137) — see createTemplateRequest. Both fields
	// are presence-aware (#2147 review round 12): an omitted field keeps the
	// template's stored value, only an explicit null (or explicit empty
	// list) clears it. A client predating these fields must not strip a
	// template's dynamic Angebots-Belegung by simply not knowing them.
	SourceCareOfferingIDs nullableInt64Slice `json:"source_care_offering_ids"`
	SourceGradeLevels     nullableIntSlice   `json:"source_grade_levels"`
	// ListKind classifies the template for printable daily lists (#1565);
	// omitted/null/empty clears it.
	ListKind *string `json:"list_kind,omitempty"`
	// Notes is the durable Wochennotiz for the template; omitted/null clears it.
	Notes          *string `json:"notes,omitempty"`
	StudentIDs     []int64 `json:"student_ids,omitempty"`
	StaffIDs       []int64 `json:"staff_ids,omitempty"`
	PrimaryStaffID *int64  `json:"primary_staff_id,omitempty"`
	// WeekdayAssignments overrides the shared roster for individual weekdays
	// (#2129); omitted/empty resets the series to one shared roster.
	WeekdayAssignments []weekdayAssignmentRequest `json:"weekday_assignments,omitempty"`
	// SeriesRosterFrom (#2187, YYYY-MM-DD) extends the saved roster backwards
	// over the template's split series: bounded predecessor segments
	// overlapping [series_roster_from, this segment's valid_from) receive the
	// same roster change. Sent by the editor only when it resolved a capped
	// predecessor to this living segment AND the roster was edited.
	SeriesRosterFrom *string `json:"series_roster_from,omitempty"`
	// SeriesRosterScope* name the people whose membership this edit actually
	// changed. Only they are reconciled on the predecessor segments:
	// student_ids / staff_ids above describe THIS segment and may legitimately
	// differ from a predecessor's roster, so they are not its target set.
	// Without a scope, series_roster_from mirrors nothing.
	SeriesRosterScopeStudentIDs []int64 `json:"series_roster_scope_student_ids,omitempty"`
	SeriesRosterScopeStaffIDs   []int64 `json:"series_roster_scope_staff_ids,omitempty"`
	// SeriesRosterScopeWeekdays narrows the mirroring to the weekdays this
	// edit describes. A series that staffs each weekday separately (#2129) is
	// edited one weekday at a time, so the predecessor's other weekdays must
	// not be reconciled against it. Omitted = every weekday of the segment.
	SeriesRosterScopeWeekdays []int `json:"series_roster_scope_weekdays,omitempty"`
	// SeriesRosterPrimaryChanged marks the Hauptbetreuung as part of this
	// edit. primary_staff_id always names THIS segment's lead, so it may only
	// travel to a predecessor row when the user actually moved it.
	SeriesRosterPrimaryChanged bool `json:"series_roster_primary_changed,omitempty"`
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
	if err := validateWeekdayAssignments(req.WeekdayAssignments, req.Weekdays); err != nil {
		return err
	}
	// The offering-source pair is presence-aware: an omitted source id merges
	// the STORED source in after bind (applyOfferingSourcePresence / the split
	// service). Validating a submitted Jahrgangsfilter against the
	// not-yet-merged nil source here would reject a legitimate filter-only
	// edit of a sourced template (#2147 review round 14), so the pair enters
	// the bind-time validation only when the source id itself was submitted.
	// The merged state is always re-validated service-side
	// (validateOfferingSourceInput → ErrOfferingSourceInvalid → 400).
	sourceGradeLevelsForBind := req.SourceGradeLevels.Value
	if !req.SourceCareOfferingIDs.Set {
		sourceGradeLevelsForBind = nil
	}
	targetType, schoolClass, sourceGradeLevels, err := normalizeTemplateTargetFields(
		req.TargetGroupType, req.TargetGradeLevel, req.TargetSchoolClass, req.EducationGroupID,
		req.SourceCareOfferingIDs.Value, sourceGradeLevelsForBind, req.Targets,
	)
	if err != nil {
		return err
	}
	req.TargetGroupType = targetType
	req.TargetSchoolClass = schoolClass
	if req.SourceCareOfferingIDs.Set {
		req.SourceGradeLevels.Value = sourceGradeLevels
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
	req              *updateTemplateRequest
	startTime        time.Time
	endTime          time.Time
	weekPattern      int
	maxParticipants  int
	seriesRosterFrom *timezone.Date
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
	var seriesRosterFrom *timezone.Date
	if req.SeriesRosterFrom != nil {
		parsedDate, err := timezone.ParseDate(*req.SeriesRosterFrom)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid series_roster_from format, expected YYYY-MM-DD")))
			return nil, false
		}
		seriesRosterFrom = &parsedDate
	}
	return &parsedUpdateTemplate{
		req:              req,
		startTime:        timing.startTime,
		endTime:          timing.endTime,
		weekPattern:      timing.weekPattern,
		maxParticipants:  timing.maxParticipants,
		seriesRosterFrom: seriesRosterFrom,
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
	periodID, ok := templatePeriodIDFromRequest(w, r)
	if !ok {
		return
	}
	if periodID == nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("period_id is required when loading a template for editing")))
		return
	}
	templates, err := rs.loadTemplates(r.Context(), &id, periodID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("load template failed", err))
		return
	}
	if len(templates) == 0 {
		templates, ok = rs.resolveTemplateForRead(w, r, id, periodID)
		if !ok {
			return
		}
	}
	common.Respond(w, r, http.StatusOK, templates[0], "Template retrieved")
}

// templateNotFoundCode is the stable error code carried by every template 404
// so clients can map the message without matching the English text (#2187).
const templateNotFoundCode = "template_not_found"

func renderTemplateNotFound(w http.ResponseWriter, r *http.Request) {
	common.RenderError(w, r, common.ErrorNotFoundWithCode(errors.New("template not found"), templateNotFoundCode))
}

// resolveTemplateForRead retries an empty period-scoped template read against
// the living segment of the same split series (#2187): the Betreuungsplan
// grid hands the editor the template id an occurrence was materialized from,
// which after a split can be a fully capped predecessor that the
// valid_until-filtered read models no longer return. The response then
// carries the living successor plus resolved_from_template_id so the client
// writes to the right template. Renders the error response itself when it
// returns ok=false.
func (rs *Resource) resolveTemplateForRead(
	w http.ResponseWriter,
	r *http.Request,
	requestedID int64,
	periodID *int64,
) ([]templateResponse, bool) {
	resolvedID, changed, err := rs.TimetableData.ResolveLivingTemplateSegment(r.Context(), requestedID)
	if err != nil {
		if errors.Is(err, scheduleSvc.ErrTemplateSeriesFullyEnded) {
			renderTemplateNotFound(w, r)
			return nil, false
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("resolve template series failed", err))
		return nil, false
	}
	if !changed {
		// The template itself is the living segment — the empty read was a
		// genuine miss (e.g. a foreign period_id) and stays a 404.
		renderTemplateNotFound(w, r)
		return nil, false
	}
	templates, err := rs.loadTemplates(r.Context(), &resolvedID, periodID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("load template failed", err))
		return nil, false
	}
	if len(templates) == 0 {
		renderTemplateNotFound(w, r)
		return nil, false
	}
	templates[0].ResolvedFromTemplateID = &requestedID
	return templates, true
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
	// Load the existing recurrence independently of the requested target
	// period. Its schedules still belong to the source period until the update
	// succeeds, so a target-scoped read would incorrectly report 404 while
	// moving a template between periods.
	templates, err := rs.loadTemplates(ctx, &id, nil)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("load template failed", err))
		return
	}
	if len(templates) == 0 {
		if hasWeekendTemplateWeekday(parsed.req.Weekdays) {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("timetable templates can only be scheduled from Monday to Friday")))
			return
		}
		renderTemplateNotFound(w, r)
		return
	}
	if err := validateLegacyTemplateWorkdays(templates[0].Schedules, parsed.req.Weekdays); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	applyOfferingSourcePresence(parsed.req, templates[0])
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
	templates, err = rs.loadTemplates(ctx, &id, parsed.req.CalendarPeriodID)
	if err != nil || len(templates) == 0 {
		common.RenderError(w, r, common.ErrorInternalServerWrap("reload template failed", err))
		return
	}
	common.Respond(w, r, http.StatusOK, templates[0], "Template updated")
}

// applyOfferingSourcePresence resolves the presence-aware offering-source
// fields against the stored template (#2147 review round 12): an omitted
// field keeps the stored value — a client sending the pre-#2137 full update
// body must not silently strip the dynamic Angebots-Belegung — while an
// explicit null clears it. Carrying only applies while the update keeps the
// 'angebot' Zielgruppe; every other type cannot hold a source (DB CHECK), so
// a type switch drops the rule like a split away from 'angebot' does. The
// merged result still passes the service validation, so a carried source
// conflicts loudly (400) with submitted student_ids or weekday_assignments
// instead of being half-applied.
func applyOfferingSourcePresence(req *updateTemplateRequest, existing templateResponse) {
	if req.TargetGroupType != activitiesModel.TargetGroupTypeAngebot {
		return
	}
	if !req.SourceCareOfferingIDs.Set && len(existing.SourceCareOfferingIDs) > 0 {
		req.SourceCareOfferingIDs.Value = slices.Clone(existing.SourceCareOfferingIDs)
	}
	if req.SourceGradeLevels.Set {
		return
	}
	// The filter follows the source: a kept or carried source keeps the stored
	// filter; a source cleared by explicit null takes the filter down with it.
	if len(req.SourceCareOfferingIDs.Value) > 0 {
		req.SourceGradeLevels.Value = slices.Clone(existing.SourceGradeLevels)
	} else {
		req.SourceGradeLevels.Value = nil
	}
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
			Name:                    req.Name,
			Type:                    req.Type,
			CategoryID:              req.CategoryID,
			PlanningTrackID:         req.PlanningTrackID.Value,
			PlanningTrackIDProvided: req.PlanningTrackID.Set,
			RoomID:                  req.RoomID,
			EducationGroupID:        req.EducationGroupID,
			MaxParticipants:         parsed.maxParticipants,
			RequiredStaff:           normalizeRequiredStaff(req.RequiredStaff),
			CalendarPeriodID:        req.CalendarPeriodID,
			TargetGroupType:         req.TargetGroupType,
			TargetGradeLevel:        req.TargetGradeLevel,
			TargetSchoolClass:       req.TargetSchoolClass,
			SourceCareOfferingIDs:   req.SourceCareOfferingIDs.Value,
			SourceGradeLevels:       req.SourceGradeLevels.Value,
			ListKind:                req.ListKind,
			Notes:                   normalizeNotes(req.Notes),
		},
		Weekdays:           req.Weekdays,
		TimeframeID:        timeframeID,
		WeekPattern:        parsed.weekPattern,
		CalendarPeriodID:   req.CalendarPeriodID,
		RosterValidFrom:    rosterValidFrom,
		StudentIDs:         req.StudentIDs,
		StaffIDs:           req.StaffIDs,
		PrimaryStaffID:     req.PrimaryStaffID,
		Targets:            targetModels(req.Targets),
		WeekdayAssignments: toServiceWeekdayAssignments(req.WeekdayAssignments),
		GradeLevelMax:      gradeLevelMax,
		SeriesRosterFrom:   parsed.seriesRosterFrom,

		SeriesRosterScopeStudentIDs: req.SeriesRosterScopeStudentIDs,
		SeriesRosterScopeStaffIDs:   req.SeriesRosterScopeStaffIDs,
		SeriesRosterScopeWeekdays:   req.SeriesRosterScopeWeekdays,
		SeriesRosterPrimaryChanged:  req.SeriesRosterPrimaryChanged,
	}
}

// renderUpdateTemplateError maps an UpdateTemplate failure to its HTTP response.
// A capped/archived segment surfaces as the same 404 as the preflight lookup;
// education-group failures and the care-offering, roster-rebase, and grade-cap
// conflicts each carry their own status and code; everything else is a 500.
func renderUpdateTemplateError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, scheduleSvc.ErrTemplateSegmentNotEditable):
		renderTemplateNotFound(w, r)
	case errors.Is(err, scheduleSvc.ErrCategoryNotAssignable):
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("category is archived or unavailable")))
	case errors.Is(err, scheduleSvc.ErrPlanningTrackNotFound), errors.Is(err, scheduleSvc.ErrPlanningTrackArchived):
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("planning track is archived or unavailable")))
	case errors.Is(err, scheduleSvc.ErrTemplateWeekendWeekday):
		common.RenderError(w, r, common.ErrorInvalidRequest(scheduleSvc.ErrTemplateWeekendWeekday))
	case errors.Is(err, scheduleSvc.ErrOfferingSourceInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case renderTemplateEducationGroupError(w, r, err):
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
		renderTemplateNotFound(w, r)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]any{"id": id}, "Template archived")
}

func templateIDFromRequest(w http.ResponseWriter, r *http.Request) (int64, bool) {
	return common.ParsePositiveInt64IDWithError(w, r, "id", "invalid template id")
}
