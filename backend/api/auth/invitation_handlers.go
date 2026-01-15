package auth

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/adapter/mailer"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
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
		common.RenderError(w, r, ErrorInternalServer(errors.New(errInvitationServiceUnavailable)))
		return
	}

	req := &CreateInvitationRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())

	invitationReq := authService.InvitationRequest{
		Email:     req.Email,
		RoleID:    req.RoleID,
		CreatedBy: int64(claims.ID),
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

	invitation, err := rs.InvitationService.CreateInvitation(r.Context(), invitationReq)
	if err != nil {
		// Check for email already exists error
		if errors.Is(err, authService.ErrEmailAlreadyExists) {
			common.RenderError(w, r, common.ErrorConflict(authService.ErrEmailAlreadyExists))
			return
		}

		if renderInvitationError(w, r, err) {
			return
		}
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	if logger.Logger != nil {
		logger.Logger.WithFields(map[string]interface{}{
			"account_id": claims.ID,
			"email":      invitation.Email,
		}).Info("Invitation created")
	}

	resp := InvitationResponse{
		ID:              invitation.ID,
		Email:           invitation.Email,
		RoleID:          invitation.RoleID,
		Token:           invitation.Token,
		ExpiresAt:       invitation.ExpiresAt,
		FirstName:       invitation.FirstName,
		LastName:        invitation.LastName,
		Position:        invitation.Position,
		CreatedBy:       invitation.CreatedBy,
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

	common.Respond(w, r, http.StatusCreated, resp, "Invitation created successfully")
}

func (rs *Resource) validateInvitation(w http.ResponseWriter, r *http.Request) {
	if rs.InvitationService == nil {
		common.RenderError(w, r, ErrorInternalServer(errors.New(errInvitationServiceUnavailable)))
		return
	}

	token := strings.TrimSpace(chi.URLParam(r, "token"))
	if logger.Logger != nil {
		logger.Logger.Debug("Invitation validation requested")
	}

	result, err := rs.InvitationService.ValidateInvitation(r.Context(), token)
	if err != nil {
		if renderInvitationError(w, r, err) {
			return
		}
		common.RenderError(w, r, ErrorInternalServer(err))
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
	AccountID int64  `json:"account_id"`
	Email     string `json:"email"`
}

func (rs *Resource) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	if rs.InvitationService == nil {
		common.RenderError(w, r, ErrorInternalServer(errors.New(errInvitationServiceUnavailable)))
		return
	}

	token := strings.TrimSpace(chi.URLParam(r, "token"))

	req := &AcceptInvitationRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	userData := authService.UserRegistrationData{
		FirstName:       req.FirstName,
		LastName:        req.LastName,
		Password:        req.Password,
		ConfirmPassword: req.ConfirmPassword,
	}

	account, err := rs.InvitationService.AcceptInvitation(r.Context(), token, userData)
	if err != nil {
		if errors.Is(err, authService.ErrPasswordTooWeak) || errors.Is(err, authService.ErrPasswordMismatch) {
			common.RenderError(w, r, ErrorInvalidRequest(err))
			return
		}

		if errors.Is(err, authService.ErrEmailAlreadyExists) {
			common.RenderError(w, r, common.ErrorConflict(authService.ErrEmailAlreadyExists))
			return
		}

		if errors.Is(err, authService.ErrInvitationNameRequired) {
			common.RenderError(w, r, ErrorInvalidRequest(authService.ErrInvitationNameRequired))
			return
		}

		if renderInvitationError(w, r, err) {
			return
		}

		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	if logger.Logger != nil {
		logger.Logger.WithFields(map[string]interface{}{
			"account_id": account.ID,
		}).Info("Invitation accepted")
	}

	resp := AcceptInvitationResponse{
		AccountID: account.ID,
		Email:     account.Email,
	}
	common.Respond(w, r, http.StatusCreated, resp, "Invitation accepted successfully")
}

func (rs *Resource) listPendingInvitations(w http.ResponseWriter, r *http.Request) {
	if rs.InvitationService == nil {
		common.RenderError(w, r, ErrorInternalServer(errors.New(errInvitationServiceUnavailable)))
		return
	}

	invitations, err := rs.InvitationService.ListPendingInvitations(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	responses := make([]InvitationResponse, 0, len(invitations))
	for _, invitation := range invitations {
		resp := InvitationResponse{
			ID:              invitation.ID,
			Email:           invitation.Email,
			RoleID:          invitation.RoleID,
			Token:           invitation.Token,
			ExpiresAt:       invitation.ExpiresAt,
			FirstName:       invitation.FirstName,
			LastName:        invitation.LastName,
			Position:        invitation.Position,
			CreatedBy:       invitation.CreatedBy,
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
		responses = append(responses, resp)
	}

	common.Respond(w, r, http.StatusOK, responses, "Pending invitations retrieved successfully")
}

func deriveDeliveryStatus(sentAt *time.Time, emailError *string) string {
	if sentAt != nil {
		return string(mailer.DeliveryStatusSent)
	}
	if emailError != nil && strings.TrimSpace(*emailError) != "" {
		return string(mailer.DeliveryStatusFailed)
	}
	return string(mailer.DeliveryStatusPending)
}

func (rs *Resource) resendInvitation(w http.ResponseWriter, r *http.Request) {
	if rs.InvitationService == nil {
		common.RenderError(w, r, ErrorInternalServer(errors.New(errInvitationServiceUnavailable)))
		return
	}

	idParam := chi.URLParam(r, "id")
	invitationID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid invitation id")))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())

	err = rs.InvitationService.ResendInvitation(r.Context(), invitationID, int64(claims.ID))
	if err != nil {
		if errors.Is(err, authService.ErrInvitationExpired) {
			common.RenderError(w, r, ErrorInvalidRequest(authService.ErrInvitationExpired))
			return
		}
		if renderInvitationError(w, r, err) {
			return
		}
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	if logger.Logger != nil {
		logger.Logger.WithFields(map[string]interface{}{
			"invitation_id": invitationID,
			"account_id":    claims.ID,
		}).Info("Invitation resend requested")
	}
	common.Respond(w, r, http.StatusOK, map[string]string{"message": "Invitation resent"}, "Invitation resent successfully")
}

func (rs *Resource) revokeInvitation(w http.ResponseWriter, r *http.Request) {
	if rs.InvitationService == nil {
		common.RenderError(w, r, ErrorInternalServer(errors.New(errInvitationServiceUnavailable)))
		return
	}

	idParam := chi.URLParam(r, "id")
	invitationID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid invitation id")))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())

	if err := rs.InvitationService.RevokeInvitation(r.Context(), invitationID, int64(claims.ID)); err != nil {
		if renderInvitationError(w, r, err) {
			return
		}
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	if logger.Logger != nil {
		logger.Logger.WithFields(map[string]interface{}{
			"invitation_id": invitationID,
			"account_id":    claims.ID,
		}).Info("Invitation revoked")
	}
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
