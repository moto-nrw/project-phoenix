package enrollment

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

// PhaseResponse is the wire shape returned to admin UIs. Int64 IDs are
// stringified to keep the existing int64-as-string convention; dates
// are formatted YYYY-MM-DD; timestamps RFC3339. The frontend reverses
// the formatting when binding the edit form.
type PhaseResponse struct {
	ID                        string  `json:"id"`
	Name                      string  `json:"name"`
	Kind                      string  `json:"kind"`
	ServiceStartDate          string  `json:"service_start_date"`
	ServiceEndDate            string  `json:"service_end_date"`
	EnrollmentOpenAt          *string `json:"enrollment_open_at,omitempty"`
	EnrollmentCloseAt         *string `json:"enrollment_close_at,omitempty"`
	FormSchemaID              *string `json:"form_schema_id,omitempty"`
	CalendarPeriodID          *string `json:"calendar_period_id,omitempty"`
	ShowStatusReasonToParent  bool    `json:"show_status_reason_to_parent"`
	CareOverflowMode          string  `json:"care_overflow_mode"`
	CareOfferingSelectionMode string  `json:"care_offering_selection_mode"`
	IsActive                  bool    `json:"is_active"`
	// Rollover columns (migration 1.15.61) — emitted so the admin UI
	// can distinguish rollover phases from fresh ones and surface the
	// review-queue link only for rolled-forward phases.
	RolloverSourcePhaseID *string `json:"rollover_source_phase_id,omitempty"`
	RolloverMode          *string `json:"rollover_mode,omitempty"`
	RolloverAutoApprove   bool    `json:"rollover_auto_approve"`
	RolloverDeadline      *string `json:"rollover_deadline,omitempty"`
	RolloverBumpsGrade    bool    `json:"rollover_bumps_grade"`
	// Concrete-class config (migration 1.15.171, issue #1833). The pick
	// list the public form offers for grade >= 2, and whether choosing is
	// mandatory. Only meaningful when the tenant setting
	// enrollment.collect_school_class is on.
	AvailableSchoolClasses []string `json:"available_school_classes"`
	RequireSchoolClass     bool     `json:"require_school_class"`
	CreatedAt              string   `json:"created_at"`
	UpdatedAt              string   `json:"updated_at"`
}

func toPhaseResponse(p *enrollmentModels.Phase) PhaseResponse {
	resp := PhaseResponse{
		ID:                        strconv.FormatInt(p.ID, 10),
		Name:                      p.Name,
		Kind:                      p.Kind,
		ServiceStartDate:          p.ServiceStartDate.String(),
		ServiceEndDate:            p.ServiceEndDate.String(),
		ShowStatusReasonToParent:  p.ShowStatusReasonToParent,
		CareOverflowMode:          p.CareOverflowMode,
		CareOfferingSelectionMode: p.CareOfferingSelectionMode,
		IsActive:                  p.IsActive,
		CreatedAt:                 p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:                 p.UpdatedAt.Format(time.RFC3339),
	}
	if p.EnrollmentOpenAt != nil {
		s := p.EnrollmentOpenAt.Format(time.RFC3339)
		resp.EnrollmentOpenAt = &s
	}
	if p.EnrollmentCloseAt != nil {
		s := p.EnrollmentCloseAt.Format(time.RFC3339)
		resp.EnrollmentCloseAt = &s
	}
	if p.FormSchemaID != nil {
		s := strconv.FormatInt(*p.FormSchemaID, 10)
		resp.FormSchemaID = &s
	}
	if p.CalendarPeriodID != nil {
		s := strconv.FormatInt(*p.CalendarPeriodID, 10)
		resp.CalendarPeriodID = &s
	}
	if p.RolloverSourcePhaseID != nil {
		s := strconv.FormatInt(*p.RolloverSourcePhaseID, 10)
		resp.RolloverSourcePhaseID = &s
	}
	if p.RolloverMode != nil {
		mode := *p.RolloverMode
		resp.RolloverMode = &mode
	}
	resp.RolloverAutoApprove = p.RolloverAutoApprove
	if p.RolloverDeadline != nil {
		s := p.RolloverDeadline.Format(time.RFC3339)
		resp.RolloverDeadline = &s
	}
	resp.RolloverBumpsGrade = p.RolloverBumpsGrade
	resp.AvailableSchoolClasses = p.AvailableSchoolClasses
	if resp.AvailableSchoolClasses == nil {
		// Emit [] rather than null so the frontend list binding is stable.
		resp.AvailableSchoolClasses = []string{}
	}
	resp.RequireSchoolClass = p.RequireSchoolClass
	return resp
}

// PhaseRequest is the wire shape POST + PUT accept. Dates are
// YYYY-MM-DD strings; window timestamps are RFC3339 (or omitted for
// "unbounded"). FormSchemaID is the string ID of an existing schema
// row, or omitted/empty for "no custom fields" (the parent form
// renders core fields only).
type PhaseRequest struct {
	Name                      string  `json:"name"`
	Kind                      string  `json:"kind"`
	ServiceStartDate          string  `json:"service_start_date"`
	ServiceEndDate            string  `json:"service_end_date"`
	EnrollmentOpenAt          *string `json:"enrollment_open_at,omitempty"`
	EnrollmentCloseAt         *string `json:"enrollment_close_at,omitempty"`
	FormSchemaID              *string `json:"form_schema_id,omitempty"`
	CalendarPeriodID          *string `json:"calendar_period_id,omitempty"`
	ShowStatusReasonToParent  bool    `json:"show_status_reason_to_parent"`
	CareOverflowMode          string  `json:"care_overflow_mode"`
	CareOfferingSelectionMode string  `json:"care_offering_selection_mode"`
	IsActive                  bool    `json:"is_active"`
	// Concrete-class config (issue #1833) is optional on the wire so a
	// stale client that predates the feature omits it rather than sending
	// zero values. Pointers distinguish "field omitted" (nil -> preserve
	// existing on update / default on create) from "explicitly cleared"
	// ([] / false). A non-pointer would make every omission look like an
	// explicit wipe, silently deleting an admin's class list. See
	// createPhase / updatePhase for how each side resolves nil.
	AvailableSchoolClasses *[]string `json:"available_school_classes,omitempty"`
	RequireSchoolClass     *bool     `json:"require_school_class,omitempty"`

	calendarPeriodIDPresent bool
}

func (req *PhaseRequest) Bind(_ *http.Request) error { return nil }

func (req *PhaseRequest) UnmarshalJSON(data []byte) error {
	type phaseRequestAlias PhaseRequest
	var alias phaseRequestAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*req = PhaseRequest(alias)
	_, req.calendarPeriodIDPresent = raw["calendar_period_id"]
	return nil
}

// toModel maps the wire shape onto a Phase model. Date parsing
// failures bubble back as 400 from the handler — kept here so the
// parsing logic is in one place.
func (req *PhaseRequest) toModel(existingID int64) (*enrollmentModels.Phase, error) {
	startDate, err := timezone.ParseDate(req.ServiceStartDate)
	if err != nil {
		return nil, errors.New("service_start_date must be YYYY-MM-DD")
	}
	endDate, err := timezone.ParseDate(req.ServiceEndDate)
	if err != nil {
		return nil, errors.New("service_end_date must be YYYY-MM-DD")
	}

	p := &enrollmentModels.Phase{
		Name:                      req.Name,
		Kind:                      req.Kind,
		ServiceStartDate:          startDate,
		ServiceEndDate:            endDate,
		ShowStatusReasonToParent:  req.ShowStatusReasonToParent,
		CareOverflowMode:          req.CareOverflowMode,
		CareOfferingSelectionMode: req.CareOfferingSelectionMode,
		IsActive:                  req.IsActive,
	}
	// Class config: a provided value (even []/false) is applied verbatim;
	// an omitted value (nil pointer) leaves the zero value here. On create
	// that means an empty list / not-required (matching a fresh phase); on
	// update the handler re-hydrates the omitted field from the stored
	// phase so a partial update never wipes it. See updatePhase.
	if req.AvailableSchoolClasses != nil {
		p.AvailableSchoolClasses = normalizeSchoolClasses(*req.AvailableSchoolClasses)
	} else {
		p.AvailableSchoolClasses = []string{}
	}
	if req.RequireSchoolClass != nil {
		p.RequireSchoolClass = *req.RequireSchoolClass
	}
	openAt, err := parseOptionalRFC3339(req.EnrollmentOpenAt, "enrollment_open_at must be RFC3339")
	if err != nil {
		return nil, err
	}
	p.EnrollmentOpenAt = openAt
	closeAt, err := parseOptionalRFC3339(req.EnrollmentCloseAt, "enrollment_close_at must be RFC3339")
	if err != nil {
		return nil, err
	}
	p.EnrollmentCloseAt = closeAt
	schemaID, err := parseOptionalPositiveID(req.FormSchemaID, "form_schema_id must be a positive integer string")
	if err != nil {
		return nil, err
	}
	p.FormSchemaID = schemaID
	periodID, err := parseOptionalPositiveID(req.CalendarPeriodID, "calendar_period_id must be a positive integer string")
	if err != nil {
		return nil, err
	}
	p.CalendarPeriodID = periodID
	p.ID = existingID
	return p, nil
}

// parseOptionalRFC3339 parses an optional RFC3339 timestamp. A nil or empty
// value yields (nil, nil); a malformed value yields errMsg.
func parseOptionalRFC3339(value *string, errMsg string) (*time.Time, error) {
	if value == nil || *value == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, *value)
	if err != nil {
		return nil, errors.New(errMsg)
	}
	return &t, nil
}

// parseOptionalPositiveID parses an optional positive int64 ID string. A nil
// or empty value yields (nil, nil); a non-positive or malformed value yields
// errMsg.
func parseOptionalPositiveID(value *string, errMsg string) (*int64, error) {
	if value == nil || *value == "" {
		return nil, nil
	}
	id, err := strconv.ParseInt(*value, 10, 64)
	if err != nil || id <= 0 {
		return nil, errors.New(errMsg)
	}
	return &id, nil
}

// normalizeSchoolClasses trims each entry, drops empties, and dedups
// case-sensitively while preserving admin-entered order. Returns a
// non-nil empty slice so the jsonb column stores '[]' rather than null.
func normalizeSchoolClasses(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, c := range in {
		t := strings.TrimSpace(c)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

func (rs *Resource) listPhases(w http.ResponseWriter, r *http.Request) {
	if rs.PhaseService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("phase service not configured")))
		return
	}

	var phases []*enrollmentModels.Phase
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		list, listErr := rs.PhaseService.List(ctx)
		phases = list
		return listErr
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	out := make([]PhaseResponse, 0, len(phases))
	for _, p := range phases {
		out = append(out, toPhaseResponse(p))
	}
	common.Respond(w, r, http.StatusOK, out, "Phases retrieved")
}

func (rs *Resource) getPhase(w http.ResponseWriter, r *http.Request) {
	if rs.PhaseService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("phase service not configured")))
		return
	}
	id, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid id")
	if !ok {
		return
	}

	var phase *enrollmentModels.Phase
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		p, e := rs.PhaseService.GetByID(ctx, id)
		phase = p
		return e
	})
	if err != nil {
		if errors.Is(err, enrollmentService.ErrPhaseNotFound) {
			common.RenderError(w, r, common.ErrorNotFound(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, toPhaseResponse(phase), "Phase retrieved")
}

func (rs *Resource) createPhase(w http.ResponseWriter, r *http.Request) {
	if rs.PhaseService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("phase service not configured")))
		return
	}
	req := &PhaseRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	model, err := req.toModel(0)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	var created *enrollmentModels.Phase
	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		p, e := rs.PhaseService.Create(ctx, model)
		created = p
		return e
	})
	if err != nil {
		common.RenderError(w, r, phaseWriteErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusCreated, toPhaseResponse(created), "Phase created")
}

// updateWithRefetch is the shared decode -> update -> refetch -> respond body
// of updatePhase and updateCareOffering. decode binds the request and maps it
// to the update model; both bind and mapping failures render as 400. updateErr
// maps an update() failure to its HTTP error (sentinel-based status dispatch
// lives at the call site).
func updateWithRefetch[M, E any](rs *Resource, w http.ResponseWriter, r *http.Request,
	serviceMissing bool, missingMsg string,
	decode func(r *http.Request, id int64) (M, error),
	update func(ctx context.Context, model M) error,
	refetch func(ctx context.Context, id int64) (E, error),
	toResponse func(E) any, successMsg string,
	updateErr func(error) render.Renderer,
) {
	if serviceMissing {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New(missingMsg)))
		return
	}
	id, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid id")
	if !ok {
		return
	}
	model, err := decode(r, id)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		return update(ctx, model)
	})
	if err != nil {
		common.RenderError(w, r, updateErr(err))
		return
	}

	// Refetch so the response carries DB-managed timestamps + applied
	// defaults (e.g. care_overflow_mode normalisation in Validate).
	var refreshed E
	if fetchErr := rs.runInTenantTx(r, func(ctx context.Context) error {
		e, err := refetch(ctx, id)
		refreshed = e
		return err
	}); fetchErr != nil {
		common.RenderError(w, r, common.ErrorInternalServer(fetchErr))
		return
	}
	common.Respond(w, r, http.StatusOK, toResponse(refreshed), successMsg)
}

// phaseWriteErrorRenderer maps phase create/update failures onto their HTTP
// status: duplicate name -> 409, missing phase -> 404, validation -> 400,
// everything else -> 500.
func phaseWriteErrorRenderer(err error) render.Renderer {
	switch {
	case errors.Is(err, enrollmentService.ErrPhaseDuplicateName):
		return common.ErrorConflict(err)
	case errors.Is(err, enrollmentService.ErrPhaseCareOfferingConflict):
		return common.ErrorConflict(err)
	case errors.Is(err, enrollmentService.ErrPhaseNotFound):
		return common.ErrorNotFound(err)
	case errors.Is(err, enrollmentService.ErrInvalidPhase):
		return common.ErrorInvalidRequest(err)
	default:
		return common.ErrorInternalServer(err)
	}
}

func (rs *Resource) updatePhase(w http.ResponseWriter, r *http.Request) {
	req := &PhaseRequest{}
	updateWithRefetch(rs, w, r, rs.PhaseService == nil, "phase service not configured",
		func(r *http.Request, id int64) (*enrollmentModels.Phase, error) {
			if err := render.Bind(r, req); err != nil {
				return nil, err
			}
			return req.toModel(id)
		},
		func(ctx context.Context, model *enrollmentModels.Phase) error {
			// A PUT without calendar_period_id keeps the stored link; only an
			// explicit null (or value) changes it. The fetch also surfaces
			// ErrPhaseNotFound before the update runs.
			existing, getErr := rs.PhaseService.GetByID(ctx, model.ID)
			if getErr != nil {
				return getErr
			}
			if !req.calendarPeriodIDPresent {
				model.CalendarPeriodID = existing.CalendarPeriodID
			}
			// A stale client (pre-#1833) omits the concrete-class fields
			// entirely. Update replaces the whole row, so without this a
			// partial update would silently wipe the admin's class pick
			// list / mandatory toggle. Re-hydrate any omitted field from
			// the stored phase before persisting.
			if req.AvailableSchoolClasses == nil {
				model.AvailableSchoolClasses = existing.AvailableSchoolClasses
			}
			if req.RequireSchoolClass == nil {
				model.RequireSchoolClass = existing.RequireSchoolClass
			}
			return rs.PhaseService.Update(ctx, model)
		},
		func(ctx context.Context, id int64) (*enrollmentModels.Phase, error) {
			return rs.PhaseService.GetByID(ctx, id)
		},
		func(p *enrollmentModels.Phase) any { return toPhaseResponse(p) },
		"Phase updated",
		phaseWriteErrorRenderer)
}

func (rs *Resource) deletePhase(w http.ResponseWriter, r *http.Request) {
	if rs.PhaseService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("phase service not configured")))
		return
	}
	id, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid id")
	if !ok {
		return
	}

	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		return rs.PhaseService.Delete(ctx, id)
	})
	if err != nil {
		if errors.Is(err, enrollmentService.ErrPhaseNotFound) {
			common.RenderError(w, r, common.ErrorNotFound(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.RespondNoContent(w, r)
}

// PhaseDeleteImpactResponse is the delete-confirmation preview the admin
// UI fetches before deleting. Requests + CareOfferings will be
// permanently removed; StudentsKept survive the delete.
type PhaseDeleteImpactResponse struct {
	Requests      int `json:"requests"`
	CareOfferings int `json:"care_offerings"`
	StudentsKept  int `json:"students_kept"`
}

func (rs *Resource) getPhaseDeleteImpact(w http.ResponseWriter, r *http.Request) {
	if rs.PhaseService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("phase service not configured")))
		return
	}
	id, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid id")
	if !ok {
		return
	}

	var impact *enrollmentService.PhaseDeleteImpact
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		i, e := rs.PhaseService.DeleteImpact(ctx, id)
		impact = i
		return e
	})
	if err != nil {
		if errors.Is(err, enrollmentService.ErrPhaseNotFound) {
			common.RenderError(w, r, common.ErrorNotFound(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, PhaseDeleteImpactResponse{
		Requests:      impact.Requests,
		CareOfferings: impact.CareOfferings,
		StudentsKept:  impact.StudentsKept,
	}, "Phase delete impact")
}
