package operator

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	identityoperator "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/operator"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
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
		common.RenderError(w, r, common.ErrorInvalidRequest(identityoperator.ErrPasskeyOriginInvalid))
		return
	}
	options, err := rs.passkeyService.BeginOperatorPasskeyLogin(r.Context(), origin)
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
	result, err := rs.passkeyService.FinishOperatorPasskeyLogin(r.Context(), identityoperator.OperatorPasskeyLoginFinish{
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
		Status:       identityoperator.LoginStatusAuthenticated,
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
	})
}

func (rs *Resource) PasskeyEnrollmentChallenge(w http.ResponseWriter, r *http.Request) {
	if !rs.requirePasskey(w, r) {
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	result, err := rs.passkeyService.StartOperatorPasskeyEnrollment(r.Context(), int64(claims.ID), getClientIP(r))
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
		common.RenderError(w, r, common.ErrorInvalidRequest(identityoperator.ErrPasskeyOriginInvalid))
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	options, err := rs.passkeyService.BeginOperatorPasskeyRegistration(r.Context(), identityoperator.OperatorPasskeyRegistrationStart{
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
	credential, err := rs.passkeyService.FinishOperatorPasskeyRegistration(r.Context(), identityoperator.OperatorPasskeyRegistrationFinish{
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
	credentials, err := rs.passkeyService.ListOperatorPasskeyCredentials(r.Context(), int64(claims.ID))
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
	if err := rs.passkeyService.RevokeOperatorPasskeyCredential(r.Context(), int64(claims.ID), id); err != nil {
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
	case errors.Is(err, identityoperator.ErrOperatorInvalidCredentials),
		errors.Is(err, identityoperator.ErrPasskeySessionInvalid):
		common.RenderError(w, r, ErrInvalidCredentials())
	case errors.Is(err, identityoperator.ErrPasskeyOriginInvalid):
		common.RenderError(w, r, ErrForbidden("Passkey origin is not allowed"))
	case errors.Is(err, identityoperator.ErrMFACodeInvalid):
		common.RenderError(w, r, ErrInvalidCredentials())
	case errors.Is(err, identityoperator.ErrMFARateLimited),
		errors.Is(err, identityoperator.ErrMFALocked):
		common.RenderError(w, r, ErrTooManyRequests("Too many code requests, please wait"))
	case errors.Is(err, identityoperator.ErrPasskeyNotFound):
		common.RenderError(w, r, ErrNotFound("Passkey not found"))
	default:
		// AuthErrorRenderer keeps the typed operator errors (invalid
		// credentials, inactive, unknown) on their own status codes and
		// answers everything else, including a failed read or write of the
		// passkey records, with a stable message.
		common.RenderError(w, r, AuthErrorRenderer(err))
	}
}
