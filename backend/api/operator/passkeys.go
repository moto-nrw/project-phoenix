package operator

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

const headerOperatorFrontendOrigin = "X-Moto-Frontend-Origin"

var errOperatorPasskeyServiceUnavailable = errors.New("operator passkey service is not configured")

// Passkey request bodies are shared with the tenant portal via api/common.
type (
	operatorPasskeyVerifyRequest          = common.PasskeyVerifyRequest
	operatorPasskeyRegisterOptionsRequest = common.PasskeyRegisterOptionsRequest
	operatorPasskeyRegisterVerifyRequest  = common.PasskeyRegisterVerifyRequest
)

func (rs *Resource) requirePasskey(w http.ResponseWriter, r *http.Request) bool {
	return common.RequireDependency(w, r, rs.passkeyService != nil, errOperatorPasskeyServiceUnavailable)
}

func (rs *Resource) PasskeyLoginOptions(w http.ResponseWriter, r *http.Request) {
	if !rs.requirePasskey(w, r) {
		return
	}
	origin := operatorPasskeyExpectedOrigin(r)
	if origin == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(authSvc.ErrPasskeyOriginInvalid))
		return
	}
	options, err := rs.passkeyService.BeginLogin(r.Context(), origin)
	if err != nil {
		mapOperatorPasskeyError(w, r, err)
		return
	}
	render.JSON(w, r, options)
}

func (rs *Resource) PasskeyLoginVerify(w http.ResponseWriter, r *http.Request) {
	if !rs.requirePasskey(w, r) {
		return
	}
	req := &operatorPasskeyVerifyRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	ip := getClientIP(r)
	ipString := ""
	if ip != nil {
		ipString = ip.String()
	}
	result, err := rs.passkeyService.FinishLogin(r.Context(), platformSvc.OperatorPasskeyLoginFinishRequest{
		SessionID:          req.SessionID,
		CredentialResponse: req.Response,
		IPAddress:          ipString,
		UserAgent:          r.Header.Get("User-Agent"),
	})
	if err != nil {
		mapOperatorPasskeyError(w, r, err)
		return
	}
	render.JSON(w, r, LoginResponse{
		Status:       string(platformSvc.OperatorLoginStatusAuthenticated),
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
	})
}

func (rs *Resource) PasskeyEnrollmentChallenge(w http.ResponseWriter, r *http.Request) {
	if !rs.requirePasskey(w, r) {
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	result, err := rs.passkeyService.StartEnrollmentChallenge(r.Context(), int64(claims.ID), getClientIP(r))
	if err != nil {
		mapOperatorPasskeyError(w, r, err)
		return
	}
	render.JSON(w, r, result)
}

func (rs *Resource) PasskeyRegisterOptions(w http.ResponseWriter, r *http.Request) {
	if !rs.requirePasskey(w, r) {
		return
	}
	req := &operatorPasskeyRegisterOptionsRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	origin := operatorPasskeyExpectedOrigin(r)
	if origin == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(authSvc.ErrPasskeyOriginInvalid))
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	options, err := rs.passkeyService.BeginRegistration(r.Context(), platformSvc.OperatorPasskeyRegistrationStartRequest{
		OperatorID:     int64(claims.ID),
		ExpectedOrigin: origin,
		Code:           req.Code,
		Name:           req.Name,
	})
	if err != nil {
		mapOperatorPasskeyError(w, r, err)
		return
	}
	render.JSON(w, r, options)
}

func (rs *Resource) PasskeyRegisterVerify(w http.ResponseWriter, r *http.Request) {
	if !rs.requirePasskey(w, r) {
		return
	}
	req := &operatorPasskeyRegisterVerifyRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	credential, err := rs.passkeyService.FinishRegistration(r.Context(), platformSvc.OperatorPasskeyRegistrationFinishRequest{
		OperatorID:         int64(claims.ID),
		SessionID:          req.SessionID,
		CredentialResponse: req.Response,
		Name:               req.Name,
	})
	if err != nil {
		mapOperatorPasskeyError(w, r, err)
		return
	}
	render.JSON(w, r, credential)
}

func (rs *Resource) PasskeyList(w http.ResponseWriter, r *http.Request) {
	if !rs.requirePasskey(w, r) {
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	credentials, err := rs.passkeyService.ListCredentials(r.Context(), int64(claims.ID))
	if err != nil {
		mapOperatorPasskeyError(w, r, err)
		return
	}
	render.JSON(w, r, credentials)
}

func (rs *Resource) PasskeyRevoke(w http.ResponseWriter, r *http.Request) {
	if !rs.requirePasskey(w, r) {
		return
	}
	rawID := chi.URLParam(r, "passkeyId")
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid passkey id")))
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	if err := rs.passkeyService.RevokeCredential(r.Context(), int64(claims.ID), id); err != nil {
		mapOperatorPasskeyError(w, r, err)
		return
	}
	common.RespondNoContent(w, r)
}

func operatorPasskeyExpectedOrigin(r *http.Request) string {
	if origin := strings.TrimSpace(r.Header.Get(headerOperatorFrontendOrigin)); origin != "" {
		return origin
	}
	return strings.TrimSpace(r.Header.Get("Origin"))
}

func mapOperatorPasskeyError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, authSvc.ErrInvalidCredentials),
		errors.Is(err, authSvc.ErrPasskeySessionInvalid):
		common.RenderError(w, r, ErrInvalidCredentials())
	case errors.Is(err, authSvc.ErrPasskeyOriginInvalid):
		common.RenderError(w, r, ErrForbidden("Passkey origin is not allowed"))
	case errors.Is(err, authSvc.ErrMFACodeInvalid):
		common.RenderError(w, r, ErrInvalidCredentials())
	case errors.Is(err, authSvc.ErrMFARateLimited),
		errors.Is(err, authSvc.ErrMFALocked):
		common.RenderError(w, r, ErrTooManyRequests("Too many code requests, please wait"))
	case errors.Is(err, authSvc.ErrPasskeyNotFound):
		common.RenderError(w, r, ErrNotFound("Passkey not found"))
	default:
		common.RenderError(w, r, AuthErrorRenderer(err))
	}
}
