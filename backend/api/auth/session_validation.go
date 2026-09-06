package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"

	"github.com/go-chi/render"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
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
	validator, ok := rs.AuthService.(authService.SessionTokenValidator)
	if !ok {
		common.RenderError(w, r, common.ErrorServiceUnavailable(errors.New("session validation unavailable")))
		return
	}
	claims, err := validator.ValidateSessionTokens(r.Context(), input.AccessToken, input.RefreshToken, input.Portal)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrInvalidToken))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	render.JSON(w, r, claims)
}
