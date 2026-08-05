// Package timetable — POST /api/timetable/templates/{id}/split handler (WP-B3).
//
// "Dieser und alle folgenden": caps the old template's schedules and rosters
// at the effective date (exclusive), creates a successor template from the
// submitted fields, deletes the old template's still-planned future instances
// and optionally re-materializes the window. Admin-only (SchedulesManage),
// runs inside the tenant transaction so the split is all-or-nothing.
package timetable

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// nullableInt distinguishes an omitted JSON field (Set=false) from an explicit
// null (Set=true, Value=nil) or a concrete value (Set=true, Value=&n). The
// stdlib `*int` collapses "omitted" and "null" to the same nil, which makes it
// impossible to tell "leave as-is/inherit" from "clear this override" — the
// split flow needs both (#1839).
type nullableInt struct {
	Set   bool
	Value *int
}

type nullableInt64 struct {
	Set   bool
	Value *int64
}

func (n *nullableInt64) UnmarshalJSON(b []byte) error {
	n.Set = true
	if string(b) == "null" {
		n.Value = nil
		return nil
	}
	return json.Unmarshal(b, &n.Value)
}

func (n *nullableInt) UnmarshalJSON(b []byte) error {
	n.Set = true
	if string(b) == "null" {
		n.Value = nil
		return nil
	}
	return json.Unmarshal(b, &n.Value)
}

// nullableIntSlice is the []int member of the tri-state family: it tells an
// omitted source_grade_levels ("keep the stored Jahrgangsfilter") apart from
// an explicit null or empty list ("clear the filter") on the template PUT
// (#2147 review round 12).
type nullableIntSlice struct {
	Set   bool
	Value []int
}

func (n *nullableIntSlice) UnmarshalJSON(b []byte) error {
	n.Set = true
	if string(b) == "null" {
		n.Value = nil
		return nil
	}
	return json.Unmarshal(b, &n.Value)
}

// nullableStr is the string analogue of nullableInt: it tells an omitted note
// ("inherit the source Wochennotiz") apart from an explicit null ("clear the
// series note") for the split flow (#1837 follow-up). (Named nullableStr to
// avoid colliding with the RawMessage-based nullableString in
// instance_students.go, which is a different tri-state mechanism.)
type nullableStr struct {
	Set   bool
	Value *string
}

func (n *nullableStr) UnmarshalJSON(b []byte) error {
	n.Set = true
	if string(b) == "null" {
		n.Value = nil
		return nil
	}
	return json.Unmarshal(b, &n.Value)
}

// splitTemplateRequest is the update-template body plus the split controls.
// effective_date is required (YYYY-MM-DD); materialize_from/materialize_to
// are optional and mirror the create-template materialization window.
//
// RequiredStaff shadows updateTemplateRequest's `*int` field (a directly
// declared field at depth 0 dominates the embedded one at depth 1 for JSON):
// the split must be able to CLEAR an existing override, which needs the
// omitted-vs-null distinction the presence-aware type provides.
type splitTemplateRequest struct {
	updateTemplateRequest
	RequiredStaff nullableInt `json:"required_staff"`
	// Notes shadows updateTemplateRequest's `*string` for the same reason as
	// RequiredStaff: the split must tell "inherit the source note" (omitted)
	// from "clear the series note" (null).
	Notes nullableStr `json:"notes"`
	// ListKind shadows updateTemplateRequest's `*string` (#1565): omitted
	// inherits the source template's Listenart, null/empty clears it. Without
	// the tri-state a plain "this and following" edit would wipe the
	// classification and the successor's instances would vanish from their
	// automatic daily list.
	ListKind        nullableStr `json:"list_kind"`
	EffectiveDate   string      `json:"effective_date"`
	MaterializeFrom *string     `json:"materialize_from,omitempty"` // YYYY-MM-DD
	MaterializeTo   *string     `json:"materialize_to,omitempty"`   // YYYY-MM-DD
}

// Bind reuses the update-template presence checks and adds the split's own
// required field. Format/business validation happens in handler + service.
func (req *splitTemplateRequest) Bind(r *http.Request) error {
	if err := req.updateTemplateRequest.Bind(r); err != nil {
		return err
	}
	if err := validateTemplateWorkdays(req.Weekdays); err != nil {
		return err
	}
	if req.EffectiveDate == "" {
		return errors.New("effective_date is required (YYYY-MM-DD)")
	}
	// req.Notes (nullableStr) shadows the embedded updateTemplateRequest.Notes,
	// so the create/update length guard in the embedded Bind never sees the
	// split note. Enforce the same 2000-char limit here (#1837 follow-up).
	if req.Notes.Set && req.Notes.Value != nil && len(*req.Notes.Value) > 2000 {
		return errors.New("notes cannot exceed 2000 characters")
	}
	// req.ListKind (nullableStr) shadows the embedded field the same way, so
	// the create/update normalization never sees the split value. Apply the
	// identical trim + validity check here (#1565).
	if req.ListKind.Set {
		normalized, err := normalizeTemplateListKind(req.ListKind.Value)
		if err != nil {
			return err
		}
		req.ListKind.Value = normalized
	}
	return nil
}

// splitTemplateResponse is the wire contract the planner UI consumes.
// instances_created comes from the optional materialization run (0 if none).
type splitTemplateResponse struct {
	OldTemplateID    int64   `json:"old_template_id"`
	NewTemplateID    int64   `json:"new_template_id"`
	ScheduleIDs      []int64 `json:"schedule_ids"`
	DeletedInstances int     `json:"deleted_instances"`
	InstancesCreated int     `json:"instances_created"`
}

// ErrCodeTemplateCareOfferingConflict is shared by split, update, end, and
// archive so the planner can show one localized resolution message for the
// same cross-domain invariant.
const ErrCodeTemplateCareOfferingConflict = "timetable.template_care_offering_conflict"

const ErrCodeTemplateRosterRebaseConflict = "timetable.template_roster_rebase_conflict"

func renderTemplateCareOfferingConflict(w http.ResponseWriter, r *http.Request, err error) bool {
	if !errors.Is(err, scheduleSvc.ErrTemplateCareOfferingConflict) {
		return false
	}
	common.RenderError(w, r, common.ErrorInvalidRequestWithCode(
		//nolint:staticcheck // ST1005: user-facing German message
		errors.New("Die Änderung ist nicht möglich, weil dadurch ein verknüpftes Betreuungsangebot ungültig würde. Bitte passen Sie zuerst das Angebot oder dessen Stundenplan-Verknüpfung an."),
		ErrCodeTemplateCareOfferingConflict,
	))
	return true
}

func renderTemplateRosterRebaseConflict(w http.ResponseWriter, r *http.Request, err error) bool {
	if !errors.Is(err, scheduleSvc.ErrTemplateRosterRebaseConflict) {
		return false
	}
	tenant.MarkRollback(r.Context())
	common.RenderError(w, r, common.ErrorConflictWithCode(
		//nolint:staticcheck // ST1005: user-facing German message
		errors.New("Die Zeitraumänderung würde geschützte Kinderzuordnungen zusammenführen. Bitte bereinigen Sie zuerst die betroffenen Zuordnungen."),
		ErrCodeTemplateRosterRebaseConflict,
	))
	return true
}

// splitTemplate handles POST /api/timetable/templates/{id}/split.
func (rs *Resource) splitTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := templateIDFromRequest(w, r)
	if !ok {
		return
	}
	if rs.TemplateSplitService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("template split service not wired")))
		return
	}
	req := &splitTemplateRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	in, err := buildTemplateSplitInput(id, req)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	gradeLevelMax, err := rs.resolveTemplateGradeLevelMax(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap(
			"resolve template grade level limit failed", err))
		return
	}
	in.GradeLevelMax = gradeLevelMax
	in.ActorAccountID = jwt.ActorAccountIDFromCtx(r.Context())
	// Same tenant-scoped period check the create/update handlers run; the
	// returned roster start date is unused — the split anchors rosters on
	// the effective date instead.
	if _, err := rs.templateRosterValidFrom(r.Context(), req.CalendarPeriodID, nil); err != nil {
		renderTemplatePeriodLookupError(w, r, err)
		return
	}
	if err := rs.TimetableData.ValidateTemplateEducationGroup(r.Context(), req.EducationGroupID); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	for _, target := range req.Targets {
		if err := rs.TimetableData.ValidateTemplateEducationGroup(r.Context(), target.EducationGroupID); err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
	}

	result, err := rs.TemplateSplitService.Split(r.Context(), in)
	if err != nil {
		renderTemplateSplitError(w, r, err)
		return
	}

	resp := splitTemplateResponse{
		OldTemplateID:    result.OldTemplateID,
		NewTemplateID:    result.NewTemplateID,
		ScheduleIDs:      result.NewScheduleIDs,
		DeletedInstances: result.DeletedInstances,
	}
	if result.Materialization != nil {
		resp.InstancesCreated = result.Materialization.InstancesCreated
	}
	common.Respond(w, r, http.StatusOK, resp, "Template split")
}

// buildTemplateSplitInput parses the wire shape into the service input.
// Reuses the shared HH:MM / YYYY-MM-DD parse helpers.
func buildTemplateSplitInput(id int64, req *splitTemplateRequest) (scheduleSvc.TemplateSplitInput, error) {
	startTime, err := parseClockTime(req.StartTime)
	if err != nil {
		return scheduleSvc.TemplateSplitInput{}, errors.New("invalid start_time format, expected HH:MM")
	}
	endTime, err := parseClockTime(req.EndTime)
	if err != nil {
		return scheduleSvc.TemplateSplitInput{}, errors.New("invalid end_time format, expected HH:MM")
	}
	effectiveDate, err := timezone.ParseDate(req.EffectiveDate)
	if err != nil {
		return scheduleSvc.TemplateSplitInput{}, errors.New("invalid effective_date format, expected YYYY-MM-DD")
	}
	materializeFrom, err := parseOptionalSplitDate(req.MaterializeFrom)
	if err != nil {
		return scheduleSvc.TemplateSplitInput{}, errors.New("invalid materialize_from format, expected YYYY-MM-DD")
	}
	materializeTo, err := parseOptionalSplitDate(req.MaterializeTo)
	if err != nil {
		return scheduleSvc.TemplateSplitInput{}, errors.New("invalid materialize_to format, expected YYYY-MM-DD")
	}

	return scheduleSvc.TemplateSplitInput{
		TemplateID:              id,
		EffectiveDate:           effectiveDate,
		Name:                    req.Name,
		Type:                    req.Type,
		Weekdays:                req.Weekdays,
		StartTime:               startTime,
		EndTime:                 endTime,
		RoomID:                  req.RoomID,
		CategoryID:              req.CategoryID,
		PlanningTrackID:         req.PlanningTrackID.Value,
		PlanningTrackIDProvided: req.PlanningTrackID.Set,
		MaxParticipants:         req.MaxParticipants,
		// Three-state: only when required_staff is present in the body do we
		// touch the successor's override — a null clears it (derive), an
		// omitted field inherits the source template's value.
		RequiredStaff:         normalizeRequiredStaff(req.RequiredStaff.Value),
		RequiredStaffProvided: req.RequiredStaff.Set,
		// Same three-state contract for the Wochennotiz: present -> set/clear,
		// omitted -> inherit the source template's note.
		Notes:             normalizeNotes(req.Notes.Value),
		NotesProvided:     req.Notes.Set,
		ListKind:          req.ListKind.Value,
		ListKindProvided:  req.ListKind.Set,
		WeekPattern:       req.WeekPattern,
		CalendarPeriodID:  req.CalendarPeriodID,
		EducationGroupID:  req.EducationGroupID,
		TargetGroupType:   req.TargetGroupType,
		TargetGradeLevel:  req.TargetGradeLevel,
		TargetSchoolClass: req.TargetSchoolClass,
		// Offering-source rule (#2137): same presence contract as the template
		// PUT — omitted fields inherit the old template's source and filter,
		// submitted fields are authoritative (#2147 review round 14). Without
		// the pass-through an editor save that changes the Quelle or the
		// Jahrgangsfilter and then picks the "following" scope would silently
		// keep the old rule on the successor.
		SourceCareOfferingID:         req.SourceCareOfferingID.Value,
		SourceCareOfferingIDProvided: req.SourceCareOfferingID.Set,
		SourceGradeLevels:            req.SourceGradeLevels.Value,
		SourceGradeLevelsProvided:    req.SourceGradeLevels.Set,
		Targets:                      targetModels(req.Targets),
		StudentIDs:                   req.StudentIDs,
		StaffIDs:                     req.StaffIDs,
		PrimaryStaffID:               req.PrimaryStaffID,
		// Per-weekday roster deviations follow the successor (#2129); the
		// embedded updateTemplateRequest.Bind already validated them against
		// the submitted weekdays.
		WeekdayAssignments: toServiceWeekdayAssignments(req.WeekdayAssignments),
		MaterializeFrom:    materializeFrom,
		MaterializeTo:      materializeTo,
	}, nil
}

func parseOptionalSplitDate(raw *string) (*timezone.Date, error) {
	if raw == nil {
		return nil, nil
	}
	d, err := timezone.ParseDate(*raw)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// renderTemplateSplitError maps the service sentinels onto HTTP statuses.
func renderTemplateSplitError(w http.ResponseWriter, r *http.Request, err error) {
	if renderTemplateCareOfferingConflict(w, r, err) {
		return
	}
	if renderTemplateRosterRebaseConflict(w, r, err) {
		return
	}
	switch {
	case errors.Is(err, scheduleSvc.ErrSplitTemplateNotFound):
		common.RenderError(w, r, common.ErrorNotFound(errors.New("template not found")))
	case errors.Is(err, scheduleSvc.ErrCategoryNotAssignable):
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("category is archived or unavailable")))
	case errors.Is(err, scheduleSvc.ErrPlanningTrackNotFound), errors.Is(err, scheduleSvc.ErrPlanningTrackArchived):
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("planning track is archived or unavailable")))
	case errors.Is(err, scheduleSvc.ErrSplitInvalidInput):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.Is(err, scheduleSvc.ErrOfferingSourceInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServerWrap("split template failed", err))
	}
}
