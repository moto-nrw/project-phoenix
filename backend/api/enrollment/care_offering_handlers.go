package enrollment

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// CareOfferingResponse is the wire shape for a single offering. IDs
// stringified so the frontend keeps its int64-as-string convention.
type CareOfferingResponse struct {
	ID                  string    `json:"id"`
	PhaseID             string    `json:"phase_id"`
	ActivityGroupID     *string   `json:"activity_group_id,omitempty"`
	Name                string    `json:"name"`
	Description         *string   `json:"description,omitempty"`
	DaysOfWeekMode      string    `json:"days_of_week_mode"`
	AvailableDays       []string  `json:"available_days"`
	IncludesHolidayCare bool      `json:"includes_holiday_care"`
	IncludesLunch       bool      `json:"includes_lunch"`
	Capacity            *int      `json:"capacity,omitempty"`
	PriceCents          *int      `json:"price_cents,omitempty"`
	IsActive            bool      `json:"is_active"`
	IsRequired          bool      `json:"is_required"`
	CountsAsCare        bool      `json:"counts_as_care"`
	AutoAddGradeLevels  []int     `json:"auto_add_grade_levels"`
	AutoAddTriggerIDs   []string  `json:"auto_add_trigger_offering_ids"`
	SortOrder           int       `json:"sort_order"`
	SelectionGroup      string    `json:"selection_group,omitempty"`
	SelectionRule       string    `json:"selection_rule"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func toCareOfferingResponse(o *enrollmentModels.CareOffering) CareOfferingResponse {
	resp := CareOfferingResponse{
		ID:                  strconv.FormatInt(o.ID, 10),
		PhaseID:             strconv.FormatInt(o.PhaseID, 10),
		Name:                o.Name,
		Description:         o.Description,
		DaysOfWeekMode:      o.DaysOfWeekMode,
		AvailableDays:       o.AvailableDays,
		IncludesHolidayCare: o.IncludesHolidayCare,
		IncludesLunch:       o.IncludesLunch,
		Capacity:            o.Capacity,
		PriceCents:          o.PriceCents,
		IsActive:            o.IsActive,
		IsRequired:          o.IsRequired,
		CountsAsCare:        o.CountsAsCare,
		AutoAddGradeLevels:  o.AutoAddGradeLevels,
		AutoAddTriggerIDs:   make([]string, 0, len(o.AutoAddTriggerOfferingIDs)),
		SortOrder:           o.SortOrder,
		SelectionGroup:      o.SelectionGroup,
		SelectionRule:       o.SelectionRule,
		CreatedAt:           o.CreatedAt,
		UpdatedAt:           o.UpdatedAt,
	}
	if o.ActivityGroupID != nil {
		s := strconv.FormatInt(*o.ActivityGroupID, 10)
		resp.ActivityGroupID = &s
	}
	for _, id := range o.AutoAddTriggerOfferingIDs {
		resp.AutoAddTriggerIDs = append(resp.AutoAddTriggerIDs, strconv.FormatInt(id, 10))
	}
	return resp
}

// CareOfferingRequest is the wire shape POST + PUT accept.
type CareOfferingRequest struct {
	PhaseID             int64    `json:"phase_id"`
	ActivityGroupID     *int64   `json:"activity_group_id,omitempty"`
	Name                string   `json:"name"`
	Description         *string  `json:"description,omitempty"`
	DaysOfWeekMode      string   `json:"days_of_week_mode"`
	AvailableDays       []string `json:"available_days"`
	IncludesHolidayCare bool     `json:"includes_holiday_care"`
	IncludesLunch       bool     `json:"includes_lunch"`
	Capacity            *int     `json:"capacity,omitempty"`
	PriceCents          *int     `json:"price_cents,omitempty"`
	IsActive            bool     `json:"is_active"`
	IsRequired          bool     `json:"is_required"`
	CountsAsCare        *bool    `json:"counts_as_care"`
	AutoAddGradeLevels  []int    `json:"auto_add_grade_levels"`
	AutoAddTriggerIDs   []string `json:"auto_add_trigger_offering_ids"`
	SortOrder           int      `json:"sort_order"`
	SelectionGroup      string   `json:"selection_group,omitempty"`
	SelectionRule       string   `json:"selection_rule,omitempty"`
}

// Bind satisfies render.Binder. Field-level validation runs in the
// model's Validate (called inside Repository.Create/Update).
func (req *CareOfferingRequest) Bind(_ *http.Request) error {
	if req.AvailableDays == nil {
		req.AvailableDays = []string{}
	}
	if req.AutoAddGradeLevels == nil {
		req.AutoAddGradeLevels = []int{}
	}
	if req.AutoAddTriggerIDs == nil {
		req.AutoAddTriggerIDs = []string{}
	}
	return nil
}

func (req *CareOfferingRequest) toModel(existingID int64) (*enrollmentModels.CareOffering, error) {
	countsAsCare := true
	if req.CountsAsCare != nil {
		countsAsCare = *req.CountsAsCare
	}
	triggerIDs, err := parseCareOfferingIDStrings(req.AutoAddTriggerIDs, "auto_add_trigger_offering_ids")
	if err != nil {
		return nil, err
	}
	o := &enrollmentModels.CareOffering{
		PhaseID:                   req.PhaseID,
		ActivityGroupID:           req.ActivityGroupID,
		Name:                      req.Name,
		Description:               req.Description,
		DaysOfWeekMode:            req.DaysOfWeekMode,
		AvailableDays:             req.AvailableDays,
		IncludesHolidayCare:       req.IncludesHolidayCare,
		IncludesLunch:             req.IncludesLunch,
		Capacity:                  req.Capacity,
		PriceCents:                req.PriceCents,
		IsActive:                  req.IsActive,
		IsRequired:                req.IsRequired,
		CountsAsCare:              countsAsCare,
		CountsAsCareSet:           true,
		AutoAddGradeLevels:        req.AutoAddGradeLevels,
		SortOrder:                 req.SortOrder,
		SelectionGroup:            req.SelectionGroup,
		SelectionRule:             req.SelectionRule,
		AutoAddTriggerOfferingIDs: triggerIDs,
	}
	o.ID = existingID
	return o, nil
}

func parseCareOfferingIDStrings(values []string, field string) ([]int64, error) {
	out := make([]int64, 0, len(values))
	for _, raw := range values {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("%s contains invalid id %q", field, raw)
		}
		out = append(out, id)
	}
	return out, nil
}

// CloneCareOfferingRequest is the body POST /{id}/clone accepts.
type CloneCareOfferingRequest struct {
	TargetPhaseID int64 `json:"target_phase_id"`
}

func (req *CloneCareOfferingRequest) Bind(_ *http.Request) error { return nil }

func (rs *Resource) listCareOfferings(w http.ResponseWriter, r *http.Request) {
	if rs.CareOfferingService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("care offering service not configured")))
		return
	}

	phaseFilter := r.URL.Query().Get("phase_id")
	var (
		offerings []*enrollmentModels.CareOffering
		err       error
	)
	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		var listErr error
		if phaseFilter != "" {
			id, parseErr := strconv.ParseInt(phaseFilter, 10, 64)
			if parseErr != nil || id <= 0 {
				return errors.New("invalid phase_id")
			}
			offerings, listErr = rs.CareOfferingService.ListByPhase(ctx, id)
			return listErr
		}
		offerings, listErr = rs.CareOfferingService.List(ctx)
		return listErr
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	out := make([]CareOfferingResponse, 0, len(offerings))
	for _, o := range offerings {
		out = append(out, toCareOfferingResponse(o))
	}
	common.Respond(w, r, http.StatusOK, out, "Care offerings retrieved")
}

func (rs *Resource) getCareOffering(w http.ResponseWriter, r *http.Request) {
	if rs.CareOfferingService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("care offering service not configured")))
		return
	}
	id, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid id")
	if !ok {
		return
	}
	var offering *enrollmentModels.CareOffering
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		o, e := rs.CareOfferingService.GetByID(ctx, id)
		offering = o
		return e
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(err))
		return
	}
	common.Respond(w, r, http.StatusOK, toCareOfferingResponse(offering), "Care offering retrieved")
}

func (rs *Resource) createCareOffering(w http.ResponseWriter, r *http.Request) {
	if rs.CareOfferingService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("care offering service not configured")))
		return
	}
	req := &CareOfferingRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	var offering *enrollmentModels.CareOffering
	model, err := req.toModel(0)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		o, e := rs.CareOfferingService.Create(ctx, model)
		offering = o
		return e
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	common.Respond(w, r, http.StatusCreated, toCareOfferingResponse(offering), "Care offering created")
}

func (rs *Resource) updateCareOffering(w http.ResponseWriter, r *http.Request) {
	updateWithRefetch(rs, w, r, rs.CareOfferingService == nil, "care offering service not configured",
		func(r *http.Request, id int64) (*enrollmentModels.CareOffering, error) {
			req := &CareOfferingRequest{}
			if err := render.Bind(r, req); err != nil {
				return nil, err
			}
			return req.toModel(id)
		},
		func(ctx context.Context, model *enrollmentModels.CareOffering) error {
			return rs.CareOfferingService.Update(ctx, model)
		},
		func(ctx context.Context, id int64) (*enrollmentModels.CareOffering, error) {
			return rs.CareOfferingService.GetByID(ctx, id)
		},
		func(o *enrollmentModels.CareOffering) any { return toCareOfferingResponse(o) },
		"Care offering updated")
}

func (rs *Resource) deleteCareOffering(w http.ResponseWriter, r *http.Request) {
	if rs.CareOfferingService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("care offering service not configured")))
		return
	}
	id, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid id")
	if !ok {
		return
	}
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		return rs.CareOfferingService.Delete(ctx, id)
	})
	if err != nil {
		// FK violation when request_child_offerings already references
		// this offering — admin should soft-delete (is_active=false).
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	common.RespondNoContent(w, r)
}

func (rs *Resource) cloneCareOffering(w http.ResponseWriter, r *http.Request) {
	if rs.CareOfferingService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("care offering service not configured")))
		return
	}
	sourceID, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid id")
	if !ok {
		return
	}
	req := &CloneCareOfferingRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	var clone *enrollmentModels.CareOffering
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		c, e := rs.CareOfferingService.Clone(ctx, sourceID, req.TargetPhaseID)
		clone = c
		return e
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	common.Respond(w, r, http.StatusCreated, toCareOfferingResponse(clone), "Care offering cloned")
}

// listPublicCareOfferings is the parent-facing endpoint. No JWT — the
// {tenantSlug} resolves to a tenant; {phaseId} narrows the offering set
// to one phase. The phase model owns the enrollment window, so the
// per-offering window check shipped in PR 6 is gone — the handler trusts
// the phase-level gate run by the caller (or by Submit on its own).
func (rs *Resource) listPublicCareOfferings(w http.ResponseWriter, r *http.Request) {
	if rs.CareOfferingService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("care offering service not configured")))
		return
	}
	if rs.SchoolService == nil || rs.RequestService == nil || rs.db == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("public endpoint not wired")))
		return
	}

	slug := chi.URLParam(r, "tenantSlug")
	if slug == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("tenant slug is required")))
		return
	}
	phaseID, ok := common.ParsePositiveInt64IDWithError(w, r, "phaseId", "phaseId is required")
	if !ok {
		return
	}

	var (
		offerings     []*enrollmentModels.CareOffering
		selectionMode = enrollmentModels.PhaseCareOfferingSelectionOptional
	)
	lateInviteToken := lateInviteTokenFromRequest(r)
	schoolID, err := rs.resolvePublicTenantID(r.Context(), slug)
	if err == nil {
		err = tenant.WithTenantTx(r.Context(), rs.db, schoolID, func(txCtx context.Context, _ bun.Tx) error {
			if rs.RequestService == nil {
				return errors.New("request service not configured")
			}
			// Shared public phase gate: enabled + active + open window, or
			// a valid late-invite exception for this exact phase.
			phase, phaseErr := rs.RequestService.LoadPublicPhaseWithLateInvite(txCtx, phaseID, time.Now(), lateInviteToken)
			if phaseErr != nil {
				return phaseErr
			}
			selectionMode = phase.CareOfferingSelectionMode
			list, listErr := rs.CareOfferingService.ListActiveByPhase(txCtx, phaseID)
			offerings = list
			return listErr
		})
	}
	if err != nil {
		renderPublicEnrollmentError(w, r, err)
		return
	}

	items := make([]CareOfferingResponse, 0, len(offerings))
	for _, o := range offerings {
		items = append(items, toCareOfferingResponse(o))
	}
	common.Respond(w, r, http.StatusOK, PublicCareOfferingsResponse{
		Offerings:                 items,
		CareOfferingSelectionMode: selectionMode,
		CareRequired:              selectionMode != enrollmentModels.PhaseCareOfferingSelectionOptional,
	}, "Public care offerings retrieved")
}

// ErrCodeEnrollmentDisabled is the stable code returned by every public
// enrollment-data endpoint (phases, schema, care offerings) when the
// tenant has toggled "Anmeldung aktiv" off. The frontend maps it to a
// friendly German notice and suppresses generic error banners. Keep in
// sync with the matching entry in
// frontend/src/lib/enrollment-submission-api.ts.
const ErrCodeEnrollmentDisabled = "enrollment.disabled"

// ErrCodeEnrollmentWindowClosed is returned by the public form-load
// endpoints when a direct/stale parent link points at a phase whose
// enrollment window is closed (or not yet open). Keep in sync with the
// matching entry in frontend/src/lib/enrollment-error-messages.ts.
const ErrCodeEnrollmentWindowClosed = "enrollment.window_closed"

// renderPublicEnrollmentError renders the error chain returned from a
// public enrollment endpoint. Disabled-tenant errors get a 404 with a
// stable code so the parent landing page can render the localized
// "Anmeldung aktuell deaktiviert" notice instead of the raw English
// service sentinel; closed-window phases get their own code so stale
// links explain the Anmeldefrist instead of "nicht gefunden". Anything
// else falls through to the generic 404 path so the existing "tenant
// not found" / "phase not found" messages still work.
func renderPublicEnrollmentError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, enrollmentService.ErrEnrollmentDisabled) {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, ErrCodeEnrollmentDisabled))
		return
	}
	if errors.Is(err, enrollmentService.ErrEnrollmentWindowClosed) {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, ErrCodeEnrollmentWindowClosed))
		return
	}
	if errors.Is(err, enrollmentService.ErrLateInviteInvalid) {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, ErrCodeEnrollmentLateInviteInvalid))
		return
	}
	common.RenderError(w, r, common.ErrorNotFound(err))
}

// PublicCareOfferingsResponse wraps the public care-offering catalog with
// the phase's selection mode so the parent form can render the hint and
// validate before submit. The mode is server-authoritative - the
// submission service re-checks it in defense-in-depth. CareRequired is
// kept as a legacy boolean for older frontend builds.
type PublicCareOfferingsResponse struct {
	Offerings                 []CareOfferingResponse `json:"offerings"`
	CareOfferingSelectionMode string                 `json:"care_offering_selection_mode"`
	CareRequired              bool                   `json:"care_required"`
}

type PublicEnrollmentFormBootstrapResponse struct {
	Phase                     PublicPhase                 `json:"phase"`
	Schema                    *PublicFormSchemaResponse   `json:"schema"`
	Offerings                 []CareOfferingResponse      `json:"offerings"`
	CareOfferingSelectionMode string                      `json:"care_offering_selection_mode"`
	CareRequired              bool                        `json:"care_required"`
	CaptchaConfig             PublicCaptchaConfigResponse `json:"captcha_config"`
	LegalTexts                PublicLegalTextsResponse    `json:"legal_texts"`
}

// We deliberately don't expose enrollmentService here — it is already
// referenced via *Resource.CareOfferingService.
var _ = enrollmentService.ErrCareOfferingNotFound

// PublicPhase is the parent-safe shape returned by the public phases
// endpoint. Intentionally slim — no created_by, no audit metadata.
type PublicPhase struct {
	ID                        string `json:"id"`
	Name                      string `json:"name"`
	Kind                      string `json:"kind"`
	ServiceStartDate          string `json:"service_start_date"`
	ServiceEndDate            string `json:"service_end_date"`
	EnrollmentOpenAt          string `json:"enrollment_open_at,omitempty"`
	EnrollmentCloseAt         string `json:"enrollment_close_at,omitempty"`
	ShowStatusReasonToParent  bool   `json:"show_status_reason_to_parent"`
	CareOfferingSelectionMode string `json:"care_offering_selection_mode"`
}

func toPublicPhase(p *enrollmentModels.Phase) PublicPhase {
	entry := PublicPhase{
		ID:                        strconv.FormatInt(p.ID, 10),
		Name:                      p.Name,
		Kind:                      p.Kind,
		ServiceStartDate:          p.ServiceStartDate.String(),
		ServiceEndDate:            p.ServiceEndDate.String(),
		ShowStatusReasonToParent:  p.ShowStatusReasonToParent,
		CareOfferingSelectionMode: p.CareOfferingSelectionMode,
	}
	if p.EnrollmentOpenAt != nil {
		entry.EnrollmentOpenAt = p.EnrollmentOpenAt.Format(time.RFC3339)
	}
	if p.EnrollmentCloseAt != nil {
		entry.EnrollmentCloseAt = p.EnrollmentCloseAt.Format(time.RFC3339)
	}
	return entry
}

func toPublicFormSchemaResponse(schema *enrollmentModels.FormSchema) *PublicFormSchemaResponse {
	if schema == nil {
		return nil
	}
	return &PublicFormSchemaResponse{
		ID:               strconv.FormatInt(schema.ID, 10),
		Version:          schema.Version,
		Fields:           schema.Fields,
		CoreRequirements: coreRequirementsValue(schema.CoreRequirements),
	}
}

func (rs *Resource) publicFormBootstrap(w http.ResponseWriter, r *http.Request) {
	if rs.SchoolService == nil || rs.CareOfferingService == nil ||
		rs.RequestService == nil || rs.CaptchaService == nil || rs.db == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("public enrollment bootstrap endpoint not wired")))
		return
	}

	slug := chi.URLParam(r, "tenantSlug")
	if slug == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("tenant slug is required")))
		return
	}
	phaseID, ok := common.ParsePositiveInt64IDWithError(w, r, "phaseId", "phaseId is required")
	if !ok {
		return
	}

	var (
		phase     *enrollmentModels.Phase
		schema    *enrollmentModels.FormSchema
		offerings []*enrollmentModels.CareOffering
		texts     enrollmentService.LegalTexts
		captcha   PublicCaptchaConfigResponse
		legalErr  error
	)
	lateInviteToken := lateInviteTokenFromRequest(r)
	schoolID, resolveErr := rs.resolvePublicTenantID(r.Context(), slug)
	if resolveErr == nil {
		resolveErr = tenant.WithTenantTx(r.Context(), rs.db, schoolID, func(txCtx context.Context, _ bun.Tx) error {
			// Shared public phase gate: enabled + active + open window, or
			// a valid late-invite exception for this exact phase.
			loadedPhase, phaseErr := rs.RequestService.LoadPublicPhaseWithLateInvite(txCtx, phaseID, time.Now(), lateInviteToken)
			if phaseErr != nil {
				return phaseErr
			}
			phase = loadedPhase
			if phase.FormSchemaID != nil {
				if rs.FormSchemaService == nil {
					return errors.New("form schema service not configured")
				}
				loadedSchema, schemaErr := rs.FormSchemaService.GetByID(txCtx, *phase.FormSchemaID)
				if schemaErr != nil {
					if !errors.Is(schemaErr, enrollmentService.ErrFormSchemaNotFound) {
						return schemaErr
					}
					// Stale pinned schema: degrade to Basis/core fields.
				} else {
					schema = loadedSchema
				}
			}
			list, listErr := rs.CareOfferingService.ListActiveByPhase(txCtx, phaseID)
			if listErr != nil {
				return listErr
			}
			offerings = list
			captcha.Enabled = rs.CaptchaService.IsEnabled(txCtx)
			captcha.SiteKey = rs.CaptchaService.SiteKey(txCtx)
			legalTexts, legalTextErr := rs.RequestService.LegalTextsForPhaseWithLateInvite(txCtx, phaseID, lateInviteToken)
			if legalTextErr != nil {
				if !errors.Is(legalTextErr, enrollmentService.ErrInvalidSubmission) &&
					!errors.Is(legalTextErr, enrollmentService.ErrEnrollmentDisabled) &&
					!errors.Is(legalTextErr, enrollmentService.ErrEnrollmentWindowClosed) &&
					!errors.Is(legalTextErr, enrollmentService.ErrLateInviteInvalid) {
					legalErr = legalTextErr
				}
				return legalTextErr
			}
			texts = legalTexts
			return nil
		})
	}
	if resolveErr != nil {
		if legalErr != nil {
			common.RenderError(w, r, common.ErrorInternalServer(fmt.Errorf("resolve legal texts: %w", legalErr)))
			return
		}
		renderPublicEnrollmentError(w, r, resolveErr)
		return
	}

	items := make([]CareOfferingResponse, 0, len(offerings))
	for _, o := range offerings {
		items = append(items, toCareOfferingResponse(o))
	}
	common.Respond(w, r, http.StatusOK, PublicEnrollmentFormBootstrapResponse{
		Phase:                     toPublicPhase(phase),
		Schema:                    toPublicFormSchemaResponse(schema),
		Offerings:                 items,
		CareOfferingSelectionMode: phase.CareOfferingSelectionMode,
		CareRequired:              phase.CareOfferingSelectionMode != enrollmentModels.PhaseCareOfferingSelectionOptional,
		CaptchaConfig:             captcha,
		LegalTexts: PublicLegalTextsResponse{
			AGB:                 texts.AGB,
			DSGVO:               texts.DSGVO,
			EmailContact:        texts.EmailContact,
			Photo:               texts.Photo,
			TermsEnabled:        texts.TermsEnabled,
			DSGVOEnabled:        texts.DSGVOEnabled,
			EmailContactEnabled: texts.EmailContactEnabled,
			PhotoEnabled:        texts.PhotoEnabled,
			Blocks:              texts.Blocks,
		},
	}, "Public enrollment form bootstrap retrieved")
}

// listPublicPhases returns the currently-open phases for the given
// tenant slug. No JWT — slug-gated. The parent landing page renders
// these as cards / pickers; clicking one routes the parent to the form.
func (rs *Resource) listPublicPhases(w http.ResponseWriter, r *http.Request) {
	if rs.SchoolService == nil || rs.PhaseService == nil || rs.db == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("public phases endpoint not wired")))
		return
	}

	slug := chi.URLParam(r, "tenantSlug")
	if slug == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("tenant slug is required")))
		return
	}

	var phases []*enrollmentModels.Phase
	schoolID, err := rs.resolvePublicTenantID(r.Context(), slug)
	if err == nil {
		err = tenant.WithTenantTx(r.Context(), rs.db, schoolID, func(txCtx context.Context, _ bun.Tx) error {
			if rs.RequestService != nil && !rs.RequestService.IsEnrollmentEnabled(txCtx) {
				return enrollmentService.ErrEnrollmentDisabled
			}
			list, listErr := rs.PhaseService.ListPublicOpen(txCtx, time.Now())
			phases = list
			return listErr
		})
	}
	if err != nil {
		renderPublicEnrollmentError(w, r, err)
		return
	}

	out := make([]PublicPhase, 0, len(phases))
	for _, p := range phases {
		out = append(out, toPublicPhase(p))
	}
	common.Respond(w, r, http.StatusOK, out, "Public phases retrieved")
}
