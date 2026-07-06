package enrollment

import (
	"context"
	"errors"
	"net/http"
	"strconv"
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
	CreatedAt             string  `json:"created_at"`
	UpdatedAt             string  `json:"updated_at"`
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
	if req.CalendarPeriodID != nil && *req.CalendarPeriodID != "" {
		id, parseErr := strconv.ParseInt(*req.CalendarPeriodID, 10, 64)
		if parseErr != nil || id <= 0 {
			return nil, errors.New("calendar_period_id must be a positive integer string")
		}
		p.CalendarPeriodID = &id
	}
	p.ID = existingID
	return p, nil
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
