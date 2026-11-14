package account

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/inbound/portalauth"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"

	"github.com/go-chi/render"
)

func (rs *Resource) validateSession(w http.ResponseWriter, r *http.Request) {
	var input struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		Portal       string `json:"portal"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid session validation request")))
		return
	}
	if rs.Sessions == nil {
		common.RenderError(w, r, common.ErrorServiceUnavailable(errors.New("session validation unavailable")))
		return
	}
	claims, err := rs.Sessions.ValidateSessionTokens(r.Context(), input.AccessToken, input.RefreshToken, input.Portal)
	if err != nil {
		// A refusal answers as the refresh would. A read that could not run
		// is 503 (#5399): the BFF keeps the session it shows and writes no
		// cookie, but does not end it on a hiccup.
		common.RenderError(w, r, portalauth.RendererOr(err, validationUnavailable, portalauth.RenewalRules))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	render.JSON(w, r, sessionClaimsResponse(claims))
}

// sessionClaimsResponse renders the validated claims in the access-token
// claim shape the frontend hand-off consumes.
func sessionClaimsResponse(claims identityaccess.SessionClaims) *sessiontoken.AppClaims {
	return &sessiontoken.AppClaims{
		ID: int(claims.AccountID), Sub: claims.Email, Username: claims.Username, FirstName: claims.FirstName, LastName: claims.LastName,
		Roles: claims.Roles, Permissions: claims.Permissions, IsAdmin: claims.IsAdmin, Scope: claims.Scope,
		TenantID: claims.TenantID, OrgID: claims.OrgID, FamilyID: claims.FamilyID,
		ReadOnly: claims.ReadOnly, ActingAdminID: claims.ActingAdminID, PreviewID: claims.PreviewID, ViaTraeger: claims.ViaTraeger,
		CommonClaims: sessiontoken.CommonClaims{ExpiresAt: claims.ExpiresAt},
	}
}

func validationUnavailable(error) render.Renderer {
	return common.ErrorServiceUnavailable(errors.New("session validation unavailable"))
}
