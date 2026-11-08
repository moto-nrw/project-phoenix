package operator

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/portalauth"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

var errOperatorPasskeyServiceUnavailable = errors.New("operator passkey service is not configured")

// Passkey request bodies are shared with the tenant portal via api/common.
type (
	operatorPasskeyVerifyRequest          = common.PasskeyVerifyRequest
	operatorPasskeyRegisterOptionsRequest = common.PasskeyRegisterOptionsRequest
	operatorPasskeyRegisterVerifyRequest  = common.PasskeyRegisterVerifyRequest
)

// PasskeyResource serves the operator passkey ceremonies and the
// management of the current operator's passkeys.
type PasskeyResource struct {
	passkeyService identityaccess.OperatorPasskeyFlows
}

// NewPasskeyResource binds the handlers to the passkey capability. A nil
// capability answers every route with 503.
func NewPasskeyResource(passkeys identityaccess.OperatorPasskeyFlows) *PasskeyResource {
	return &PasskeyResource{passkeyService: passkeys}
}

func (rs *PasskeyResource) requirePasskey(w http.ResponseWriter, r *http.Request) bool {
	return common.RequireDependency(w, r, rs.passkeyService != nil, errOperatorPasskeyServiceUnavailable)
}

func (rs *PasskeyResource) PasskeyLoginOptions(w http.ResponseWriter, r *http.Request) {
	if !rs.requirePasskey(w, r) {
		return
	}
	origin := common.PasskeyOrigin(r)
	if origin == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(identityaccess.ErrPasskeyOriginInvalid))
		return
	}
	options, err := rs.passkeyService.BeginOperatorPasskeyLogin(r.Context(), origin)
	if err != nil {
		mapOperatorPasskeyError(w, r, err)
		return
	}
	render.JSON(w, r, options)
}

func (rs *PasskeyResource) PasskeyLoginVerify(w http.ResponseWriter, r *http.Request) {
	if !rs.requirePasskey(w, r) {
		return
	}
	req := &operatorPasskeyVerifyRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	ip := common.ParseClientIP(r)
	ipString := ""
	if ip != nil {
		ipString = ip.String()
	}
	result, err := rs.passkeyService.FinishOperatorPasskeyLogin(r.Context(), identityaccess.OperatorPasskeyLoginFinish{
		SessionID:          req.SessionID,
		CredentialResponse: req.Response,
		IPAddress:          ipString,
		UserAgent:          r.UserAgent(),
	})
	if err != nil {
		mapOperatorPasskeyError(w, r, err)
		return
	}
	render.JSON(w, r, portalauth.LoginBody(identityaccess.LoginResult{
		Status: identityaccess.LoginStatusAuthenticated, AccessToken: result.AccessToken, RefreshToken: result.RefreshToken,
	}))
}

func (rs *PasskeyResource) PasskeyEnrollmentChallenge(w http.ResponseWriter, r *http.Request) {
	if !rs.requirePasskey(w, r) {
		return
	}
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	result, err := rs.passkeyService.StartOperatorPasskeyEnrollment(r.Context(), int64(claims.ID), common.ParseClientIP(r))
	if err != nil {
		mapOperatorPasskeyError(w, r, err)
		return
	}
	render.JSON(w, r, result)
}

func (rs *PasskeyResource) PasskeyRegisterOptions(w http.ResponseWriter, r *http.Request) {
	if !rs.requirePasskey(w, r) {
		return
	}
	req := &operatorPasskeyRegisterOptionsRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	origin := common.PasskeyOrigin(r)
	if origin == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(identityaccess.ErrPasskeyOriginInvalid))
		return
	}
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	options, err := rs.passkeyService.BeginOperatorPasskeyRegistration(r.Context(), identityaccess.OperatorPasskeyRegistrationStart{
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

func (rs *PasskeyResource) PasskeyRegisterVerify(w http.ResponseWriter, r *http.Request) {
	if !rs.requirePasskey(w, r) {
		return
	}
	req := &operatorPasskeyRegisterVerifyRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	credential, err := rs.passkeyService.FinishOperatorPasskeyRegistration(r.Context(), identityaccess.OperatorPasskeyRegistrationFinish{
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

func (rs *PasskeyResource) PasskeyList(w http.ResponseWriter, r *http.Request) {
	if !rs.requirePasskey(w, r) {
		return
	}
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	credentials, err := rs.passkeyService.ListOperatorPasskeyCredentials(r.Context(), int64(claims.ID))
	if err != nil {
		mapOperatorPasskeyError(w, r, err)
		return
	}
	render.JSON(w, r, credentials)
}

func (rs *PasskeyResource) PasskeyRevoke(w http.ResponseWriter, r *http.Request) {
	if !rs.requirePasskey(w, r) {
		return
	}
	rawID := chi.URLParam(r, "passkeyId")
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid passkey id")))
		return
	}
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	if err := rs.passkeyService.RevokeOperatorPasskeyCredential(r.Context(), int64(claims.ID), id); err != nil {
		mapOperatorPasskeyError(w, r, err)
		return
	}
	common.RespondNoContent(w, r)
}

func mapOperatorPasskeyError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, identityaccess.ErrOperatorInvalidCredentials),
		errors.Is(err, identityaccess.ErrPasskeySessionInvalid):
		common.RenderError(w, r, common.OperatorRejection(http.StatusUnauthorized, common.CodeIdentityPasskeyLoginFailed, "Invalid email or password"))
	case errors.Is(err, identityaccess.ErrPasskeyOriginInvalid):
		common.RenderError(w, r, common.OperatorRejection(http.StatusForbidden, common.CodeIdentityPasskeyLoginFailed, "Passkey origin is not allowed"))
	case errors.Is(err, identityaccess.ErrMFACodeInvalid):
		common.RenderError(w, r, common.OperatorRejection(http.StatusUnauthorized, common.CodeIdentityMfaCodeInvalid, "Invalid email or password"))
	case errors.Is(err, identityaccess.ErrMFARateLimited),
		errors.Is(err, identityaccess.ErrMFALocked):
		common.RenderError(w, r, common.OperatorRejection(http.StatusTooManyRequests, common.CodeIdentityMfaBlocked, "Too many code requests, please wait"))
	case errors.Is(err, identityaccess.ErrPasskeyNotFound):
		common.RenderError(w, r, common.OperatorRejection(http.StatusNotFound, common.CodeIdentityPasskeyNotFound, "Passkey not found"))
	default:
		// A failed read or write of the passkey records answers with the
		// sign-in's stable message.
		common.RenderError(w, r, AuthErrorRenderer(err))
	}
}
