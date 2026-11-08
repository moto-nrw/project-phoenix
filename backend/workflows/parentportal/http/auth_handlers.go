package parent

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/portalauth"
)

// LoginRequest is the body shape for POST /parent/auth/login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Bind normalizes + validates the request.
func (req *LoginRequest) Bind(_ *http.Request) error {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	return validation.ValidateStruct(req,
		validation.Field(&req.Email, validation.Required, is.Email),
		validation.Field(&req.Password, validation.Required),
	)
}

// PasswordResetRequest is the body shape for POST /parent/auth/password-reset.
type PasswordResetRequest struct {
	Email string `json:"email"`
}

// Bind normalizes + validates the password reset request.
func (req *PasswordResetRequest) Bind(_ *http.Request) error {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	return validation.ValidateStruct(req,
		validation.Field(&req.Email, validation.Required, is.Email),
	)
}

// PasswordResetConfirmRequest is the body shape for POST /parent/auth/password-reset/confirm.
type PasswordResetConfirmRequest struct {
	Token           string `json:"token"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

// Bind validates the password reset confirmation request.
func (req *PasswordResetConfirmRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.Token, validation.Required),
		validation.Field(&req.NewPassword, validation.Required, validation.Length(8, 0)),
		validation.Field(&req.ConfirmPassword, validation.Required, validation.By(func(_ interface{}) error {
			if req.NewPassword != req.ConfirmPassword {
				return errors.New("passwords do not match")
			}
			return nil
		})),
	)
}

// login signs a guardian in to a parent-scope session. Accounts without
// guardian access at any school are refused.
func (rs *Resource) login(w http.ResponseWriter, r *http.Request) {
	req := &LoginRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if !common.RequireDependency(w, r, rs.Auth.Login != nil, errLoginUnavailable) {
		return
	}
	client := portalauth.ClientOf(r)
	accessToken, refreshToken, err := rs.Auth.Login(r.Context(), req.Email, req.Password, client.IP, client.UserAgent)
	if err != nil {
		portalauth.Render(w, r, err, portalauth.ParentLoginRules, portalauth.LoginRules)
		return
	}
	common.Respond(w, r, http.StatusOK, portalauth.TokenResponse{AccessToken: accessToken, RefreshToken: refreshToken}, "Login successful")
}

// initiatePasswordReset sends a parent-portal password reset link when the
// email belongs to an account with guardian access. It always returns the same
// success body for unknown or non-guardian emails to avoid account enumeration.
func (rs *Resource) initiatePasswordReset(w http.ResponseWriter, r *http.Request) {
	req := &PasswordResetRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	if !rs.Resets.complete() {
		common.RenderError(w, r, common.ErrorInternalServer(ErrPasswordResetUnavailable))
		return
	}

	if err := rs.Resets.Initiate(r.Context(), req.Email); err != nil {
		if rs.renderPasswordResetRateLimit(w, r, err) {
			return
		}

		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "If the email exists, a password reset link has been sent")
}

// resetPassword confirms a parent password reset token. Tokens are shared with
// the staff reset flow; this route exists so parent pages never need to call
// the tenant auth API surface.
func (rs *Resource) resetPassword(w http.ResponseWriter, r *http.Request) {
	req := &PasswordResetConfirmRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	if !rs.Resets.complete() {
		common.RenderError(w, r, common.ErrorInternalServer(ErrPasswordResetUnavailable))
		return
	}

	if err := rs.Resets.Reset(r.Context(), req.Token, req.NewPassword); err != nil {
		switch {
		case rs.Resets.LinkUnusable(err):
			// 410 Gone: the token is invalid, already used, or expired.
			// The parents reset page maps this status to the "request a
			// new link" copy. Distinct from the 400 weak-password case
			// below, which tells the user to fix the password itself —
			// collapsing both into 400 sent the wrong remedy.
			common.RenderError(w, r, common.ErrorGoneWithCode(errors.New("invalid or expired reset token"), common.CodeIdentityPasswordResetLinkInvalid))
			return
		case rs.Resets.TooWeak(err):
			common.RenderError(w, r, common.ErrorInvalidOnField(ErrPasswordTooWeak, common.CodeIdentityPasswordTooWeak, "new_password"))
			return
		}

		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Password reset successfully")
}
