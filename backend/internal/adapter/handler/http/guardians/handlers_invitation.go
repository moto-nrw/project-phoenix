package guardians

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	guardianSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// GuardianInvitationAcceptRequest represents a request to accept a guardian invitation
type GuardianInvitationAcceptRequest struct {
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

// Bind validates the invitation accept request
func (req *GuardianInvitationAcceptRequest) Bind(_ *http.Request) error {
	if req.Password == "" {
		return errors.New("password is required")
	}
	if req.ConfirmPassword == "" {
		return errors.New("confirm_password is required")
	}
	if req.Password != req.ConfirmPassword {
		return errors.New("passwords do not match")
	}
	return nil
}

// sendInvitation handles sending an invitation to a guardian
func (rs *Resource) sendInvitation(w http.ResponseWriter, r *http.Request) {
	// Parse guardian ID from URL
	guardianID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errInvalidGuardianID)))
		return
	}

	// Get current user ID
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("user not authenticated")))
		return
	}
	accountID := int64(claims.ID)

	// Send invitation
	invitationReq := guardianSvc.GuardianInvitationRequest{
		GuardianProfileID: guardianID,
		CreatedBy:         accountID,
	}

	invitation, err := rs.GuardianService.SendInvitation(r.Context(), invitationReq)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Return invitation details (without token for security)
	response := map[string]interface{}{
		"id":                  invitation.ID,
		"guardian_profile_id": invitation.GuardianProfileID,
		"expires_at":          invitation.ExpiresAt,
		"email_sent":          invitation.EmailSentAt != nil,
	}

	common.Respond(w, r, http.StatusCreated, response, "Invitation sent successfully")
}

// listPendingInvitations handles listing all pending guardian invitations
func (rs *Resource) listPendingInvitations(w http.ResponseWriter, r *http.Request) {
	invitations, err := rs.GuardianService.GetPendingInvitations(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Convert to response format (without tokens)
	responses := make([]map[string]interface{}, 0, len(invitations))
	for _, inv := range invitations {
		responses = append(responses, map[string]interface{}{
			"id":                  inv.ID,
			"guardian_profile_id": inv.GuardianProfileID,
			"created_at":          inv.CreatedAt,
			"expires_at":          inv.ExpiresAt,
			"email_sent_at":       inv.EmailSentAt,
			"email_error":         inv.EmailError,
			"email_retry_count":   inv.EmailRetryCount,
		})
	}

	common.Respond(w, r, http.StatusOK, responses, "Pending invitations retrieved successfully")
}

// validateGuardianInvitation handles validating a guardian invitation token (PUBLIC)
func (rs *Resource) validateGuardianInvitation(w http.ResponseWriter, r *http.Request) {
	// Get token from URL
	token := chi.URLParam(r, "token")
	if token == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invitation token is required")))
		return
	}

	// Validate invitation
	result, err := rs.GuardianService.ValidateInvitation(r.Context(), token)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("invitation not found or expired")))
		return
	}

	common.Respond(w, r, http.StatusOK, result, "Invitation is valid")
}

// acceptGuardianInvitation handles accepting a guardian invitation and creating an account (PUBLIC)
func (rs *Resource) acceptGuardianInvitation(w http.ResponseWriter, r *http.Request) {
	// Get token from URL
	token := chi.URLParam(r, "token")
	if token == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invitation token is required")))
		return
	}

	// Parse request
	req := &GuardianInvitationAcceptRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Convert to service request
	acceptReq := guardianSvc.GuardianInvitationAcceptRequest{
		Token:           token,
		Password:        req.Password,
		ConfirmPassword: req.ConfirmPassword,
	}

	// Accept invitation
	account, err := rs.GuardianService.AcceptInvitation(r.Context(), acceptReq)
	if err != nil {
		// Log the full error for debugging
		if logger.Logger != nil {
			logger.Logger.WithError(err).Error("failed to accept guardian invitation")
		}

		// Return appropriate error
		if err.Error() == "invitation not found" || err.Error() == "invitation has expired" {
			common.RenderError(w, r, common.ErrorNotFound(err))
		} else {
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}

	// Return account details (without password hash)
	response := map[string]interface{}{
		"id":       account.ID,
		"email":    account.Email,
		"username": account.Username,
		"message":  "Account created successfully. You can now log in to the parent portal.",
	}

	common.Respond(w, r, http.StatusCreated, response, "Invitation accepted and account created successfully")
}
