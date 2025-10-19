package auth

import (
	"errors"
	"log"
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
	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

type CreateInvitationRequest struct {
	Email     string `json:"email"`
	RoleID    int64  `json:"role_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func (req *CreateInvitationRequest) Bind(r *http.Request) error {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)

	return validation.ValidateStruct(req,
		validation.Field(&req.Email, validation.Required, is.Email),
		validation.Field(&req.RoleID, validation.Required, validation.Min(int64(1))),
	)
}

type InvitationResponse struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	RoleID    int64     `json:"role_id"`
	RoleName  string    `json:"role_name,omitempty"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	FirstName *string   `json:"first_name,omitempty"`
	LastName  *string   `json:"last_name,omitempty"`
	CreatedBy int64     `json:"created_by"`
	Creator   string    `json:"creator,omitempty"`
}

func (rs *Resource) createInvitation(w http.ResponseWriter, r *http.Request) {
	if rs.InvitationService == nil {
		if err := render.Render(w, r, ErrorInternalServer(errors.New("invitation service unavailable"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	req := &CreateInvitationRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
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

	invitation, err := rs.InvitationService.CreateInvitation(r.Context(), invitationReq)
	if err != nil {
		if renderInvitationError(w, r, err) {
			return
		}
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	log.Printf("Invitation created by account=%d for email=%s", claims.ID, invitation.Email)

	resp := InvitationResponse{
		ID:        invitation.ID,
		Email:     invitation.Email,
		RoleID:    invitation.RoleID,
		Token:     invitation.Token,
		ExpiresAt: invitation.ExpiresAt,
		FirstName: invitation.FirstName,
		LastName:  invitation.LastName,
		CreatedBy: invitation.CreatedBy,
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
		if err := render.Render(w, r, ErrorInternalServer(errors.New("invitation service unavailable"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	token := strings.TrimSpace(chi.URLParam(r, "token"))
	log.Printf("Invitation validation requested token=%s", token)

	result, err := rs.InvitationService.ValidateInvitation(r.Context(), token)
	if err != nil {
		if renderInvitationError(w, r, err) {
			return
		}
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
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

func (req *AcceptInvitationRequest) Bind(r *http.Request) error {
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.Password = strings.TrimSpace(req.Password)
	req.ConfirmPassword = strings.TrimSpace(req.ConfirmPassword)

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
		if err := render.Render(w, r, ErrorInternalServer(errors.New("invitation service unavailable"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	token := strings.TrimSpace(chi.URLParam(r, "token"))

	req := &AcceptInvitationRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
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
			if renderErr := render.Render(w, r, ErrorInvalidRequest(err)); renderErr != nil {
				log.Printf("Render error: %v", renderErr)
			}
			return
		}

		if errors.Is(err, authService.ErrEmailAlreadyExists) {
			if renderErr := render.Render(w, r, common.ErrorConflict(authService.ErrEmailAlreadyExists)); renderErr != nil {
				log.Printf("Render error: %v", renderErr)
			}
			return
		}

		if renderInvitationError(w, r, err) {
			return
		}

		if renderErr := render.Render(w, r, ErrorInternalServer(err)); renderErr != nil {
			log.Printf("Render error: %v", renderErr)
		}
		return
	}

	log.Printf("Invitation accepted token=%s account=%d", token, account.ID)

	resp := AcceptInvitationResponse{
		AccountID: account.ID,
		Email:     account.Email,
	}
	common.Respond(w, r, http.StatusCreated, resp, "Invitation accepted successfully")
}

func (rs *Resource) listPendingInvitations(w http.ResponseWriter, r *http.Request) {
	if rs.InvitationService == nil {
		if err := render.Render(w, r, ErrorInternalServer(errors.New("invitation service unavailable"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	invitations, err := rs.InvitationService.ListPendingInvitations(r.Context())
	if err != nil {
		if renderErr := render.Render(w, r, ErrorInternalServer(err)); renderErr != nil {
			log.Printf("Render error: %v", renderErr)
		}
		return
	}

	responses := make([]InvitationResponse, 0, len(invitations))
	for _, invitation := range invitations {
		resp := InvitationResponse{
			ID:        invitation.ID,
			Email:     invitation.Email,
			RoleID:    invitation.RoleID,
			Token:     invitation.Token,
			ExpiresAt: invitation.ExpiresAt,
			FirstName: invitation.FirstName,
			LastName:  invitation.LastName,
			CreatedBy: invitation.CreatedBy,
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

func (rs *Resource) resendInvitation(w http.ResponseWriter, r *http.Request) {
	if rs.InvitationService == nil {
		if err := render.Render(w, r, ErrorInternalServer(errors.New("invitation service unavailable"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	idParam := chi.URLParam(r, "id")
	invitationID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		if renderErr := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid invitation id"))); renderErr != nil {
			log.Printf("Render error: %v", renderErr)
		}
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())

	err = rs.InvitationService.ResendInvitation(r.Context(), invitationID, int64(claims.ID))
	if err != nil {
		if errors.Is(err, authService.ErrInvitationExpired) {
			if renderErr := render.Render(w, r, ErrorInvalidRequest(authService.ErrInvitationExpired)); renderErr != nil {
				log.Printf("Render error: %v", renderErr)
			}
			return
		}
		if renderInvitationError(w, r, err) {
			return
		}
		if renderErr := render.Render(w, r, ErrorInternalServer(err)); renderErr != nil {
			log.Printf("Render error: %v", renderErr)
		}
		return
	}

	log.Printf("Invitation resend requested id=%d by account=%d", invitationID, claims.ID)
	common.Respond(w, r, http.StatusOK, map[string]string{"message": "Invitation resent"}, "Invitation resent successfully")
}

func (rs *Resource) revokeInvitation(w http.ResponseWriter, r *http.Request) {
	if rs.InvitationService == nil {
		if err := render.Render(w, r, ErrorInternalServer(errors.New("invitation service unavailable"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	idParam := chi.URLParam(r, "id")
	invitationID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		if renderErr := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid invitation id"))); renderErr != nil {
			log.Printf("Render error: %v", renderErr)
		}
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())

	if err := rs.InvitationService.RevokeInvitation(r.Context(), invitationID, int64(claims.ID)); err != nil {
		if renderInvitationError(w, r, err) {
			return
		}
		if renderErr := render.Render(w, r, ErrorInternalServer(err)); renderErr != nil {
			log.Printf("Render error: %v", renderErr)
		}
		return
	}

	log.Printf("Invitation revoked id=%d by account=%d", invitationID, claims.ID)
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
		if renderErr := render.Render(w, r, common.ErrorNotFound(authService.ErrInvitationNotFound)); renderErr != nil {
			return false
		}
		return true
	case errors.Is(err, authService.ErrInvitationExpired), errors.Is(err, authService.ErrInvitationUsed):
		if renderErr := render.Render(w, r, common.ErrorGone(err)); renderErr != nil {
			return false
		}
		return true
	default:
		return false
	}
}
