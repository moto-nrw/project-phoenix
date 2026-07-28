package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/email"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Error messages (S1192 - avoid duplicate string literals)
const errInvitationServiceUnavailable = "invitation service unavailable"

type CreateInvitationRequest struct {
	Email     string `json:"email"`
	RoleID    int64  `json:"role_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Position  string `json:"position"`
}

func (req *CreateInvitationRequest) Bind(_ *http.Request) error {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.Position = strings.TrimSpace(req.Position)

	return validation.ValidateStruct(req,
		validation.Field(&req.Email, validation.Required, is.Email),
		validation.Field(&req.RoleID, validation.Required, validation.Min(int64(1))),
		validation.Field(&req.FirstName, validation.Length(0, 100)),
		validation.Field(&req.LastName, validation.Length(0, 100)),
		validation.Field(&req.Position, validation.Length(0, 100)),
	)
}

type InvitationResponse struct {
	ID              int64      `json:"id"`
	Email           string     `json:"email"`
	RoleID          int64      `json:"role_id"`
	RoleName        string     `json:"role_name,omitempty"`
	Token           string     `json:"token"`
	ExpiresAt       time.Time  `json:"expires_at"`
	FirstName       *string    `json:"first_name,omitempty"`
	LastName        *string    `json:"last_name,omitempty"`
	Position        *string    `json:"position,omitempty"`
	CreatedBy       int64      `json:"created_by"`
	Creator         string     `json:"creator,omitempty"`
	DeliveryStatus  string     `json:"delivery_status"`
	EmailSentAt     *time.Time `json:"email_sent_at,omitempty"`
	EmailError      *string    `json:"email_error,omitempty"`
	EmailRetryCount int        `json:"email_retry_count"`
}

func (rs *Resource) createInvitation(w http.ResponseWriter, r *http.Request) {
	if rs.InvitationService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New(errInvitationServiceUnavailable)))
		return
	}

	req := &CreateInvitationRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	invitationReq := rs.buildInvitationRequest(r, req, claims)

	invitation, err := rs.runCreateInvitation(r.Context(), invitationReq)
	if err != nil {
		if renderCreateInvitationError(w, r, err) {
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	slog.Default().Info("invitation created",
		slog.Int64("account_id", int64(claims.ID)),
		slog.String("email", invitation.Email))

	common.Respond(w, r, http.StatusCreated, toInvitationResponse(invitation), "Invitation created successfully")
}

// buildInvitationRequest maps the wire request into the service invitation
// request, resolving the tenant display name for the invitation email and
// copying the optional name/position fields.
func (rs *Resource) buildInvitationRequest(r *http.Request, req *CreateInvitationRequest, claims jwt.AppClaims) authService.InvitationRequest {
	invitationReq := authService.InvitationRequest{
		Email:            req.Email,
		RoleID:           req.RoleID,
		CreatedBy:        int64(claims.ID),
		ActorPermissions: claims.Permissions,
	}
	if rs.SchoolService != nil {
		tenantID := tenant.FromContext(r.Context())
		if school, err := rs.SchoolService.GetSchoolByID(r.Context(), tenantID); err == nil && school != nil && !school.IsDeleted() {
			invitationReq.SchoolName = school.Name
		}
	}
	if req.FirstName != "" {
		first := req.FirstName
		invitationReq.FirstName = &first
	}
	if req.LastName != "" {
		last := req.LastName
		invitationReq.LastName = &last
	}
	if req.Position != "" {
		position := req.Position
		invitationReq.Position = &position
	}
	return invitationReq
}

// runCreateInvitation invokes the invitation service inside the tenant tx when
// a DB is wired, or directly otherwise.
func (rs *Resource) runCreateInvitation(ctx context.Context, invitationReq authService.InvitationRequest) (*authModels.InvitationToken, error) {
	if rs.db == nil {
		return rs.InvitationService.CreateInvitation(ctx, invitationReq)
	}
	var invitation *authModels.InvitationToken
	err := tenant.WithTenantTx(ctx, rs.db, tenant.FromContext(ctx), func(txCtx context.Context, _ bun.Tx) error {
		inv, txErr := rs.InvitationService.CreateInvitation(txCtx, invitationReq)
		invitation = inv
		return txErr
	})
	return invitation, err
}

// renderCreateInvitationError maps the create-specific conflict errors before
// delegating to the shared invitation error renderer. Returns true if handled.
func renderCreateInvitationError(w http.ResponseWriter, r *http.Request, err error) bool {
	if errors.Is(err, authService.ErrEmailAlreadyExists) {
		common.RenderError(w, r, common.ErrorConflict(authService.ErrEmailAlreadyExists))
		return true
	}
	if errors.Is(err, authService.ErrAccountAlreadyHasTenantAccess) {
		common.RenderError(w, r, common.ErrorConflictWithCode(authService.ErrAccountAlreadyHasTenantAccess, "ACCOUNT_ALREADY_HAS_TENANT_ACCESS"))
		return true
	}
	switch {
	case errors.Is(err, authService.ErrRoleGrantNotPermitted):
		common.RenderError(w, r, common.ErrorForbidden(authService.ErrRoleGrantNotPermitted))
		return true
	case errors.Is(err, authService.ErrRoleNotAssignable),
		errors.Is(err, authService.ErrRoleForeignTenant),
		errors.Is(err, authService.ErrRoleGuardianNotAssignable),
		errors.Is(err, authService.ErrRoleLegacyTeacherNotAssignable):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return true
	}
	return renderInvitationError(w, r, err)
}

// toInvitationResponse maps an invitation token to its wire response shape.
func toInvitationResponse(invitation *authModels.InvitationToken) InvitationResponse {
	resp := InvitationResponse{
		ID:              invitation.ID,
		Email:           invitation.Email,
		RoleID:          invitation.RoleID,
		Token:           invitation.Token,
		ExpiresAt:       invitation.ExpiresAt,
		FirstName:       invitation.FirstName,
		LastName:        invitation.LastName,
		Position:        invitation.Position,
		CreatedBy:       invitationCreatedByValue(invitation.CreatedBy),
		DeliveryStatus:  deriveDeliveryStatus(invitation.EmailSentAt, invitation.EmailError),
		EmailSentAt:     invitation.EmailSentAt,
		EmailError:      invitation.EmailError,
		EmailRetryCount: invitation.EmailRetryCount,
	}
	if invitation.Role != nil {
		resp.RoleName = invitation.Role.Name
	}
	if invitation.Creator != nil {
		resp.Creator = invitation.Creator.Email
	}
	return resp
}

func (rs *Resource) validateInvitation(w http.ResponseWriter, r *http.Request) {
	if rs.InvitationService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New(errInvitationServiceUnavailable)))
		return
	}

	token := strings.TrimSpace(chi.URLParam(r, "token"))
	slog.Default().Info("invitation validation requested")

	// Public route — no JWT/tenant context. Use WithAdminTx (BYPASSRLS) to read invitation_tokens.
	var result *authService.InvitationValidationResult
	var err error
	if rs.db != nil {
		err = tenant.WithAdminTx(r.Context(), rs.db, func(txCtx context.Context, _ bun.Tx) error {
			var txErr error
			result, txErr = rs.InvitationService.ValidateInvitation(txCtx, token)
			return txErr
		})
	} else {
		result, err = rs.InvitationService.ValidateInvitation(r.Context(), token)
	}
	if err != nil {
		if renderInvitationError(w, r, err) {
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, result, "Invitation validated successfully")
}

type AcceptInvitationRequest struct {
	FirstName       string `json:"first_name"`
	LastName        string `json:"last_name"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

func (req *AcceptInvitationRequest) Bind(_ *http.Request) error {
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)

	return validation.ValidateStruct(req,
		validation.Field(&req.Password, validation.Required),
		validation.Field(&req.ConfirmPassword, validation.Required),
	)
}

type AcceptInvitationResponse struct {
	AccountID       int64  `json:"account_id"`
	Email           string `json:"email"`
	TenantSubdomain string `json:"tenant_subdomain,omitempty"`
}

// renderAcceptError maps service-layer errors to HTTP responses.
// Returns true if the error was handled.
func renderAcceptError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, authService.ErrPasswordTooWeak),
		errors.Is(err, authService.ErrPasswordMismatch):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.Is(err, authService.ErrEmailAlreadyExists):
		common.RenderError(w, r, common.ErrorConflict(authService.ErrEmailAlreadyExists))
	case errors.Is(err, authService.ErrInvitationNameRequired):
		common.RenderError(w, r, common.ErrorInvalidRequest(authService.ErrInvitationNameRequired))
	case errors.Is(err, authService.ErrInvitationTenantDeleted):
		common.RenderError(w, r, common.ErrorNotFound(authService.ErrInvitationTenantDeleted))
	case renderInvitationError(w, r, err):
		// handled by renderInvitationError
	default:
		return false
	}
	return true
}

func (rs *Resource) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	if rs.InvitationService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New(errInvitationServiceUnavailable)))
		return
	}

	token := strings.TrimSpace(chi.URLParam(r, "token"))

	req := &AcceptInvitationRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	userData := authService.UserRegistrationData{
		FirstName:       req.FirstName,
		LastName:        req.LastName,
		Password:        req.Password,
		ConfirmPassword: req.ConfirmPassword,
	}

	// Public route — no JWT/tenant context. Use WithAdminTx (BYPASSRLS) so the service's
	// inner RunInTx reuses the admin tx from context (TxHandler.GetTx checks context first).
	var account *authModels.Account
	var err error
	if rs.db != nil {
		err = tenant.WithAdminTx(r.Context(), rs.db, func(txCtx context.Context, _ bun.Tx) error {
			var txErr error
			account, txErr = rs.InvitationService.AcceptInvitation(txCtx, token, userData)
			return txErr
		})
	} else {
		account, err = rs.InvitationService.AcceptInvitation(r.Context(), token, userData)
	}
	if err != nil {
		if !renderAcceptError(w, r, err) {
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}

	slog.Default().Info("invitation accepted",
		slog.Int64("account_id", account.ID))

	resp := AcceptInvitationResponse{
		AccountID: account.ID,
		Email:     account.Email,
	}
	if rs.SchoolService != nil && rs.db != nil {
		if subdomain := rs.lookupTenantSubdomainForInvitation(r.Context(), token); subdomain != "" {
			resp.TenantSubdomain = subdomain
		}
	}
	common.Respond(w, r, http.StatusCreated, resp, "Invitation accepted successfully")
}

// lookupTenantSubdomainForInvitation resolves the tenant subdomain from an
// invitation token via the invitation service. Best-effort: returns "" on any
// error so the accept response still succeeds.
func (rs *Resource) lookupTenantSubdomainForInvitation(ctx context.Context, token string) string {
	return rs.InvitationService.GetTenantSubdomainForToken(ctx, token)
}

func (rs *Resource) listPendingInvitations(w http.ResponseWriter, r *http.Request) {
	if rs.InvitationService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New(errInvitationServiceUnavailable)))
		return
	}

	var invitations []*authModels.InvitationToken
	ctx := r.Context()
	var err error
	if rs.db != nil {
		err = tenant.WithTenantTx(ctx, rs.db, tenant.FromContext(ctx), func(txCtx context.Context, _ bun.Tx) error {
			inv, txErr := rs.InvitationService.ListPendingInvitations(txCtx)
			invitations = inv
			return txErr
		})
	} else {
		invitations, err = rs.InvitationService.ListPendingInvitations(ctx)
	}
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	responses := make([]InvitationResponse, 0, len(invitations))
	for _, invitation := range invitations {
		responses = append(responses, toInvitationResponse(invitation))
	}

	common.Respond(w, r, http.StatusOK, responses, "Pending invitations retrieved successfully")
}

func deriveDeliveryStatus(sentAt *time.Time, emailError *string) string {
	if sentAt != nil {
		return string(email.DeliveryStatusSent)
	}
	if emailError != nil && strings.TrimSpace(*emailError) != "" {
		return string(email.DeliveryStatusFailed)
	}
	return string(email.DeliveryStatusPending)
}

func invitationCreatedByValue(createdBy *int64) int64 {
	if createdBy == nil {
		return 0
	}
	return *createdBy
}

func (rs *Resource) resendInvitation(w http.ResponseWriter, r *http.Request) {
	if rs.InvitationService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New(errInvitationServiceUnavailable)))
		return
	}
	rs.resendInvitationHandler(w, r,
		rs.InvitationService.ResendInvitation,
		"invitation resend requested", "Invitation resent", "Invitation resent successfully")
}

// resendInvitationHandler is the shared body of the staff and guardian
// invitation resend endpoints: parse id, run the resend inside the tenant
// tx (when a DB is wired), map expired/known errors, log, respond.
// The response strings are passed verbatim per endpoint. Callers must
// nil-check their service before delegating (svcCall is a bound method).
func (rs *Resource) resendInvitationHandler(w http.ResponseWriter, r *http.Request, svcCall func(ctx context.Context, invitationID, actorID int64) error, logMsg, message, respondMsg string) {
	idParam := chi.URLParam(r, "id")
	invitationID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid invitation id")))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())

	ctx := r.Context()
	if rs.db != nil {
		err = tenant.WithTenantTx(ctx, rs.db, tenant.FromContext(ctx), func(txCtx context.Context, _ bun.Tx) error {
			return svcCall(txCtx, invitationID, int64(claims.ID))
		})
	} else {
		err = svcCall(ctx, invitationID, int64(claims.ID))
	}
	if err != nil {
		if errors.Is(err, authService.ErrInvitationExpired) {
			common.RenderError(w, r, common.ErrorInvalidRequest(authService.ErrInvitationExpired))
			return
		}
		if renderInvitationError(w, r, err) {
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	slog.Default().Info(logMsg,
		slog.Int64("invitation_id", invitationID),
		slog.Int64("account_id", int64(claims.ID)))
	common.Respond(w, r, http.StatusOK, map[string]string{"message": message}, respondMsg)
}

func (rs *Resource) revokeInvitation(w http.ResponseWriter, r *http.Request) {
	if rs.InvitationService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New(errInvitationServiceUnavailable)))
		return
	}

	idParam := chi.URLParam(r, "id")
	invitationID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid invitation id")))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())

	ctx := r.Context()
	var revokeErr error
	if rs.db != nil {
		revokeErr = tenant.WithTenantTx(ctx, rs.db, tenant.FromContext(ctx), func(txCtx context.Context, _ bun.Tx) error {
			return rs.InvitationService.RevokeInvitation(txCtx, invitationID, int64(claims.ID))
		})
	} else {
		revokeErr = rs.InvitationService.RevokeInvitation(ctx, invitationID, int64(claims.ID))
	}
	if revokeErr != nil {
		if renderInvitationError(w, r, revokeErr) {
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(revokeErr))
		return
	}

	slog.Default().Info("invitation revoked",
		slog.Int64("invitation_id", invitationID),
		slog.Int64("account_id", int64(claims.ID)))
	common.RespondNoContent(w, r)
}

// renderInvitationError maps invitation service errors to appropriate HTTP responses.
func renderInvitationError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}

	var authErr *authService.AuthError
	if errors.As(err, &authErr) && authErr.Err != nil {
		err = authErr.Err
	}

	switch {
	case errors.Is(err, authService.ErrInvitationNotFound):
		if render.Render(w, r, common.ErrorNotFound(authService.ErrInvitationNotFound)) != nil {
			return false
		}
		return true
	case errors.Is(err, authService.ErrInvitationExpired), errors.Is(err, authService.ErrInvitationUsed):
		if render.Render(w, r, common.ErrorGone(err)) != nil {
			return false
		}
		return true
	default:
		return false
	}
}
