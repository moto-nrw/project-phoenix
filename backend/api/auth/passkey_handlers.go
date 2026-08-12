package auth

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
)

const headerMotoFrontendOrigin = "X-Moto-Frontend-Origin"

var errPasskeyServiceUnavailable = errors.New("passkey service is not configured for this deployment")

type passkeyLoginOptionsRequest struct {
	TenantSlug string `json:"tenant_slug"`
}

func (req *passkeyLoginOptionsRequest) Bind(_ *http.Request) error {
	req.TenantSlug = strings.TrimSpace(req.TenantSlug)
	return validation.ValidateStruct(req, validation.Field(&req.TenantSlug, validation.Required))
}

// Passkey request bodies are shared with the operator portal via api/common;
// only the login-options request above is tenant-specific.
type (
	passkeyVerifyRequest          = common.PasskeyVerifyRequest
	passkeyRegisterOptionsRequest = common.PasskeyRegisterOptionsRequest
	passkeyRegisterVerifyRequest  = common.PasskeyRegisterVerifyRequest
)

func (rs *Resource) requirePasskey(w http.ResponseWriter, r *http.Request) bool {
	return common.RequireDependency(w, r, rs.PasskeyService != nil, errPasskeyServiceUnavailable)
}

func requireTenantPasskeyClaims(w http.ResponseWriter, r *http.Request) (jwt.AppClaims, bool) {
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID <= 0 || claims.TenantID <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrTenantAccessDenied))
		return jwt.AppClaims{}, false
	}
	switch claims.Scope {
	case tenant.ScopeTenant, "tenant", tenant.ScopeOrg:
		return claims, true
	default:
		common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrTenantAccessDenied))
		return jwt.AppClaims{}, false
	}
}

func (rs *Resource) passkeyLoginOptions(w http.ResponseWriter, r *http.Request) {
	if !rs.requirePasskey(w, r) {
		return
	}
	req := &passkeyLoginOptionsRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	origin := passkeyExpectedOrigin(r)
	if origin == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(authService.ErrPasskeyOriginInvalid))
		return
	}
	school, err := rs.SchoolService.GetSchoolBySubdomain(r.Context(), req.TenantSlug)
	if err != nil || school == nil || school.IsDeleted() || !school.Active {
		common.RenderError(w, r, common.ErrorNotFound(authService.ErrTenantNotFound))
		return
	}
	options, err := rs.PasskeyService.BeginLogin(r.Context(), authService.PasskeyLoginStartRequest{
		TenantID:        school.ID,
		TenantSubdomain: school.Subdomain,
		ExpectedOrigin:  origin,
	})
	if err != nil {
		mapPasskeyError(w, r, err)
		return
	}
	render.JSON(w, r, options)
}

func (rs *Resource) passkeyLoginVerify(w http.ResponseWriter, r *http.Request) {
	if !rs.requirePasskey(w, r) {
		return
	}
	req := &passkeyVerifyRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	result, err := rs.PasskeyService.FinishLogin(r.Context(), authService.PasskeyLoginFinishRequest{
		SessionID:          req.SessionID,
		CredentialResponse: req.Response,
		IPAddress:          getClientIP(r),
		UserAgent:          r.Header.Get(headerUserAgent),
	})
	if err != nil {
		mapPasskeyError(w, r, err)
		return
	}
	render.JSON(w, r, LoginResponse{
		Status:       string(authService.LoginStatusAuthenticated),
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
	})
}

func (rs *Resource) passkeyEnrollmentChallenge(w http.ResponseWriter, r *http.Request) {
	if !rs.requirePasskey(w, r) {
		return
	}
	claims, ok := requireTenantPasskeyClaims(w, r)
	if !ok {
		return
	}
	result, err := rs.PasskeyService.StartEnrollmentChallenge(r.Context(), int64(claims.ID), claims.TenantID, parseClientIP(r))
	if err != nil {
		mapPasskeyError(w, r, err)
		return
	}
	render.JSON(w, r, result)
}

func (rs *Resource) passkeyRegisterOptions(w http.ResponseWriter, r *http.Request) {
	if !rs.requirePasskey(w, r) {
		return
	}
	req := &passkeyRegisterOptionsRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	claims, ok := requireTenantPasskeyClaims(w, r)
	if !ok {
		return
	}
	origin := passkeyExpectedOrigin(r)
	if origin == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(authService.ErrPasskeyOriginInvalid))
		return
	}
	school, err := rs.SchoolService.GetSchoolByID(r.Context(), claims.TenantID)
	if err != nil || school == nil || school.IsDeleted() || !school.Active {
		common.RenderError(w, r, common.ErrorNotFound(authService.ErrTenantNotFound))
		return
	}
	options, err := rs.PasskeyService.BeginRegistration(r.Context(), authService.PasskeyRegistrationStartRequest{
		AccountID:       int64(claims.ID),
		TenantID:        claims.TenantID,
		TenantSubdomain: school.Subdomain,
		ExpectedOrigin:  origin,
		Code:            req.Code,
		Name:            req.Name,
	})
	if err != nil {
		mapPasskeyError(w, r, err)
		return
	}
	render.JSON(w, r, options)
}

func (rs *Resource) passkeyRegisterVerify(w http.ResponseWriter, r *http.Request) {
	if !rs.requirePasskey(w, r) {
		return
	}
	req := &passkeyRegisterVerifyRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	claims, ok := requireTenantPasskeyClaims(w, r)
	if !ok {
		return
	}
	credential, err := rs.PasskeyService.FinishRegistration(r.Context(), authService.PasskeyRegistrationFinishRequest{
		AccountID:          int64(claims.ID),
		SessionID:          req.SessionID,
		CredentialResponse: req.Response,
		Name:               req.Name,
	})
	if err != nil {
		mapPasskeyError(w, r, err)
		return
	}
	render.JSON(w, r, credential)
}

func (rs *Resource) passkeyList(w http.ResponseWriter, r *http.Request) {
	if !rs.requirePasskey(w, r) {
		return
	}
	claims, ok := requireTenantPasskeyClaims(w, r)
	if !ok {
		return
	}
	credentials, err := rs.PasskeyService.ListCredentials(r.Context(), int64(claims.ID))
	if err != nil {
		mapPasskeyError(w, r, err)
		return
	}
	render.JSON(w, r, credentials)
}

func (rs *Resource) passkeyRevoke(w http.ResponseWriter, r *http.Request) {
	if !rs.requirePasskey(w, r) {
		return
	}
	rawID := chi.URLParam(r, "passkeyId")
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid passkey id")))
		return
	}
	claims, ok := requireTenantPasskeyClaims(w, r)
	if !ok {
		return
	}
	if err := rs.PasskeyService.RevokeCredential(r.Context(), int64(claims.ID), id); err != nil {
		mapPasskeyError(w, r, err)
		return
	}
	common.RespondNoContent(w, r)
}

func passkeyExpectedOrigin(r *http.Request) string {
	if origin := strings.TrimSpace(r.Header.Get(headerMotoFrontendOrigin)); origin != "" {
		return origin
	}
	return strings.TrimSpace(r.Header.Get("Origin"))
}

func mapPasskeyError(w http.ResponseWriter, r *http.Request, err error) {
	var authErr *authService.AuthError
	if errors.As(err, &authErr) {
		err = authErr.Err
	}
	switch {
	case errors.Is(err, authService.ErrInvalidCredentials),
		errors.Is(err, authService.ErrPasskeySessionInvalid),
		errors.Is(err, authService.ErrAccountNotFound):
		common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrInvalidCredentials))
	case errors.Is(err, authService.ErrAccountInactive):
		common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrAccountInactive))
	case errors.Is(err, authService.ErrTenantAccessDenied):
		common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrTenantAccessDenied))
	case errors.Is(err, authService.ErrTenantNotFound):
		common.RenderError(w, r, common.ErrorNotFound(authService.ErrTenantNotFound))
	case errors.Is(err, authService.ErrPasskeyOriginInvalid):
		common.RenderError(w, r, common.ErrorUnauthorized(err))
	case errors.Is(err, authService.ErrMFACodeInvalid):
		common.RenderError(w, r, common.ErrorUnauthorized(err))
	case errors.Is(err, authService.ErrMFARateLimited),
		errors.Is(err, authService.ErrMFALocked):
		common.RenderError(w, r, common.ErrorTooManyRequests(err))
	case errors.Is(err, authService.ErrMFAStatusUnavailable):
		// Passkey enrollment starts an email challenge, so it inherits the
		// MFA service's fail-closed behaviour on a status or rate-limit
		// lookup error. That is a transient infrastructure problem: 503 so
		// the client retries instead of surfacing a permanent-looking 500.
		common.RenderError(w, r, common.ErrorServiceUnavailable(err))
	case errors.Is(err, authService.ErrParentMustUseParentPortal):
		// Same code as the password path in session_handlers.go — a client
		// must not have to care which login route produced the 403.
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "use_parent_portal"))
	case errors.Is(err, authService.ErrPasskeyNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
