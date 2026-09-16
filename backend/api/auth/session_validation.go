package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"

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
		common.RenderError(w, r, common.ErrorUnauthorized(identityaccess.ErrInvalidToken))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	render.JSON(w, r, sessionClaimsResponse(claims))
}

// sessionClaimsResponse renders the validated claims in the access-token
// claim shape the frontend hand-off consumes.
func sessionClaimsResponse(claims identityaccess.SessionClaims) *jwt.AppClaims {
	return &jwt.AppClaims{
		ID: int(claims.AccountID), Sub: claims.Email, Username: claims.Username, FirstName: claims.FirstName, LastName: claims.LastName,
		Roles: claims.Roles, Permissions: claims.Permissions, IsAdmin: claims.IsAdmin, Scope: claims.Scope,
		TenantID: claims.TenantID, OrgID: claims.OrgID, FamilyID: claims.FamilyID,
		ReadOnly: claims.ReadOnly, ActingAdminID: claims.ActingAdminID, PreviewID: claims.PreviewID,
		CommonClaims: jwt.CommonClaims{ExpiresAt: claims.ExpiresAt, IssuedAt: claims.IssuedAt},
	}
}
