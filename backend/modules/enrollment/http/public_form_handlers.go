package enrollmenthttp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
)

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
	if rs.SchoolService == nil || rs.RequestService == nil || !rs.transactions {
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

	var data *PublicFormBootstrapData
	lateInviteToken := lateInviteTokenFromRequest(r)
	schoolID, err := rs.resolvePublicTenantID(r.Context(), slug)
	if err == nil {
		err = withinTenant(r.Context(), schoolID, func(txCtx context.Context) error {
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
		items = append(items, toPublicCareOfferingResponse(o))
	}
	capabilities := data.EffectiveCapabilities
	common.Respond(w, r, http.StatusOK, PublicCareOfferingsResponse{
		Offerings:                 items,
		CareOfferingSelectionMode: effectiveCareOfferingSelectionMode(selectionMode, capabilities.CareOfferingsEnabled),
		CareRequired:              capabilities.CareOfferingsEnabled && selectionMode != capability.PhaseCareOfferingSelectionOptional,
		SchoolClass:               schoolClassCfg,
		CollectGradeLevel:         capabilities.CollectGradeLevel,
		CareOfferingsEnabled:      capabilities.CareOfferingsEnabled,
		EligibleGradeLevels:       publicEligibleGradeLevels(data.Phase),
	}, "Public care offerings retrieved")
}

// renderPublicEnrollmentError renders the error chain returned from a
// public enrollment endpoint. Disabled-tenant errors get a 404 with a
// stable code so the parent landing page can render the localized
// "Anmeldung aktuell deaktiviert" notice instead of the raw English
// service sentinel; closed-window phases get their own code so stale
// links explain the Anmeldefrist instead of "nicht gefunden". Anything
// else falls through to the generic 404 path so the existing "tenant
// not found" / "phase not found" messages still work.
func renderPublicEnrollmentError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, capability.ErrEnrollmentDisabled) {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentDisabled))
		return
	}
	if errors.Is(err, capability.ErrEnrollmentWindowClosed) {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentWindowClosed))
		return
	}
	if errors.Is(err, capability.ErrLateInviteInvalid) {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentLateInviteInvalid))
		return
	}
	common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentFormNotFound))
}

// renderPublicBootstrapError maps a public-bootstrap error: a stage error
// (capability/legal resolution failure) is a server problem rendered as a
// 500 with the stage-specific wrap; everything else falls through to the
// public gate mapping (404 + stable codes).
func renderPublicBootstrapError(w http.ResponseWriter, r *http.Request, err error) {
	var stageErr *BootstrapStageError
	if errors.As(err, &stageErr) {
		switch stageErr.Stage {
		case BootstrapStageCapabilities:
			common.RenderError(w, r, common.ErrorInternalServer(fmt.Errorf("resolve collect_school_class: %w", stageErr.Err)))
			return
		case BootstrapStageLegal:
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
	LateInvite                *PublicLateInvitePrefill    `json:"late_invite,omitempty"`
}

// PublicLateInvitePrefill exposes only the recipient fields the parent form
// may prefill. The token remains the authorization boundary and is never
// returned in the response.
type PublicLateInvitePrefill struct {
	GuardianEmail     string  `json:"guardian_email"`
	GuardianFirstName *string `json:"guardian_first_name,omitempty"`
	GuardianLastName  *string `json:"guardian_last_name,omitempty"`
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
	// CollectGrade1 is the server-authoritative answer to "does grade 1
	// declare a concrete class in this phase?" (#1663). Grade 1 is opt-in
	// (#1833) and the form cannot derive the answer itself: a phase
	// restricted to a prefixless class ("Bienen") collects it for grade 1
	// too, yet its narrowed pick list looks exactly like an unrestricted
	// phase that merely offers a named class. Hiding the field where the
	// submit path demands it — or showing it where the submit path clears
	// it — is a dead end either way, so the decision is made once, in
	// enrollment.CollectsGrade1Class, and shipped to the form.
	CollectGrade1 bool `json:"collect_grade_1"`
}

func toPublicSchoolClassConfig(phase *capability.Phase, collect bool) PublicSchoolClassConfig {
	classes := phase.AvailableSchoolClasses
	if classes == nil {
		classes = []string{}
	}
	return PublicSchoolClassConfig{
		Collect:          collect,
		AvailableClasses: classes,
		Require:          phase.RequireSchoolClass,
		CollectGrade1:    capability.CollectsGrade1Class(phase),
	}
}

// publicEligibleGradeLevels returns the phase's grade restriction as a
// non-nil slice so the JSON is `[]` rather than `null` (#1663).
func publicEligibleGradeLevels(phase *capability.Phase) []int {
	if phase == nil || phase.EligibleGradeLevels == nil {
		return []int{}
	}
	return phase.EligibleGradeLevels
}

func effectiveCareOfferingSelectionMode(mode string, enabled bool) string {
	if !enabled {
		return capability.PhaseCareOfferingSelectionOptional
	}
	return mode
}

// We deliberately don't expose enrollmentService here — it is already
// referenced via *Resource.CareOfferingService.
var _ = capability.ErrCareOfferingNotFound

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
	// Audience (#1663): audience-restricted phases (linked_parents and
	// existing_students) never reach this listing — ListPublicOpen filters
	// exactly what the anonymous form gate refuses; "new_students" lets the
	// public picker label the remaining restriction.
	Audience string `json:"audience"`
	// EligibleGradeLevels (#1663) is the phase's grade restriction, empty
	// when unrestricted. The form narrows its grade select to these values
	// so a parent cannot fill in the whole form only to be rejected with
	// grade_not_eligible — the same reason the offered class list is
	// narrowed server-side.
	EligibleGradeLevels []int `json:"eligible_grade_levels"`
	// Translations holds only name translations that still match the
	// German name, without their source (#3377).
	Translations capability.Translations `json:"translations,omitempty"`
}

func toPublicPhase(p *capability.Phase) PublicPhase {
	entry := PublicPhase{
		ID:                        strconv.FormatInt(p.ID, 10),
		Name:                      p.Name,
		Kind:                      p.Kind,
		ServiceStartDate:          string(p.ServiceStartDate),
		ServiceEndDate:            string(p.ServiceEndDate),
		ShowStatusReasonToParent:  p.ShowStatusReasonToParent,
		CareOfferingSelectionMode: p.CareOfferingSelectionMode,
		Audience:                  p.Audience,
		EligibleGradeLevels:       p.EligibleGradeLevels,
		Translations: p.Translations.Fresh(map[string]string{
			capability.TranslationAttrName: p.Name,
		}),
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

func toPublicFormSchemaResponse(schema *capability.FormSchema) *PublicFormSchemaResponse {
	if schema == nil {
		return nil
	}
	return &PublicFormSchemaResponse{
		ID:               strconv.FormatInt(schema.ID, 10),
		Version:          schema.Version,
		Fields:           capability.PublicFormFields(schema.Fields),
		CoreRequirements: coreRequirementsValue(schema.CoreRequirements),
	}
}

func (rs *Resource) publicFormBootstrap(w http.ResponseWriter, r *http.Request) {
	if rs.SchoolService == nil || rs.CareOfferingService == nil ||
		rs.RequestService == nil || rs.CaptchaService == nil || !rs.transactions {
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
		data    *PublicFormBootstrapData
		captcha PublicCaptchaConfigResponse
	)
	lateInviteToken := lateInviteTokenFromRequest(r)
	schoolID, resolveErr := rs.resolvePublicTenantID(r.Context(), slug)
	if resolveErr == nil {
		resolveErr = withinTenant(r.Context(), schoolID, func(txCtx context.Context) error {
			loaded, loadErr := rs.RequestService.LoadPublicFormBootstrap(txCtx, phaseID, time.Now(), lateInviteToken)
			if loadErr != nil {
				return loadErr
			}
			data = loaded
			captcha.Enabled, loadErr = rs.CaptchaService.IsEnabled(txCtx)
			if loadErr != nil {
				return loadErr
			}
			captcha.SiteKey, loadErr = rs.CaptchaService.SiteKey(txCtx)
			if loadErr != nil {
				return loadErr
			}
			return nil
		})
	}
	if resolveErr != nil {
		renderPublicBootstrapError(w, r, resolveErr)
		return
	}

	common.Respond(w, r, http.StatusOK, buildPublicEnrollmentFormBootstrapResponse(data, captcha),
		"Public enrollment form bootstrap retrieved")
}

// buildPublicEnrollmentFormBootstrapResponse assembles the parent-facing
// bootstrap wire response from resolved bootstrap data. Shared by the
// anonymous public form-bootstrap handler and the parents portal's form
// flow (ParentForms) so both form-load paths emit an identical contract.
// captcha is empty for the parent path (the parent JWT is the anti-bot
// signal, so captcha is skipped there).
func buildPublicEnrollmentFormBootstrapResponse(data *PublicFormBootstrapData, captcha PublicCaptchaConfigResponse) PublicEnrollmentFormBootstrapResponse {
	items := make([]CareOfferingResponse, 0, len(data.Offerings))
	for _, o := range data.Offerings {
		items = append(items, toPublicCareOfferingResponse(o))
	}
	phase := data.Phase
	texts := data.LegalTexts
	capabilities := data.EffectiveCapabilities
	var lateInvite *PublicLateInvitePrefill
	if data.LateInvite != nil {
		lateInvite = &PublicLateInvitePrefill{
			GuardianEmail:     data.LateInvite.GuardianEmail,
			GuardianFirstName: data.LateInvite.GuardianFirstName,
			GuardianLastName:  data.LateInvite.GuardianLastName,
		}
	}
	return PublicEnrollmentFormBootstrapResponse{
		Phase:                     toPublicPhase(phase),
		Schema:                    toPublicFormSchemaResponse(data.Schema),
		Offerings:                 items,
		CareOfferingSelectionMode: effectiveCareOfferingSelectionMode(phase.CareOfferingSelectionMode, capabilities.CareOfferingsEnabled),
		CareRequired:              capabilities.CareOfferingsEnabled && phase.CareOfferingSelectionMode != capability.PhaseCareOfferingSelectionOptional,
		SchoolClass:               toPublicSchoolClassConfig(phase, capabilities.CollectSchoolClass),
		CollectGradeLevel:         capabilities.CollectGradeLevel,
		CareOfferingsEnabled:      capabilities.CareOfferingsEnabled,
		CaptchaConfig:             captcha,
		LateInvite:                lateInvite,
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

// listPublicPhases returns the currently-open phases for the given
// tenant slug. No JWT — slug-gated. The parent landing page renders
// these as cards / pickers; clicking one routes the parent to the form.
func (rs *Resource) listPublicPhases(w http.ResponseWriter, r *http.Request) {
	if rs.SchoolService == nil || rs.PhaseService == nil || !rs.transactions {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("public phases endpoint not wired")))
		return
	}

	slug := chi.URLParam(r, "tenantSlug")
	if slug == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("tenant slug is required")))
		return
	}

	var phases []*capability.Phase
	schoolID, err := rs.resolvePublicTenantID(r.Context(), slug)
	if err == nil {
		err = withinTenant(r.Context(), schoolID, func(txCtx context.Context) error {
			if rs.RequestService != nil && !rs.RequestService.IsEnrollmentEnabled(txCtx) {
				return capability.ErrEnrollmentDisabled
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
