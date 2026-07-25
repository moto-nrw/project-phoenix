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
	ID                  string                                         `json:"id"`
	PhaseID             string                                         `json:"phase_id"`
	ActivityGroupID     *string                                        `json:"activity_group_id,omitempty"`
	Name                string                                         `json:"name"`
	Description         *string                                        `json:"description,omitempty"`
	DaysOfWeekMode      string                                         `json:"days_of_week_mode"`
	AvailableDays       []string                                       `json:"available_days"`
	IncludesHolidayCare bool                                           `json:"includes_holiday_care"`
	IncludesLunch       bool                                           `json:"includes_lunch"`
	Capacity            *int                                           `json:"capacity,omitempty"`
	PriceCents          *int                                           `json:"price_cents,omitempty"`
	IsActive            bool                                           `json:"is_active"`
	IsRequired          bool                                           `json:"is_required"`
	CountsAsCare        bool                                           `json:"counts_as_care"`
	AutoAddGradeLevels  []int                                          `json:"auto_add_grade_levels"`
	AvailabilityRule    *enrollmentModels.CareOfferingAvailabilityRule `json:"availability_rule,omitempty"`
	AutoAddTriggerIDs   []string                                       `json:"auto_add_trigger_offering_ids"`
	SortOrder           int                                            `json:"sort_order"`
	SelectionGroup      string                                         `json:"selection_group,omitempty"`
	SelectionRule       string                                         `json:"selection_rule"`
	CreatedAt           time.Time                                      `json:"created_at"`
	UpdatedAt           time.Time                                      `json:"updated_at"`
}

// ErrCodeCareOfferingTemplatePeriodMismatch lets the admin frontend map the
// authoritative service validation to a localized explanation.
const ErrCodeCareOfferingTemplatePeriodMismatch = "enrollment.care_offering_template_period_mismatch"

// ErrCodeCareOfferingInUse identifies a delete blocked by existing enrollment
// selections without exposing PostgreSQL constraint names to the client.
const ErrCodeCareOfferingInUse = "enrollment.care_offering_in_use"

// ErrCodeCareOfferingDaysRequired identifies a save without any weekday so
// the admin editor can show a localized message (#1885).
const ErrCodeCareOfferingDaysRequired = "enrollment.care_offering_days_required"

func careOfferingWriteErrorRenderer(err error) render.Renderer {
	switch {
	case errors.Is(err, enrollmentService.ErrCareOfferingTemplatePeriodMismatch):
		return common.ErrorInvalidRequestWithCode(err, ErrCodeCareOfferingTemplatePeriodMismatch)
	case errors.Is(err, enrollmentModels.ErrCareOfferingDaysRequired):
		return common.ErrorInvalidRequestWithCode(err, ErrCodeCareOfferingDaysRequired)
	case errors.Is(err, enrollmentService.ErrCareOfferingInvalid),
		errors.Is(err, enrollmentService.ErrCareOfferingGroupRuleConflict):
		return common.ErrorInvalidRequest(err)
	default:
		return common.ErrorInternalServerWrap("care offering operation failed", err)
	}
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
		AvailabilityRule:    o.AvailabilityRule,
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
	PhaseID             int64                                          `json:"phase_id"`
	ActivityGroupID     *int64                                         `json:"activity_group_id,omitempty"`
	Name                string                                         `json:"name"`
	Description         *string                                        `json:"description,omitempty"`
	DaysOfWeekMode      string                                         `json:"days_of_week_mode"`
	AvailableDays       []string                                       `json:"available_days"`
	IncludesHolidayCare bool                                           `json:"includes_holiday_care"`
	IncludesLunch       bool                                           `json:"includes_lunch"`
	Capacity            *int                                           `json:"capacity,omitempty"`
	PriceCents          *int                                           `json:"price_cents,omitempty"`
	IsActive            bool                                           `json:"is_active"`
	IsRequired          bool                                           `json:"is_required"`
	CountsAsCare        *bool                                          `json:"counts_as_care"`
	AutoAddGradeLevels  []int                                          `json:"auto_add_grade_levels"`
	AvailabilityRule    *enrollmentModels.CareOfferingAvailabilityRule `json:"availability_rule,omitempty"`
	AutoAddTriggerIDs   []string                                       `json:"auto_add_trigger_offering_ids"`
	SortOrder           int                                            `json:"sort_order"`
	SelectionGroup      string                                         `json:"selection_group,omitempty"`
	SelectionRule       string                                         `json:"selection_rule,omitempty"`
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
		AvailabilityRule:          req.AvailabilityRule,
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
	var phaseID int64
	if phaseFilter != "" {
		var parseErr error
		phaseID, parseErr = strconv.ParseInt(phaseFilter, 10, 64)
		if parseErr != nil || phaseID <= 0 {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid phase_id")))
			return
		}
	}
	var (
		offerings []*enrollmentModels.CareOffering
		err       error
	)
	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		var listErr error
		if phaseFilter != "" {
			offerings, listErr = rs.CareOfferingService.ListByPhase(ctx, phaseID)
			return listErr
		}
		offerings, listErr = rs.CareOfferingService.List(ctx)
		return listErr
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("list care offerings failed", err))
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
		if errors.Is(err, enrollmentService.ErrCareOfferingNotFound) {
			common.RenderError(w, r, common.ErrorNotFound(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("load care offering failed", err))
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
		common.RenderError(w, r, careOfferingWriteErrorRenderer(err))
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
		"Care offering updated",
		careOfferingWriteErrorRenderer)
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
		if common.IsConstraintViolation(err) {
			common.RenderError(w, r, common.ErrorInvalidRequestWithCode(
				//nolint:staticcheck // ST1005: user-facing German message
				errors.New("Das Betreuungsangebot wird bereits verwendet und kann nicht gelöscht werden. Deaktivieren Sie es stattdessen."),
				ErrCodeCareOfferingInUse,
			))
			return
		}
		if errors.Is(err, enrollmentService.ErrCareOfferingInvalid) {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("delete care offering failed", err))
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
		common.RenderError(w, r, careOfferingWriteErrorRenderer(err))
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

	var data *enrollmentService.PublicFormBootstrapData
	lateInviteToken := lateInviteTokenFromRequest(r)
	schoolID, err := rs.resolvePublicTenantID(r.Context(), slug)
	if err == nil {
		err = tenant.WithTenantTx(r.Context(), rs.db, schoolID, func(txCtx context.Context, _ bun.Tx) error {
			loaded, loadErr := rs.RequestService.LoadPublicCareOfferings(txCtx, phaseID, time.Now(), lateInviteToken)
			data = loaded
			return loadErr
		})
	}
	if err != nil {
		renderPublicBootstrapError(w, r, err)
		return
	}

	selectionMode := data.Phase.CareOfferingSelectionMode
	schoolClassCfg := toPublicSchoolClassConfig(data.Phase, data.Capabilities.CollectSchoolClass)
	items := make([]CareOfferingResponse, 0, len(data.Offerings))
	for _, o := range data.Offerings {
		items = append(items, toCareOfferingResponse(o))
	}
	capabilities := enrollmentService.EffectiveFormCapabilities(data.Capabilities, data.Offerings)
	common.Respond(w, r, http.StatusOK, PublicCareOfferingsResponse{
		Offerings:                 items,
		CareOfferingSelectionMode: effectiveCareOfferingSelectionMode(selectionMode, capabilities.CareOfferingsEnabled),
		CareRequired:              capabilities.CareOfferingsEnabled && selectionMode != enrollmentModels.PhaseCareOfferingSelectionOptional,
		SchoolClass:               schoolClassCfg,
		CollectGradeLevel:         capabilities.CollectGradeLevel,
		CareOfferingsEnabled:      capabilities.CareOfferingsEnabled,
		EligibleGradeLevels:       publicEligibleGradeLevels(data.Phase),
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

// renderPublicBootstrapError maps a public-bootstrap error: a stage error
// (capability/legal resolution failure) is a server problem rendered as a
// 500 with the stage-specific wrap; everything else falls through to the
// public gate mapping (404 + stable codes).
func renderPublicBootstrapError(w http.ResponseWriter, r *http.Request, err error) {
	var stageErr *enrollmentService.BootstrapStageError
	if errors.As(err, &stageErr) {
		switch stageErr.Stage {
		case enrollmentService.BootstrapStageCapabilities:
			common.RenderError(w, r, common.ErrorInternalServer(fmt.Errorf("resolve collect_school_class: %w", stageErr.Err)))
			return
		case enrollmentService.BootstrapStageLegal:
			common.RenderError(w, r, common.ErrorInternalServer(fmt.Errorf("resolve legal texts: %w", stageErr.Err)))
			return
		}
	}
	renderPublicEnrollmentError(w, r, err)
}

// PublicCareOfferingsResponse wraps the public care-offering catalog with
// the phase's selection mode so the parent form can render the hint and
// validate before submit. The mode is server-authoritative - the
// submission service re-checks it in defense-in-depth. CareRequired is
// kept as a legacy boolean for older frontend builds.
type PublicCareOfferingsResponse struct {
	Offerings                 []CareOfferingResponse  `json:"offerings"`
	CareOfferingSelectionMode string                  `json:"care_offering_selection_mode"`
	CareRequired              bool                    `json:"care_required"`
	SchoolClass               PublicSchoolClassConfig `json:"school_class"`
	CollectGradeLevel         bool                    `json:"collect_grade_level"`
	CareOfferingsEnabled      bool                    `json:"care_offerings_enabled"`
	// EligibleGradeLevels mirrors PublicPhase.EligibleGradeLevels (#1663).
	// This response carries no phase object, and it is the form's fallback
	// load path when the page did not prefetch a bootstrap — so the grade
	// restriction has to ride along here too, next to the class config.
	EligibleGradeLevels []int `json:"eligible_grade_levels"`
}

type PublicEnrollmentFormBootstrapResponse struct {
	Phase                     PublicPhase                 `json:"phase"`
	Schema                    *PublicFormSchemaResponse   `json:"schema"`
	Offerings                 []CareOfferingResponse      `json:"offerings"`
	CareOfferingSelectionMode string                      `json:"care_offering_selection_mode"`
	CareRequired              bool                        `json:"care_required"`
	SchoolClass               PublicSchoolClassConfig     `json:"school_class"`
	CollectGradeLevel         bool                        `json:"collect_grade_level"`
	CareOfferingsEnabled      bool                        `json:"care_offerings_enabled"`
	CaptchaConfig             PublicCaptchaConfigResponse `json:"captcha_config"`
	LegalTexts                PublicLegalTextsResponse    `json:"legal_texts"`
}

// PublicSchoolClassConfig is the parent-facing concrete-class config for
// a phase (issue #1833): whether the tenant collects a concrete class at
// all, the phase's pick list, and whether it is mandatory from grade 2.
// Emitted on both the bootstrap and the care-offerings public responses
// so both form-load paths (prefetched public page + parent-portal
// internal load) see the same contract.
type PublicSchoolClassConfig struct {
	Collect          bool     `json:"collect"`
	AvailableClasses []string `json:"available_classes"`
	Require          bool     `json:"require"`
}

func toPublicSchoolClassConfig(phase *enrollmentModels.Phase, collect bool) PublicSchoolClassConfig {
	classes := phase.AvailableSchoolClasses
	if classes == nil {
		classes = []string{}
	}
	return PublicSchoolClassConfig{
		Collect:          collect,
		AvailableClasses: classes,
		Require:          phase.RequireSchoolClass,
	}
}

// publicEligibleGradeLevels returns the phase's grade restriction as a
// non-nil slice so the JSON is `[]` rather than `null` (#1663).
func publicEligibleGradeLevels(phase *enrollmentModels.Phase) []int {
	if phase == nil || phase.EligibleGradeLevels == nil {
		return []int{}
	}
	return phase.EligibleGradeLevels
}

func effectiveCareOfferingSelectionMode(mode string, enabled bool) string {
	if !enabled {
		return enrollmentModels.PhaseCareOfferingSelectionOptional
	}
	return mode
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
	// Audience (#1663): linked_parents phases never reach this listing
	// (filtered in ListPublicOpen); "new_students" lets the public
	// picker label the restriction.
	Audience string `json:"audience"`
	// EligibleGradeLevels (#1663) is the phase's grade restriction, empty
	// when unrestricted. The form narrows its grade select to these values
	// so a parent cannot fill in the whole form only to be rejected with
	// grade_not_eligible — the same reason the offered class list is
	// narrowed server-side.
	EligibleGradeLevels []int `json:"eligible_grade_levels"`
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
		Audience:                  p.Audience,
		EligibleGradeLevels:       p.EligibleGradeLevels,
	}
	if entry.EligibleGradeLevels == nil {
		// Emit [] rather than null so the frontend list binding is stable.
		entry.EligibleGradeLevels = []int{}
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
		data    *enrollmentService.PublicFormBootstrapData
		captcha PublicCaptchaConfigResponse
	)
	lateInviteToken := lateInviteTokenFromRequest(r)
	schoolID, resolveErr := rs.resolvePublicTenantID(r.Context(), slug)
	if resolveErr == nil {
		resolveErr = tenant.WithTenantTx(r.Context(), rs.db, schoolID, func(txCtx context.Context, _ bun.Tx) error {
			loaded, loadErr := rs.RequestService.LoadPublicFormBootstrap(txCtx, phaseID, time.Now(), lateInviteToken)
			if loadErr != nil {
				return loadErr
			}
			data = loaded
			captcha.Enabled = rs.CaptchaService.IsEnabled(txCtx)
			captcha.SiteKey = rs.CaptchaService.SiteKey(txCtx)
			return nil
		})
	}
	if resolveErr != nil {
		renderPublicBootstrapError(w, r, resolveErr)
		return
	}

	common.Respond(w, r, http.StatusOK, BuildPublicEnrollmentFormBootstrapResponse(data, captcha),
		"Public enrollment form bootstrap retrieved")
}

// BuildPublicEnrollmentFormBootstrapResponse assembles the parent-facing
// bootstrap wire response from resolved bootstrap data. Shared by the
// anonymous public form-bootstrap handler and the authenticated
// parents-portal bootstrap handler so both form-load paths emit an
// identical contract. captcha is empty for the parent path (the parent JWT
// is the anti-bot signal, so captcha is skipped there).
func BuildPublicEnrollmentFormBootstrapResponse(data *enrollmentService.PublicFormBootstrapData, captcha PublicCaptchaConfigResponse) PublicEnrollmentFormBootstrapResponse {
	items := make([]CareOfferingResponse, 0, len(data.Offerings))
	for _, o := range data.Offerings {
		items = append(items, toCareOfferingResponse(o))
	}
	phase := data.Phase
	texts := data.LegalTexts
	capabilities := enrollmentService.EffectiveFormCapabilities(data.Capabilities, data.Offerings)
	return PublicEnrollmentFormBootstrapResponse{
		Phase:                     toPublicPhase(phase),
		Schema:                    toPublicFormSchemaResponse(data.Schema),
		Offerings:                 items,
		CareOfferingSelectionMode: effectiveCareOfferingSelectionMode(phase.CareOfferingSelectionMode, capabilities.CareOfferingsEnabled),
		CareRequired:              capabilities.CareOfferingsEnabled && phase.CareOfferingSelectionMode != enrollmentModels.PhaseCareOfferingSelectionOptional,
		SchoolClass:               toPublicSchoolClassConfig(phase, capabilities.CollectSchoolClass),
		CollectGradeLevel:         capabilities.CollectGradeLevel,
		CareOfferingsEnabled:      capabilities.CareOfferingsEnabled,
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
	}
}

// RenderPublicEnrollmentBootstrapError maps a public/enrollee bootstrap
// error to HTTP. Exported so the parents-portal bootstrap handler emits the
// same error codes (disabled / window-closed / late-invite → coded 404s,
// stage failures → 500) as the anonymous public path.
func RenderPublicEnrollmentBootstrapError(w http.ResponseWriter, r *http.Request, err error) {
	renderPublicBootstrapError(w, r, err)
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
