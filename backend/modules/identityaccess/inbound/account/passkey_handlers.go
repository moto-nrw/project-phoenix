package account

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/inbound/portalauth"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/tokenprimitives"
)

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

func requireTenantPasskeyClaims(w http.ResponseWriter, r *http.Request) (sessiontoken.AppClaims, bool) {
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	if claims.ID <= 0 || claims.TenantID <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(identityaccess.ErrTenantAccessDenied))
		return sessiontoken.AppClaims{}, false
	}
	if portal, ok := tokenprimitives.PortalOfClaim(claims.Scope); !ok || portal.Name != tokenprimitives.PortalTenant {
		common.RenderError(w, r, common.ErrorUnauthorized(identityaccess.ErrTenantAccessDenied))
		return sessiontoken.AppClaims{}, false
	}
	return claims, true
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
	origin := common.PasskeyOrigin(r)
	if origin == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(identityaccess.ErrPasskeyOriginInvalid))
		return
	}
	school, err := rs.SchoolService.GetSchoolBySubdomain(r.Context(), req.TenantSlug)
	if err != nil || school == nil || school.Deleted || !school.Active {
		common.RenderError(w, r, common.ErrorNotFound(identityaccess.ErrTenantNotFound))
		return
	}
	options, err := rs.PasskeyService.BeginAccountPasskeyLogin(r.Context(), identityaccess.AccountPasskeyLoginStart{
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
	result, err := rs.PasskeyService.FinishAccountPasskeyLogin(r.Context(), identityaccess.AccountPasskeyLoginFinish{
		SessionID:          req.SessionID,
		CredentialResponse: req.Response,
		IPAddress:          common.GetClientIPString(r),
		UserAgent:          r.UserAgent(),
	})
	if err != nil {
		mapPasskeyLoginError(w, r, err)
		return
	}
	render.JSON(w, r, portalauth.LoginBody(identityaccess.LoginResult{
		Status: identityaccess.LoginStatusAuthenticated, AccessToken: result.AccessToken, RefreshToken: result.RefreshToken,
	}))
}

func (rs *Resource) passkeyEnrollmentChallenge(w http.ResponseWriter, r *http.Request) {
	if !rs.requirePasskey(w, r) {
		return
	}
	claims, ok := requireTenantPasskeyClaims(w, r)
	if !ok {
		return
	}
	result, err := rs.PasskeyService.StartAccountPasskeyEnrollment(r.Context(), int64(claims.ID), claims.TenantID, common.ParseClientIP(r))
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
	origin := common.PasskeyOrigin(r)
	if origin == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(identityaccess.ErrPasskeyOriginInvalid))
		return
	}
	school, err := rs.SchoolService.GetSchoolByID(r.Context(), claims.TenantID)
	if err != nil || school == nil || school.Deleted || !school.Active {
		common.RenderError(w, r, common.ErrorNotFound(identityaccess.ErrTenantNotFound))
		return
	}
	options, err := rs.PasskeyService.BeginAccountPasskeyRegistration(r.Context(), identityaccess.AccountPasskeyRegistrationStart{
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
	credential, err := rs.PasskeyService.FinishAccountPasskeyRegistration(r.Context(), identityaccess.AccountPasskeyRegistrationFinish{
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
	credentials, err := rs.PasskeyService.ListAccountPasskeyCredentials(r.Context(), int64(claims.ID))
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
	if err := rs.PasskeyService.RevokeAccountPasskeyCredential(r.Context(), int64(claims.ID), id); err != nil {
		mapPasskeyError(w, r, err)
		return
	}
	common.RespondNoContent(w, r)
}

// mapPasskeyLoginError answers every refused passkey sign-in alike; the
// other outcomes answer like the ceremonies.
func mapPasskeyLoginError(w http.ResponseWriter, r *http.Request, err error) {
	common.RenderError(w, r, portalauth.RendererOr(err, portalauth.PasskeyFailed, portalauth.PasskeyLoginRules, portalauth.PasskeyRules))
}

func mapPasskeyError(w http.ResponseWriter, r *http.Request, err error) {
	common.RenderError(w, r, portalauth.RendererOr(err, portalauth.PasskeyFailed, portalauth.PasskeyRules))
}
