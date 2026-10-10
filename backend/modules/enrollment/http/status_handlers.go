package enrollmenthttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// --- status / edit / withdraw handlers (token-gated, public) ---

// StatusResponse is the public status-page payload. ChildID is
// stringified so the frontend keeps its int64-as-string convention.
type StatusResponse struct {
	RequestID         string                `json:"request_id"`
	GuardianFirstName string                `json:"guardian_first_name"`
	GuardianLastName  string                `json:"guardian_last_name"`
	GuardianEmail     string                `json:"guardian_email"`
	GuardianPhone     *string               `json:"guardian_phone,omitempty"`
	SubmittedAt       time.Time             `json:"submitted_at"`
	WithdrawnAt       *time.Time            `json:"withdrawn_at,omitempty"`
	EditMode          string                `json:"edit_mode"`
	Children          []StatusChildResponse `json:"children"`
	// AdditionalGuardians are the co-guardians the parent added beyond the
	// primary guardian above. Empty when none were added.
	AdditionalGuardians []StatusGuardianResponse `json:"additional_guardians,omitempty"`
	// HasParentAccount reports whether the primary guardian can log in to
	// the parent app. It is set only while a child is taken over into care
	// and left out when the lookup failed, so the page never claims a
	// missing account it could not check (#3742).
	HasParentAccount *bool `json:"has_parent_account,omitempty"`
	// ParentPortalAccess is the next step for the primary guardian: account,
	// invitation, or contact_ogs. It is sent with HasParentAccount only while
	// a child is taken over into care.
	ParentPortalAccess *string `json:"parent_portal_access,omitempty"`
}

// StatusGuardianResponse is one additional guardian on the public status
// page. Email/phone are optional.
type StatusGuardianResponse struct {
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Email     *string `json:"email,omitempty"`
	Phone     *string `json:"phone,omitempty"`
}

// StatusChildResponse is one row in StatusResponse.Children.
type StatusChildResponse struct {
	ID           string  `json:"id"`
	FirstName    string  `json:"first_name"`
	LastName     string  `json:"last_name"`
	Status       string  `json:"status"`
	StatusReason *string `json:"status_reason,omitempty"`
	// Locked marks a child that is already taken over into care. Its data
	// stays readable through the status link, but changes to it run through
	// the parent app only (ADR 0003).
	Locked bool `json:"locked"`
}

type EditBootstrapResponse struct {
	Phase                     PublicPhase               `json:"phase"`
	Schema                    *PublicFormSchemaResponse `json:"schema"`
	Offerings                 []CareOfferingResponse    `json:"offerings"`
	CareOfferingSelectionMode string                    `json:"care_offering_selection_mode"`
	CareRequired              bool                      `json:"care_required"`
	SchoolClass               PublicSchoolClassConfig   `json:"school_class"`
	CollectGradeLevel         bool                      `json:"collect_grade_level"`
	CareOfferingsEnabled      bool                      `json:"care_offerings_enabled"`
	GradeLevelMax             int                       `json:"grade_level_max"`
	LegalTexts                PublicLegalTextsResponse  `json:"legal_texts"`
	Draft                     EditDraftResponse         `json:"draft"`
	EditMode                  string                    `json:"edit_mode"`
}

type EditDraftResponse struct {
	RequestID           string                      `json:"request_id"`
	StatusToken         string                      `json:"status_token"`
	TenantID            string                      `json:"tenant_id"`
	TenantSubdomain     string                      `json:"tenant_subdomain"`
	PhaseID             string                      `json:"phase_id"`
	GuardianFirstName   string                      `json:"guardian_first_name"`
	GuardianLastName    string                      `json:"guardian_last_name"`
	GuardianEmail       string                      `json:"guardian_email"`
	GuardianPhone       *string                     `json:"guardian_phone,omitempty"`
	ConsentFlags        map[string]any              `json:"consent_flags"`
	CustomData          map[string]any              `json:"custom_data"`
	AdditionalGuardians []EditDraftGuardianResponse `json:"additional_guardians,omitempty"`
	Children            []EditDraftChildResponse    `json:"children"`
}

type EditDraftGuardianResponse struct {
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Email     *string `json:"email,omitempty"`
	Phone     *string `json:"phone,omitempty"`
}

type EditDraftChildResponse struct {
	ID                string                         `json:"id"`
	FirstName         string                         `json:"first_name"`
	LastName          string                         `json:"last_name"`
	DateOfBirth       string                         `json:"date_of_birth"`
	TargetGradeLevel  *int16                         `json:"target_grade_level,omitempty"`
	TargetSchoolClass *string                        `json:"target_school_class,omitempty"`
	CustomData        map[string]any                 `json:"custom_data"`
	OfferingIDs       []string                       `json:"offering_ids"`
	OfferingDays      []EditDraftOfferingDayResponse `json:"offering_days,omitempty"`
	// Locked marks a child that is already taken over into care: the change
	// form shows it read-only and points to the parent app (ADR 0003).
	Locked bool `json:"locked"`
}

type EditDraftOfferingDayResponse struct {
	OfferingID            string   `json:"offering_id"`
	SelectedDays          []string `json:"selected_days"`
	ManualSelectedDays    []string `json:"manual_selected_days,omitempty"`
	AutomaticSelectedDays []string `json:"automatic_selected_days,omitempty"`
}

// getStatus returns the per-child status for a token-bearing parent.
// Public route — caller must wrap in admin-tx because there's no JWT.
func (rs *Resource) getStatus(w http.ResponseWriter, r *http.Request) {
	if rs.RequestService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("enrollment request service not configured")))
		return
	}
	token := strings.TrimSpace(chi.URLParam(r, "statusToken"))
	if token == "" {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(errors.New("status token is required"), common.CodeEnrollmentStatusLinkInvalid))
		return
	}

	var (
		req       *Request
		children  []*RequestChild
		guardians []*capability.RequestGuardian
		editMode  string
		statusErr error
	)
	err := withinAdmin(r.Context(), func(adminCtx context.Context) error {
		serviceReq, serviceChildren, err := rs.RequestService.GetByStatusToken(adminCtx, token)
		if err != nil {
			return err
		}
		req = serviceReq
		children = serviceChildren
		mode, modeErr := rs.RequestService.EditModeForStatus(adminCtx, req, children)
		if modeErr != nil {
			statusErr = fmt.Errorf("status: compute edit mode: %w", modeErr)
			return statusErr
		}
		editMode = mode
		// Best-effort: a failure here must not hide the request status, but a
		// silent drop would mask a permission/RLS/DB problem behind a 200 with
		// missing co-guardians. Log it so the failure is visible.
		if g, gerr := rs.RequestService.GuardiansByStatusToken(adminCtx, token); gerr == nil {
			guardians = g
		} else {
			rs.logger().Warn("enrollment status: load co-guardians failed",
				slog.String("error", gerr.Error()))
		}
		return nil
	})
	if err != nil {
		if statusErr != nil {
			common.RenderError(w, r, common.ErrorInternalServer(statusErr))
			return
		}
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentStatusLinkInvalid))
		return
	}

	resp, anyLocked := newStatusResponse(req, editMode, children, guardians)
	if anyLocked {
		resp.HasParentAccount, resp.ParentPortalAccess = rs.statusParentAccount(r.Context(), token, req.ID)
	}
	common.Respond(w, r, http.StatusOK, resp, "Status retrieved")
}

// newStatusResponse maps a request to the public status payload and reports
// whether any child is taken over into care.
func newStatusResponse(req *Request, editMode string, children []*RequestChild, guardians []*capability.RequestGuardian) (StatusResponse, bool) {
	resp := StatusResponse{
		RequestID:         strconv.FormatInt(req.ID, 10),
		GuardianFirstName: req.GuardianFirstName,
		GuardianLastName:  req.GuardianLastName,
		GuardianEmail:     req.GuardianEmail,
		GuardianPhone:     req.GuardianPhone,
		SubmittedAt:       req.SubmittedAt,
		WithdrawnAt:       req.WithdrawnAt,
		EditMode:          editMode,
	}
	anyLocked := false
	for _, c := range children {
		locked := ChildTakenOver(c)
		anyLocked = anyLocked || locked
		resp.Children = append(resp.Children, StatusChildResponse{
			ID:           strconv.FormatInt(c.ID, 10),
			FirstName:    c.FirstName,
			LastName:     c.LastName,
			Status:       c.Status,
			StatusReason: c.StatusReason,
			Locked:       locked,
		})
	}
	for _, g := range guardians {
		resp.AdditionalGuardians = append(resp.AdditionalGuardians, StatusGuardianResponse{
			FirstName: g.FirstName,
			LastName:  g.LastName,
			Email:     g.Email,
			Phone:     g.Phone,
		})
	}
	return resp, anyLocked
}

// statusParentAccount answers the useful route into the parent app for the
// family behind a status token. Best-effort: a failure must not hide the
// status, so it answers nil values and the page keeps its existing login link.
func (rs *Resource) statusParentAccount(ctx context.Context, token string, requestID int64) (*bool, *string) {
	if rs.ChangeRequestService == nil {
		return nil, nil
	}
	access, err := rs.ChangeRequestService.PrimaryGuardianPortalAccess(ctx, token)
	if err != nil {
		rs.logger().Warn("enrollment status: parent account lookup failed",
			slog.Int64("request_id", requestID),
			slog.String("error", err.Error()))
		return nil, nil
	}
	switch access {
	case "account":
		hasAccount := true
		return &hasAccount, &access
	case "invitation", "contact_ogs":
		hasAccount := false
		return &hasAccount, &access
	default:
		rs.logger().Warn("enrollment status: unknown parent portal access",
			slog.Int64("request_id", requestID),
			slog.String("parent_portal_access", access))
		return nil, nil
	}
}

func (rs *Resource) getEditBootstrap(w http.ResponseWriter, r *http.Request) {
	if rs.RequestService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("enrollment request service not configured")))
		return
	}
	token := strings.TrimSpace(chi.URLParam(r, "statusToken"))
	if token == "" {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(errors.New("status token is required"), common.CodeEnrollmentStatusLinkInvalid))
		return
	}

	draft, err := rs.RequestService.GetEditDraft(r.Context(), token)
	if err != nil {
		mapEditError(w, r, err)
		return
	}

	offerings := make([]CareOfferingResponse, 0, len(draft.OpenOfferings))
	for _, o := range draft.OpenOfferings {
		offerings = append(offerings, toPublicCareOfferingResponse(o))
	}
	common.Respond(w, r, http.StatusOK, EditBootstrapResponse{
		Phase:                     toPublicPhase(draft.Phase),
		Schema:                    toPublicFormSchemaResponse(draft.Schema),
		Offerings:                 offerings,
		CareOfferingSelectionMode: effectiveCareOfferingSelectionMode(draft.Phase.CareOfferingSelectionMode, draft.CareOfferingsEnabled),
		CareRequired:              draft.CareOfferingsEnabled && draft.Phase.CareOfferingSelectionMode != capability.PhaseCareOfferingSelectionOptional,
		SchoolClass:               toPublicSchoolClassConfig(draft.Phase, draft.CollectSchoolClass),
		CollectGradeLevel:         draft.CollectGradeLevel,
		CareOfferingsEnabled:      draft.CareOfferingsEnabled,
		GradeLevelMax:             draft.GradeLevelMax,
		LegalTexts: PublicLegalTextsResponse{
			AGB:                 draft.LegalTexts.AGB,
			DSGVO:               draft.LegalTexts.DSGVO,
			EmailContact:        draft.LegalTexts.EmailContact,
			Photo:               draft.LegalTexts.Photo,
			TermsEnabled:        draft.LegalTexts.TermsEnabled,
			DSGVOEnabled:        draft.LegalTexts.DSGVOEnabled,
			EmailContactEnabled: draft.LegalTexts.EmailContactEnabled,
			PhotoEnabled:        draft.LegalTexts.PhotoEnabled,
			Blocks:              draft.LegalTexts.Blocks,
		},
		Draft:    toEditDraftResponse(draft),
		EditMode: draft.EditMode,
	}, "Enrollment edit bootstrap retrieved")
}

func toEditDraftResponse(draft *EditDraft) EditDraftResponse {
	resp := EditDraftResponse{
		RequestID:           strconv.FormatInt(draft.Request.ID, 10),
		StatusToken:         draft.Request.StatusToken,
		TenantID:            strconv.FormatInt(draft.Request.TenantID, 10),
		PhaseID:             strconv.FormatInt(draft.Request.PhaseID, 10),
		GuardianFirstName:   draft.Request.GuardianFirstName,
		GuardianLastName:    draft.Request.GuardianLastName,
		GuardianEmail:       draft.Request.GuardianEmail,
		GuardianPhone:       draft.Request.GuardianPhone,
		ConsentFlags:        draft.Request.ConsentFlags,
		CustomData:          draft.Request.CustomData,
		AdditionalGuardians: toEditDraftGuardianResponses(draft.Guardians),
		Children:            toEditDraftChildResponses(draft),
	}
	if draft.School != nil {
		// Tenant resolution (TenantProvider → /auth/tenant/resolve) works by
		// subdomain, not slug (#1977).
		resp.TenantSubdomain = draft.School.Subdomain
	}
	return resp
}

func toEditDraftGuardianResponses(guardians []*capability.RequestGuardian) []EditDraftGuardianResponse {
	var responses []EditDraftGuardianResponse
	for _, guardian := range guardians {
		responses = append(responses, EditDraftGuardianResponse{
			FirstName: guardian.FirstName,
			LastName:  guardian.LastName,
			Email:     guardian.Email,
			Phone:     guardian.Phone,
		})
	}
	return responses
}

func toEditDraftChildResponses(draft *EditDraft) []EditDraftChildResponse {
	responses := make([]EditDraftChildResponse, 0, len(draft.Children))
	for _, child := range draft.Children {
		offeringLinks := draft.OfferingsByChild[child.ID]
		if !draft.CareOfferingsEnabled && !ChildTakenOver(child) {
			offeringLinks = nil
		}
		responses = append(responses, toEditDraftChildResponse(child, offeringLinks))
	}
	return responses
}

func toEditDraftChildResponse(child *RequestChild, offeringLinks []*RequestChildOffering) EditDraftChildResponse {
	response := EditDraftChildResponse{
		ID:                strconv.FormatInt(child.ID, 10),
		FirstName:         child.FirstName,
		LastName:          child.LastName,
		DateOfBirth:       string(child.DateOfBirth),
		TargetGradeLevel:  child.TargetGradeLevel,
		TargetSchoolClass: child.TargetSchoolClass,
		CustomData:        child.CustomData,
		OfferingIDs:       []string{},
		Locked:            ChildTakenOver(child),
	}
	for _, link := range offeringLinks {
		response.OfferingIDs = append(response.OfferingIDs, strconv.FormatInt(link.CareOfferingID, 10))
		if offeringDay := toEditDraftOfferingDayResponse(link); offeringDay != nil {
			response.OfferingDays = append(response.OfferingDays, *offeringDay)
		}
	}
	return response
}

func toEditDraftOfferingDayResponse(link *RequestChildOffering) *EditDraftOfferingDayResponse {
	if len(link.SelectedDays) == 0 {
		return nil
	}
	manualDays := link.ManualSelectedDays
	if len(manualDays) == 0 && len(link.AutomaticSelectedDays) == 0 {
		manualDays = link.SelectedDays
	}
	return &EditDraftOfferingDayResponse{
		OfferingID:            strconv.FormatInt(link.CareOfferingID, 10),
		SelectedDays:          link.SelectedDays,
		ManualSelectedDays:    manualDays,
		AutomaticSelectedDays: link.AutomaticSelectedDays,
	}
}

// EditPatchRequest is the wire shape for PATCH /requests/{token}.
type EditPatchRequest struct {
	GuardianFirstName *string        `json:"guardian_first_name,omitempty"`
	GuardianLastName  *string        `json:"guardian_last_name,omitempty"`
	GuardianPhone     *string        `json:"guardian_phone,omitempty"`
	ConsentFlags      map[string]any `json:"consent_flags,omitempty"`
	CustomData        map[string]any `json:"custom_data,omitempty"`
}

// Bind makes EditPatchRequest a render.Binder so the chi binder helper
// can decode request bodies into it without ad-hoc json.Decoder code.
func (req *EditPatchRequest) Bind(_ *http.Request) error { return nil }

func (rs *Resource) patchStatus(w http.ResponseWriter, r *http.Request) {
	if rs.RequestService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("enrollment request service not configured")))
		return
	}
	token := strings.TrimSpace(chi.URLParam(r, "statusToken"))
	if token == "" {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(errors.New("status token is required"), common.CodeEnrollmentStatusLinkInvalid))
		return
	}
	patchReq := &EditPatchRequest{}
	if err := render.Bind(r, patchReq); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	patch := EditPatch{
		GuardianFirstName: patchReq.GuardianFirstName,
		GuardianLastName:  patchReq.GuardianLastName,
		GuardianPhone:     patchReq.GuardianPhone,
		ConsentFlags:      patchReq.ConsentFlags,
		CustomData:        patchReq.CustomData,
	}
	err := withinAdmin(r.Context(), func(adminCtx context.Context) error {
		return rs.RequestService.Edit(adminCtx, token, patch)
	})
	if err != nil {
		switch {
		case errors.Is(err, capability.ErrRequestNotFound):
			common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentStatusLinkInvalid))
		case errors.Is(err, capability.ErrEditNotAllowed):
			common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeEnrollmentEditNotAllowed))
		case errors.Is(err, capability.ErrInvalidGuardianPhone):
			common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeEnrollmentInvalidPhone))
		default:
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]string{"message": "updated"}, "Request updated")
}

func (rs *Resource) replaceStatus(w http.ResponseWriter, r *http.Request) {
	if rs.RequestService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("enrollment request service not configured")))
		return
	}
	token := strings.TrimSpace(chi.URLParam(r, "statusToken"))
	if token == "" {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(errors.New("status token is required"), common.CodeEnrollmentStatusLinkInvalid))
		return
	}
	wireReq := &SubmitEnrollmentRequest{}
	if err := render.Bind(r, wireReq); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	serviceReq, err := buildServiceRequest(wireReq, 0, "")
	if err != nil {
		mapSubmitError(w, r, err)
		return
	}
	result, err := rs.RequestService.ReplaceEditable(r.Context(), token, serviceReq)
	if err != nil {
		mapEditError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, SubmitEnrollmentResponse{
		RequestID: strconv.FormatInt(result.Request.ID, 10),
		StatusURL: result.StatusURL,
		Warnings:  result.Warnings,
	}, "Request updated")
}

func mapEditError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, capability.ErrRequestNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentStatusLinkInvalid))
	case errors.Is(err, capability.ErrEditNotAllowed):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeEnrollmentEditNotAllowed))
	default:
		mapSubmitError(w, r, err)
	}
}

// WithdrawRequest is the wire shape for POST /requests/{token}/withdraw.
// Optional child_id; omit to withdraw every non-terminal child.
type WithdrawRequest struct {
	ChildID *string `json:"child_id,omitempty"`
}

func (req *WithdrawRequest) Bind(_ *http.Request) error { return nil }

func (rs *Resource) withdrawStatus(w http.ResponseWriter, r *http.Request) {
	if rs.RequestService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("enrollment request service not configured")))
		return
	}
	token := strings.TrimSpace(chi.URLParam(r, "statusToken"))
	if token == "" {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(errors.New("status token is required"), common.CodeEnrollmentStatusLinkInvalid))
		return
	}

	body := &WithdrawRequest{}
	// The body is optional; render.Bind requires a body but we tolerate
	// empty payloads (omit child_id = withdraw all). Decode manually.
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(body); err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
	}
	var childID int64
	if body.ChildID != nil && *body.ChildID != "" {
		v, err := strconv.ParseInt(*body.ChildID, 10, 64)
		if err != nil || v <= 0 {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid child_id")))
			return
		}
		childID = v
	}

	err := rs.RequestService.Withdraw(r.Context(), token, childID)
	if err != nil {
		switch {
		case errors.Is(err, capability.ErrRequestNotFound):
			common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentStatusLinkInvalid))
		case errors.Is(err, capability.ErrWithdrawNotAllowed):
			common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeEnrollmentWithdrawNotAllowed))
		default:
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}
	common.RespondNoContent(w, r)
}

// ConfirmRenewalResponse reports how many child rows were transitioned
// from pending_renewal to submitted.
type ConfirmRenewalResponse struct {
	Confirmed int `json:"confirmed"`
}

// confirmRenewal handles POST /requests/{statusToken}/confirm-renewal.
// Public — gated only by status token possession, the same way the
// other parent-facing /requests endpoints are.
func (rs *Resource) confirmRenewal(w http.ResponseWriter, r *http.Request) {
	if rs.RequestService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("enrollment request service not configured")))
		return
	}
	token := strings.TrimSpace(chi.URLParam(r, "statusToken"))
	if token == "" {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(errors.New("status token is required"), common.CodeEnrollmentStatusLinkInvalid))
		return
	}

	var confirmed int
	err := withinAdmin(r.Context(), func(adminCtx context.Context) error {
		c, runErr := rs.RequestService.ConfirmRenewal(adminCtx, token)
		confirmed = c
		return runErr
	})
	if err != nil {
		switch {
		case errors.Is(err, capability.ErrRequestNotFound):
			common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentStatusLinkInvalid))
		default:
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}
	common.Respond(w, r, http.StatusOK, ConfirmRenewalResponse{Confirmed: confirmed}, "Renewal confirmed")
}
