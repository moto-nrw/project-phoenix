package enrollment

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
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

	var result *enrollmentService.CreateLateInviteResult
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		out, createErr := rs.RequestService.CreateLateInvite(ctx, enrollmentService.CreateLateInviteInput{
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
	reason := strings.TrimSpace(body.Reason)
	if reason == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("reason is required")))
		return
	}
	if !body.ExternalConsentConfirmed {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("external consent confirmation is required")))
		return
	}
	if len(body.Children) != 1 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("manual approved enrollment requires exactly one child")))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	actorID := int64(claims.ID)
	if actorID <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("no account ID in context")))
		return
	}
	tenantID := tenant.FromContext(r.Context())
	if tenantID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("tenant context is required")))
		return
	}

	body.PhaseID = phaseID
	serviceReq, parseErr := BuildServiceRequest(&body.SubmitEnrollmentRequest, tenantID, remoteIPFromRequest(r))
	if parseErr != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(parseErr))
		return
	}

	var result *enrollmentService.ManualApprovedEnrollmentResult
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		out, createErr := rs.RequestService.CreateManualApprovedEnrollment(ctx, enrollmentService.ManualApprovedEnrollmentInput{
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

	if result.PendingInvite != nil && rs.GuardianInvitationService != nil {
		go rs.dispatchPostDecisionInvite(r.Context(), result.PendingInvite)
	}

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
	common.Respond(w, r, http.StatusCreated, resp, "Manual enrollment created and approved")
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

	var data *enrollmentService.PublicFormBootstrapData
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		loaded, loadErr := rs.RequestService.LoadManualEnrollmentBootstrap(ctx, phaseID)
		data = loaded
		return loadErr
	})
	if err != nil {
		switch {
		case errors.Is(err, enrollmentService.ErrEnrollmentDisabled),
			errors.Is(err, enrollmentService.ErrInvalidSubmission):
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
		items = append(items, toCareOfferingResponse(o))
	}
	capabilities := enrollmentService.EffectiveFormCapabilities(data.Capabilities, data.Offerings)
	common.Respond(w, r, http.StatusOK, PublicEnrollmentFormBootstrapResponse{
		Phase:                     toPublicPhase(phase),
		Schema:                    toPublicFormSchemaResponse(data.Schema),
		Offerings:                 items,
		CareOfferingSelectionMode: effectiveCareOfferingSelectionMode(phase.CareOfferingSelectionMode, capabilities.CareOfferingsEnabled),
		CareRequired:              capabilities.CareOfferingsEnabled && phase.CareOfferingSelectionMode != enrollmentModels.PhaseCareOfferingSelectionOptional,
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
	case errors.Is(err, enrollmentService.ErrEnrollmentDisabled),
		errors.Is(err, enrollmentService.ErrInvalidSubmission),
		errors.Is(err, enrollmentService.ErrInvalidGuardianEmail):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

func mapManualEnrollmentError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, enrollmentService.ErrDecisionChildNotFound),
		errors.Is(err, enrollmentService.ErrDecisionRequestNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, enrollmentService.ErrDecisionInvalidStatus),
		errors.Is(err, enrollmentService.ErrDecisionAlreadyTerminal),
		errors.Is(err, enrollmentService.ErrDecisionInvalidData):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.Is(err, enrollmentService.ErrGuardianAccountMismatch):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "enrollment.guardian_account_mismatch"))
	case common.IsTransientDatabaseError(err):
		common.RenderError(w, r, common.ErrorServiceUnavailable(err))
	default:
		MapSubmitError(w, r, err)
	}
}
