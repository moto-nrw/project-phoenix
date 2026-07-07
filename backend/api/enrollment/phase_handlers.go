package enrollment

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
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
	// Concrete-class config (migration 1.15.167, issue #1833). The pick
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
}

func (req *PhaseRequest) Bind(_ *http.Request) error { return nil }

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
	if req.EnrollmentOpenAt != nil && *req.EnrollmentOpenAt != "" {
		t, parseErr := time.Parse(time.RFC3339, *req.EnrollmentOpenAt)
		if parseErr != nil {
			return nil, errors.New("enrollment_open_at must be RFC3339")
		}
		p.EnrollmentOpenAt = &t
	}
	if req.EnrollmentCloseAt != nil && *req.EnrollmentCloseAt != "" {
		t, parseErr := time.Parse(time.RFC3339, *req.EnrollmentCloseAt)
		if parseErr != nil {
			return nil, errors.New("enrollment_close_at must be RFC3339")
		}
		p.EnrollmentCloseAt = &t
	}
	if req.FormSchemaID != nil && *req.FormSchemaID != "" {
		id, parseErr := strconv.ParseInt(*req.FormSchemaID, 10, 64)
		if parseErr != nil || id <= 0 {
			return nil, errors.New("form_schema_id must be a positive integer string")
		}
		p.FormSchemaID = &id
	}
	p.ID = existingID
	return p, nil
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
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid id")))
		return
	}

	var phase *enrollmentModels.Phase
	err = rs.runInTenantTx(r, func(ctx context.Context) error {
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
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	common.Respond(w, r, http.StatusCreated, toPhaseResponse(created), "Phase created")
}

func (rs *Resource) updatePhase(w http.ResponseWriter, r *http.Request) {
	if rs.PhaseService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("phase service not configured")))
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid id")))
		return
	}
	req := &PhaseRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	model, err := req.toModel(id)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// A stale client (pre-#1833) omits the concrete-class fields entirely.
	// Update replaces the whole row, so without this a partial update would
	// silently wipe the admin's class pick list / mandatory toggle. Re-hydrate
	// any omitted field from the stored phase before persisting.
	if req.AvailableSchoolClasses == nil || req.RequireSchoolClass == nil {
		var existing *enrollmentModels.Phase
		if fetchErr := rs.runInTenantTx(r, func(ctx context.Context) error {
			p, e := rs.PhaseService.GetByID(ctx, id)
			existing = p
			return e
		}); fetchErr != nil {
			if errors.Is(fetchErr, enrollmentService.ErrPhaseNotFound) {
				common.RenderError(w, r, common.ErrorNotFound(fetchErr))
				return
			}
			common.RenderError(w, r, common.ErrorInternalServer(fetchErr))
			return
		}
		if existing != nil {
			if req.AvailableSchoolClasses == nil {
				model.AvailableSchoolClasses = existing.AvailableSchoolClasses
			}
			if req.RequireSchoolClass == nil {
				model.RequireSchoolClass = existing.RequireSchoolClass
			}
		}
	}

	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		return rs.PhaseService.Update(ctx, model)
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Refetch so the response carries DB-managed timestamps + applied
	// defaults (e.g. care_overflow_mode normalisation in Validate).
	var refreshed *enrollmentModels.Phase
	if fetchErr := rs.runInTenantTx(r, func(ctx context.Context) error {
		p, e := rs.PhaseService.GetByID(ctx, id)
		refreshed = p
		return e
	}); fetchErr != nil {
		common.RenderError(w, r, common.ErrorInternalServer(fetchErr))
		return
	}
	common.Respond(w, r, http.StatusOK, toPhaseResponse(refreshed), "Phase updated")
}

func (rs *Resource) deletePhase(w http.ResponseWriter, r *http.Request) {
	if rs.PhaseService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("phase service not configured")))
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid id")))
		return
	}

	err = rs.runInTenantTx(r, func(ctx context.Context) error {
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
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid id")))
		return
	}

	var impact *enrollmentService.PhaseDeleteImpact
	err = rs.runInTenantTx(r, func(ctx context.Context) error {
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
