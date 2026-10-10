package enrollmenthttp

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type AdminCreateLateInviteRequest struct {
	GuardianEmail     string     `json:"guardian_email"`
	GuardianFirstName string     `json:"guardian_first_name,omitempty"`
	GuardianLastName  string     `json:"guardian_last_name,omitempty"`
	Reason            string     `json:"reason,omitempty"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
}

func (req *AdminCreateLateInviteRequest) Bind(_ *http.Request) error { return nil }

type AdminCreateLateInviteResponse struct {
	ID                string    `json:"id"`
	PhaseID           string    `json:"phase_id"`
	GuardianEmail     string    `json:"guardian_email"`
	GuardianFirstName *string   `json:"guardian_first_name,omitempty"`
	GuardianLastName  *string   `json:"guardian_last_name,omitempty"`
	ExpiresAt         time.Time `json:"expires_at"`
	CreatedBy         string    `json:"created_by"`
	Reason            *string   `json:"reason,omitempty"`
	Token             string    `json:"token"`
}

func (rs *Resource) createLateInvite(w http.ResponseWriter, r *http.Request) {
	if rs.RequestService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("enrollment request service not configured")))
		return
	}
	phaseID, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid phase id")
	if !ok {
		return
	}
	body := &AdminCreateLateInviteRequest{}
	if err := render.Bind(r, body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	createdBy := int64(claims.ID)
	if createdBy <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("no account ID in context")))
		return
	}

	var result *CreateLateInviteResult
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		out, createErr := rs.RequestService.CreateLateInvite(ctx, CreateLateInviteInput{
			PhaseID:           phaseID,
			GuardianEmail:     body.GuardianEmail,
			GuardianFirstName: body.GuardianFirstName,
			GuardianLastName:  body.GuardianLastName,
			Reason:            body.Reason,
			ExpiresAt:         body.ExpiresAt,
			CreatedBy:         createdBy,
		})
		if createErr != nil {
			return createErr
		}
		result = out
		return nil
	})
	if err != nil {
		mapLateInviteAdminError(w, r, err)
		return
	}

	invite := result.Invite
	common.Respond(w, r, http.StatusCreated, AdminCreateLateInviteResponse{
		ID:                strconv.FormatInt(invite.ID, 10),
		PhaseID:           strconv.FormatInt(invite.PhaseID, 10),
		GuardianEmail:     invite.GuardianEmail,
		GuardianFirstName: invite.GuardianFirstName,
		GuardianLastName:  invite.GuardianLastName,
		ExpiresAt:         invite.ExpiresAt,
		CreatedBy:         strconv.FormatInt(invite.CreatedBy, 10),
		Reason:            invite.Reason,
		Token:             result.Token,
	}, "Late invite created")
}

type AdminManualApprovedEnrollmentRequest struct {
	SubmitEnrollmentRequest
	ExternalConsentConfirmed bool   `json:"external_consent_confirmed"`
	Reason                   string `json:"reason"`
	SendNotification         bool   `json:"send_notification"`
}

func (req *AdminManualApprovedEnrollmentRequest) Bind(r *http.Request) error {
	return req.SubmitEnrollmentRequest.Bind(r)
}

type AdminManualApprovedEnrollmentResponse struct {
	RequestID string `json:"request_id"`
	ChildID   string `json:"child_id"`
	StudentID string `json:"student_id,omitempty"`
	Status    string `json:"status"`
	StatusURL string `json:"status_url"`
}

func (rs *Resource) createManualApprovedEnrollment(w http.ResponseWriter, r *http.Request) {
	if rs.RequestService == nil || rs.DecisionService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("manual enrollment services not configured")))
		return
	}
	phaseID, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid phase id")
	if !ok {
		return
	}
	body := &AdminManualApprovedEnrollmentRequest{}
	if err := render.Bind(r, body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	reason, err := manualApprovedEnrollmentReason(body)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	actorID, tenantID, ok := manualEnrollmentActor(w, r)
	if !ok {
		return
	}

	body.PhaseID = phaseID
	serviceReq, parseErr := buildServiceRequest(&body.SubmitEnrollmentRequest, tenantID, remoteIPFromRequest(r))
	if parseErr != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(parseErr))
		return
	}

	var result *ManualApprovedEnrollmentResult
	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		out, createErr := rs.RequestService.CreateManualApprovedEnrollment(ctx, ManualApprovedEnrollmentInput{
			Request:          serviceReq,
			Reason:           reason,
			SendNotification: body.SendNotification,
			ActorID:          actorID,
		})
		result = out
		return createErr
	})
	if err != nil {
		mapManualEnrollmentError(w, r, err)
		return
	}

	if result.PendingInvite != nil && rs.GuardianInvitations.configured() {
		go rs.dispatchPostDecisionInvite(r.Context(), result.PendingInvite)
	}

	common.Respond(w, r, http.StatusCreated, manualApprovedEnrollmentResponse(result), "Manual enrollment created and approved")
}

// manualEnrollmentActor reads the staff account and the school a manual
// enrollment is created for; it answers the request itself when either is
// missing.
func manualEnrollmentActor(w http.ResponseWriter, r *http.Request) (actorID, tenantID int64, ok bool) {
	claims := jwt.ClaimsFromCtx(r.Context())
	actorID = int64(claims.ID)
	if actorID <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("no account ID in context")))
		return 0, 0, false
	}
	tenantID = tenant.FromContext(r.Context())
	if tenantID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("tenant context is required")))
		return 0, 0, false
	}
	return actorID, tenantID, true
}

// manualApprovedEnrollmentReason checks what a staff-created approval must
// carry and returns its trimmed reason.
func manualApprovedEnrollmentReason(body *AdminManualApprovedEnrollmentRequest) (string, error) {
	reason := strings.TrimSpace(body.Reason)
	if reason == "" {
		return "", errors.New("reason is required")
	}
	if !body.ExternalConsentConfirmed {
		return "", errors.New("external consent confirmation is required")
	}
	if len(body.Children) != 1 {
		return "", errors.New("manual approved enrollment requires exactly one child")
	}
	return reason, nil
}

func manualApprovedEnrollmentResponse(result *ManualApprovedEnrollmentResult) AdminManualApprovedEnrollmentResponse {
	child := result.Child
	resp := AdminManualApprovedEnrollmentResponse{
		RequestID: strconv.FormatInt(result.Request.ID, 10),
		ChildID:   strconv.FormatInt(child.ID, 10),
		Status:    child.Status,
		StatusURL: result.StatusURL,
	}
	if child.CreatedStudentID != nil {
		resp.StudentID = strconv.FormatInt(*child.CreatedStudentID, 10)
	}
	return resp
}

func (rs *Resource) getManualEnrollmentBootstrap(w http.ResponseWriter, r *http.Request) {
	if rs.RequestService == nil || rs.FormSchemaService == nil || rs.CareOfferingService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("manual enrollment bootstrap services not configured")))
		return
	}
	phaseID, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid phase id")
	if !ok {
		return
	}

	var data *PublicFormBootstrapData
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		loaded, loadErr := rs.RequestService.LoadManualEnrollmentBootstrap(ctx, phaseID)
		data = loaded
		return loadErr
	})
	if err != nil {
		switch {
		case errors.Is(err, capability.ErrEnrollmentDisabled),
			errors.Is(err, capability.ErrInvalidSubmission):
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
		default:
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}

	phase := data.Phase
	texts := data.LegalTexts
	items := make([]CareOfferingResponse, 0, len(data.Offerings))
	for _, o := range data.Offerings {
		items = append(items, toPublicCareOfferingResponse(o))
	}
	capabilities := data.EffectiveCapabilities
	common.Respond(w, r, http.StatusOK, PublicEnrollmentFormBootstrapResponse{
		Phase:                     toPublicPhase(phase),
		Schema:                    toPublicFormSchemaResponse(data.Schema),
		Offerings:                 items,
		CareOfferingSelectionMode: effectiveCareOfferingSelectionMode(phase.CareOfferingSelectionMode, capabilities.CareOfferingsEnabled),
		CareRequired:              capabilities.CareOfferingsEnabled && phase.CareOfferingSelectionMode != capability.PhaseCareOfferingSelectionOptional,
		SchoolClass:               toPublicSchoolClassConfig(phase, capabilities.CollectSchoolClass),
		CollectGradeLevel:         capabilities.CollectGradeLevel,
		CareOfferingsEnabled:      capabilities.CareOfferingsEnabled,
		CaptchaConfig:             PublicCaptchaConfigResponse{},
		LegalTexts: PublicLegalTextsResponse{
			AGB:                 texts.AGB,
			AGBDocumentURL:      texts.AGBDocumentURL,
			AGBDisplayMode:      texts.AGBDisplayMode,
			DSGVO:               texts.DSGVO,
			EmailContact:        texts.EmailContact,
			Photo:               texts.Photo,
			TermsEnabled:        texts.TermsEnabled,
			DSGVOEnabled:        texts.DSGVOEnabled,
			EmailContactEnabled: texts.EmailContactEnabled,
			PhotoEnabled:        texts.PhotoEnabled,
			Blocks:              texts.Blocks,
		},
	}, "Manual enrollment bootstrap retrieved")
}

func mapLateInviteAdminError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, capability.ErrEnrollmentDisabled):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeEnrollmentDisabled))
	case errors.Is(err, capability.ErrInvalidGuardianEmail):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeEnrollmentInvalidEmail))
	case errors.Is(err, capability.ErrInvalidSubmission):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

func mapManualEnrollmentError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, capability.ErrDecisionChildNotFound),
		errors.Is(err, capability.ErrDecisionRequestNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentRequestNotFound))
	case errors.Is(err, capability.ErrDecisionAlreadyTerminal):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeEnrollmentDecisionAlreadyFinal))
	case errors.Is(err, capability.ErrDecisionInvalidData):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeEnrollmentApprovalDataInvalid))
	case errors.Is(err, capability.ErrDecisionInvalidStatus):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.Is(err, capability.ErrGuardianAccountMismatch):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeEnrollmentGuardianAccountMismatch))
	case common.IsTransientDatabaseError(err):
		common.RenderError(w, r, common.ErrorServiceUnavailable(err))
	default:
		mapSubmitError(w, r, err)
	}
}
