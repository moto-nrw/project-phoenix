package operator

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
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

// ProfileErrorRenderer maps profile-related service errors to HTTP responses
func ProfileErrorRenderer(err error) render.Renderer {
	var passwordMismatch *platformSvc.PasswordMismatchError
	var operatorNotFound *platformSvc.OperatorNotFoundError
	var invalidData *platformSvc.InvalidDataError

	switch {
	case errors.As(err, &passwordMismatch):
		return ErrInvalidRequest(errors.New("das aktuelle Passwort ist falsch"))
	case errors.As(err, &operatorNotFound):
		return ErrNotFound("Operator not found")
	case errors.As(err, &invalidData):
		return ErrInvalidRequest(err)
	default:
		return ErrInternal("An error occurred")
	}
}
