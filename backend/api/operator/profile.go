package operator

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/render"
	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

// ProfileResource handles operator profile endpoints
type ProfileResource struct {
	authService platformSvc.OperatorAuthService
}

// NewProfileResource creates a new profile resource
func NewProfileResource(authService platformSvc.OperatorAuthService) *ProfileResource {
	return &ProfileResource{
		authService: authService,
	}
}

// UpdateProfileRequest represents the profile update request body
type UpdateProfileRequest struct {
	DisplayName string `json:"display_name"`
}

// Bind validates the update profile request
func (req *UpdateProfileRequest) Bind(r *http.Request) error {
	return nil
}

// ChangePasswordRequest represents the password change request body
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// Bind validates the change password request
func (req *ChangePasswordRequest) Bind(r *http.Request) error {
	return nil
}

// GetProfile handles retrieving the current operator's profile
func (rs *ProfileResource) GetProfile(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	operatorID := int64(claims.ID)

	operator, err := rs.authService.GetOperator(r.Context(), operatorID)
	if err != nil {
		common.RenderError(w, r, ProfileErrorRenderer(err))
		return
	}

	response := &OperatorResponse{
		ID:          operator.ID,
		Email:       operator.Email,
		DisplayName: operator.DisplayName,
	}

	common.Respond(w, r, http.StatusOK, response, "Profile retrieved successfully")
}

// UpdateProfile handles updating the current operator's profile
func (rs *ProfileResource) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	operatorID := int64(claims.ID)

	req := &UpdateProfileRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	if req.DisplayName == "" {
		common.RenderError(w, r, ErrInvalidRequest(errors.New("display_name is required")))
		return
	}

	operator, err := rs.authService.UpdateProfile(r.Context(), operatorID, req.DisplayName)
	if err != nil {
		common.RenderError(w, r, ProfileErrorRenderer(err))
		return
	}

	response := &OperatorResponse{
		ID:          operator.ID,
		Email:       operator.Email,
		DisplayName: operator.DisplayName,
	}

	common.Respond(w, r, http.StatusOK, response, "Profile updated successfully")
}

// ChangePassword handles changing the current operator's password
func (rs *ProfileResource) ChangePassword(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	operatorID := int64(claims.ID)

	req := &ChangePasswordRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		common.RenderError(w, r, ErrInvalidRequest(errors.New("current_password and new_password are required")))
		return
	}

	if err := rs.authService.ChangePassword(r.Context(), operatorID, req.CurrentPassword, req.NewPassword); err != nil {
		common.RenderError(w, r, ProfileErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Password changed successfully")
}

// InitiateEmailChangeRequest represents the email change initiation request body
type InitiateEmailChangeRequest struct {
	NewEmail        string `json:"new_email"`
	CurrentPassword string `json:"current_password"`
}

// Bind validates the initiate email change request
func (req *InitiateEmailChangeRequest) Bind(r *http.Request) error {
	req.NewEmail = strings.TrimSpace(req.NewEmail)
	if req.NewEmail == "" || req.CurrentPassword == "" {
		return errors.New("new_email and current_password are required")
	}
	return nil
}

// ConfirmEmailChangeRequest represents the email change confirmation request body
type ConfirmEmailChangeRequest struct {
	Token string `json:"token"`
}

// Bind validates the confirm email change request
func (req *ConfirmEmailChangeRequest) Bind(r *http.Request) error {
	req.Token = strings.TrimSpace(req.Token)
	if req.Token == "" {
		return errors.New("dieser Link ist abgelaufen oder ungültig")
	}
	if _, err := uuid.FromString(req.Token); err != nil {
		return errors.New("dieser Link ist abgelaufen oder ungültig")
	}
	return nil
}

// InitiateEmailChange handles starting the email change verification flow
func (rs *ProfileResource) InitiateEmailChange(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	operatorID := int64(claims.ID)

	req := &InitiateEmailChangeRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	clientIP := getClientIP(r)
	if err := rs.authService.InitiateEmailChange(r.Context(), operatorID, req.NewEmail, req.CurrentPassword, clientIP); err != nil {
		common.RenderError(w, r, ProfileErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Verification email will be sent shortly")
}

// ConfirmEmailChange handles confirming the email change via token
func (rs *ProfileResource) ConfirmEmailChange(w http.ResponseWriter, r *http.Request) {
	req := &ConfirmEmailChangeRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	clientIP := getClientIP(r)
	// Return value intentionally discarded — no PII on unauthenticated endpoint.
	_, err := rs.authService.ConfirmEmailChange(r.Context(), req.Token, clientIP)
	if err != nil {
		common.RenderError(w, r, confirmEmailChangeErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Email changed successfully")
}

// confirmEmailChangeErrorRenderer maps all ConfirmEmailChange errors to a
// generic 400 response. This endpoint is unauthenticated, so distinct status
// codes (403 inactive, 409 email-taken) would leak account/email state to
// anyone with a token or brute-forcing tokens.
func confirmEmailChangeErrorRenderer(err error) render.Renderer {
	return ErrInvalidRequest(errors.New("dieser Link ist abgelaufen oder ungültig. Bitte starte den Vorgang erneut"))
}

// ProfileErrorRenderer maps profile-related service errors to HTTP responses
func ProfileErrorRenderer(err error) render.Renderer {
	var passwordMismatch *platformSvc.PasswordMismatchError
	var operatorNotFound *platformSvc.OperatorNotFoundError
	var operatorInactive *platformSvc.OperatorInactiveError
	var invalidData *platformSvc.InvalidDataError
	var emailAlreadyInUse *platformSvc.EmailAlreadyInUseError
	var emailChangeRateLimit *platformSvc.EmailChangeRateLimitError
	var emailChangeSameEmail *platformSvc.EmailChangeSameEmailError
	var emailChangeTokenInvalid *platformSvc.EmailChangeTokenInvalidError

	switch {
	case errors.As(err, &passwordMismatch):
		return ErrInvalidRequest(errors.New("das aktuelle Passwort ist falsch"))
	case errors.As(err, &operatorNotFound):
		return ErrNotFound("Operator not found")
	case errors.As(err, &operatorInactive):
		return ErrForbidden("Dieser Account ist deaktiviert")
	case errors.As(err, &emailAlreadyInUse):
		// Defensive: InitiateEmailChange returns nil for duplicates (anti-enumeration),
		// and ConfirmEmailChange uses confirmEmailChangeErrorRenderer. No current caller
		// surfaces this error, but the mapping exists as a safety net if future profile
		// endpoints produce it.
		return ErrConflict("E-Mail-Adresse wird bereits verwendet")
	case errors.As(err, &emailChangeRateLimit):
		return ErrTooManyRequests("Zu viele Versuche. Bitte warte eine Stunde.")
	case errors.As(err, &emailChangeSameEmail):
		return ErrInvalidRequest(errors.New("die neue E-Mail ist identisch mit der aktuellen"))
	case errors.As(err, &emailChangeTokenInvalid):
		return ErrInvalidRequest(errors.New("dieser Link ist abgelaufen oder ungültig"))
	case errors.As(err, &invalidData):
		return ErrInvalidRequest(err)
	default:
		return ErrInternal("An error occurred")
	}
}
