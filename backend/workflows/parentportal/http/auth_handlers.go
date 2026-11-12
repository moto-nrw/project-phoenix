package parent

import (
	"net/http"
	"strings"

	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/inbound/portalauth"
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
	// The sign-in proved the address: its holiday registrations join the
	// account (ADR 0216). The workflow logs a failure; the sign-in stands.
	if rs.ParentService != nil {
		rs.ParentService.ClaimHolidayRegistrationsAfterSignIn(r.Context(), req.Email)
	}
	common.Respond(w, r, http.StatusOK, portalauth.TokenResponse{AccessToken: accessToken, RefreshToken: refreshToken}, "Login successful")
}
