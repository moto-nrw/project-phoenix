package operator

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
)

// UpdateProfileRequest represents the profile update request body
type UpdateProfileRequest struct {
	DisplayName string `json:"display_name"`
}

// Bind validates the update profile request
func (req *UpdateProfileRequest) Bind(_ *http.Request) error {
	return nil
}

// ChangePasswordRequest represents the password change request body
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// Bind validates the change password request
func (req *ChangePasswordRequest) Bind(_ *http.Request) error {
	return nil
}

// UpdateProfile handles updating the current operator's display name
func (rs *Resource) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	operatorID := int64(jwt.ClaimsFromCtx(r.Context()).ID)

	req := &UpdateProfileRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, rs.responses.InvalidRequest(err))
		return
	}
	if req.DisplayName == "" {
		common.RenderError(w, r, rs.responses.InvalidRequest(errors.New("display_name is required")))
		return
	}

	operator, err := rs.identity.UpdateOperatorProfile(r.Context(), operatorID, req.DisplayName)
	if err != nil {
		common.RenderError(w, r, rs.profileError(err))
		return
	}
	common.Respond(w, r, http.StatusOK, operatorResponse(&operator), "Profile updated successfully")
}

// ChangePassword handles changing the current operator's password; the
// capability revokes the operator's sessions with it.
func (rs *Resource) ChangePassword(w http.ResponseWriter, r *http.Request) {
	operatorID := int64(jwt.ClaimsFromCtx(r.Context()).ID)

	req := &ChangePasswordRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, rs.responses.InvalidRequest(err))
		return
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		common.RenderError(w, r, rs.responses.InvalidRequest(errors.New("current_password and new_password are required")))
		return
	}

	if err := rs.identity.ChangeOperatorPassword(r.Context(), operatorID, req.CurrentPassword, req.NewPassword); err != nil {
		common.RenderError(w, r, rs.profileError(err))
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Password changed successfully")
}
