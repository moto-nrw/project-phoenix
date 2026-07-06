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
	serviceReq, parseErr := buildServiceRequest(&body.SubmitEnrollmentRequest, tenantID, remoteIPFromRequest(r))
	if parseErr != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(parseErr))
		return
	}
	serviceReq.SkipRateLimit = true
	serviceReq.AllowClosedPhase = true
	serviceReq.SuppressSubmissionEmails = true
	serviceReq.ExternalConsentConfirmed = true
	serviceReq.SubmissionSource = enrollmentModels.RequestSourceAdminManual
	serviceReq.SourceMetadata = map[string]any{
		"external_consent_confirmed": true,
		"admin_reason":               reason,
		"send_notification":          body.SendNotification,
		"actor_account_id":           actorID,
	}

	var (
		submitResult *enrollmentService.SubmitResult
		outcome      *enrollmentService.DecideOutcome
	)
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		submitted, submitErr := rs.RequestService.Submit(ctx, serviceReq)
		if submitErr != nil {
			return submitErr
		}
		if len(submitted.Children) != 1 {
			return fmtInvalidManualEnrollmentChildCount(len(submitted.Children))
		}
		decided, decideErr := rs.DecisionService.Decide(ctx, enrollmentService.DecideInput{
			RequestID:                  submitted.Request.ID,
			ChildID:                    submitted.Children[0].ID,
			Status:                     enrollmentService.DecisionApproved,
			Reason:                     reason,
			ReviewedBy:                 actorID,
			SuppressParentEmail:        !body.SendNotification,
			SuppressGuardianInvitation: !body.SendNotification,
		})
		if decideErr != nil {
			return decideErr
		}
		submitResult = submitted
		outcome = decided
		return nil
	})
	if err != nil {
		mapManualEnrollmentError(w, r, err)
		return
	}

	if outcome != nil && outcome.PendingInvite != nil && rs.GuardianInvitationService != nil {
		go rs.dispatchPostDecisionInvite(r.Context(), outcome.PendingInvite)
	}

	child := outcome.Child
	resp := AdminManualApprovedEnrollmentResponse{
		RequestID: strconv.FormatInt(submitResult.Request.ID, 10),
		ChildID:   strconv.FormatInt(child.ID, 10),
		Status:    child.Status,
		StatusURL: submitResult.StatusURL,
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

	var (
		phase     *enrollmentModels.Phase
		schema    *enrollmentModels.FormSchema
		offerings []*enrollmentModels.CareOffering
		texts     enrollmentService.LegalTexts
	)
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		loadedPhase, phaseErr := rs.RequestService.LoadManualEnrollmentPhase(ctx, phaseID)
		if phaseErr != nil {
			return phaseErr
		}
		phase = loadedPhase
		if phase.FormSchemaID != nil {
			loadedSchema, schemaErr := rs.FormSchemaService.GetByID(ctx, *phase.FormSchemaID)
			if schemaErr != nil {
				if !errors.Is(schemaErr, enrollmentService.ErrFormSchemaNotFound) {
					return schemaErr
				}
			} else {
				schema = loadedSchema
			}
		}
		list, listErr := rs.CareOfferingService.ListActiveByPhase(ctx, phaseID)
		if listErr != nil {
			return listErr
		}
		offerings = list
		legalTexts, legalErr := rs.RequestService.LegalTextsForManualEnrollmentPhase(ctx, phaseID)
		if legalErr != nil {
			return legalErr
		}
		texts = legalTexts
		return nil
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

func fmtInvalidManualEnrollmentChildCount(count int) error {
	return errors.New("manual approved enrollment produced " + strconv.Itoa(count) + " children")
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
	case common.IsTransientDatabaseError(err):
		common.RenderError(w, r, common.ErrorServiceUnavailable(err))
	default:
		mapSubmitError(w, r, err)
	}
}
