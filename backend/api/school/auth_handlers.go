package school

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/clientip"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

const headerUserAgent = "User-Agent"

// trustedDeviceCookieName matches the tenant portal's cookie name — the
// trusted-device record itself is keyed by (account, tenant) in the service,
// so a device marked on one portal is legitimately trusted on the other for
// the same account + school.
const trustedDeviceCookieName = "mfa_trust_device"

var errMFAServiceUnavailable = errors.New("mfa service is not configured for this deployment")

// LoginRequest is the body shape for POST /school/auth/login. No tenant
// slug: the login resolves the school from the account's school-portal role
// mappings (first match), mirroring the parents login; multi-school
// accounts switch afterwards via /school/auth/switch-school.
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

// LoginResponse mirrors the tenant login response shape (status +
// tokens / challenge) so the frontend NextAuth provider can share the
// MFA-aware consumption logic.
type LoginResponse struct {
	Status                string `json:"status"`
	AccessToken           string `json:"access_token,omitempty"`
	RefreshToken          string `json:"refresh_token,omitempty"`
	ChallengeToken        string `json:"challenge_token,omitempty"`
	MaskedEmail           string `json:"masked_email,omitempty"`
	MFAEnrollmentRequired bool   `json:"mfa_enrollment_required,omitempty"`
	TrustedDeviceEnabled  *bool  `json:"trusted_device_enabled,omitempty"`
	TrustedDeviceDays     *int   `json:"trusted_device_days,omitempty"`
}

// TokenResponse is the plain token-pair shape — same as the tenant and
// parent login responses.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// SwitchSchoolRequest is the body shape for POST /school/auth/switch-school.
type SwitchSchoolRequest struct {
	TenantSlug string `json:"tenant_slug"`
}

// Bind validates the switch-school request.
func (req *SwitchSchoolRequest) Bind(_ *http.Request) error {
	req.TenantSlug = strings.TrimSpace(req.TenantSlug)
	return validation.ValidateStruct(req,
		validation.Field(&req.TenantSlug, validation.Required),
	)
}

// login authenticates a school-portal user via email/password and issues a
// school-scope JWT (or an MFA challenge). Refuses accounts without a
// school-portal role at any school.
func (rs *Resource) login(w http.ResponseWriter, r *http.Request) {
	req := &LoginRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	ipAddress := getClientIP(r)
	userAgent := r.Header.Get(headerUserAgent)

	var trustedDeviceCookie string
	if c, err := r.Cookie(trustedDeviceCookieName); err == nil {
		trustedDeviceCookie = c.Value
	}

	result, err := rs.AuthService.LoginSchoolWithMFAGate(
		r.Context(), req.Email, req.Password, ipAddress, userAgent, trustedDeviceCookie,
	)
	if err != nil {
		rs.handleLoginError(w, r, err)
		return
	}

	switch result.Status {
	case authService.LoginStatusMFARequired:
		tde := result.TrustedDeviceEnabled
		tdd := result.TrustedDeviceDays
		render.JSON(w, r, LoginResponse{
			Status:               string(authService.LoginStatusMFARequired),
			ChallengeToken:       result.ChallengeToken,
			MaskedEmail:          result.MaskedEmail,
			TrustedDeviceEnabled: &tde,
			TrustedDeviceDays:    &tdd,
		})
		return
	case authService.LoginStatusMFAEnrollmentRequired:
		render.JSON(w, r, LoginResponse{
			Status:                string(authService.LoginStatusMFAEnrollmentRequired),
			AccessToken:           result.AccessToken,
			MaskedEmail:           result.MaskedEmail,
			MFAEnrollmentRequired: true,
		})
		return
	}

	render.JSON(w, r, LoginResponse{
		Status:       string(authService.LoginStatusAuthenticated),
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
	})
}

// handleLoginError maps school-login service errors onto HTTP responses.
// Credential-shaped failures mirror the parent login (enumeration-safe);
// the portal-role refusal gets a stable code the frontend switches on.
func (rs *Resource) handleLoginError(w http.ResponseWriter, r *http.Request, err error) {
	var authErr *authService.AuthError
	if errors.As(err, &authErr) {
		switch {
		case errors.Is(authErr.Err, authService.ErrInvalidCredentials),
			errors.Is(authErr.Err, authService.ErrAccountNotFound):
			// Mask the specific cause to prevent account enumeration.
			common.RenderError(w, r, common.ErrorUnauthorizedWithCode(
				authService.ErrInvalidCredentials, "invalid_credentials"))
		case errors.Is(authErr.Err, authService.ErrAccountInactive):
			common.RenderError(w, r, common.ErrorUnauthorizedWithCode(
				authService.ErrAccountInactive, "account_inactive"))
		case errors.Is(authErr.Err, authService.ErrAccountNoSchoolPortalRole):
			// 403 with a stable code — reachable only after the password
			// was accepted, so it leaks nothing about foreign accounts.
			common.RenderError(w, r, common.ErrorForbiddenWithCode(
				authService.ErrAccountNoSchoolPortalRole, "no_school_portal_role"))
		case errors.Is(authErr.Err, authService.ErrMFARateLimited),
			errors.Is(authErr.Err, authService.ErrMFALocked):
			common.RenderError(w, r, common.ErrorTooManyRequests(authErr.Err))
		case errors.Is(authErr.Err, authService.ErrMFAStatusUnavailable):
			// Fail-closed MFA status lookup — 503 so the client retries
			// instead of treating it as bad credentials.
			common.RenderError(w, r, common.ErrorServiceUnavailable(authErr.Err))
		default:
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}
	common.RenderError(w, r, common.ErrorInternalServer(err))
}

// mfaVerify exchanges a SCHOOL-scope challenge token + email code for a
// school-scope token pair. Challenges started at the tenant login carry the
// tenant scope and are refused here (and vice versa) — the scope check
// happens before any code comparison, so a mismatched challenge is never
// consumed.
func (rs *Resource) mfaVerify(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	req := &common.MFAVerifyRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	verified, err := rs.MFAService.VerifyChallengeForScope(r.Context(), req.ChallengeToken, req.Code, jwt.MFAChallengeScopeSchool)
	if err != nil {
		mapMFAError(w, r, err)
		return
	}

	ipAddress := getClientIP(r)
	userAgent := r.Header.Get(headerUserAgent)

	accessToken, refreshToken, err := rs.AuthService.IssueSchoolTokensForAuthenticatedAccount(
		r.Context(), verified.AccountID, verified.TenantID, ipAddress, userAgent,
	)
	if err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrAccountNotFound):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrInvalidCredentials))
			case errors.Is(authErr.Err, authService.ErrAccountInactive):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrAccountInactive))
			case errors.Is(authErr.Err, authService.ErrAccountNoSchoolPortalRole):
				common.RenderError(w, r, common.ErrorForbiddenWithCode(
					authService.ErrAccountNoSchoolPortalRole, "no_school_portal_role"))
			default:
				common.RenderError(w, r, common.ErrorInternalServer(err))
			}
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	if req.RememberDevice {
		if err := rs.issueTrustedDeviceCookie(w, r, verified.AccountID, verified.TenantID); err != nil {
			// Don't fail the whole login — log and proceed.
			slog.Default().Warn("failed to issue trusted-device cookie",
				slog.Int64("account_id", verified.AccountID),
				slog.String("error", err.Error()),
			)
		}
	}

	render.JSON(w, r, TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

// mfaResend re-issues an email code against the existing challenge token.
// Rate-limited inside the service; returns the renewed challenge JWT.
func (rs *Resource) mfaResend(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	req := &common.MFAResendRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	renewed, err := rs.MFAService.ResendChallenge(r.Context(), req.ChallengeToken, parseClientIP(r))
	if err != nil {
		mapMFAError(w, r, err)
		return
	}
	render.JSON(w, r, common.MFAResendResponse{ChallengeToken: renewed})
}

// switchSchool handles POST /school/auth/switch-school. The account id comes
// from the school-scope claims; the target school must be an active mapping
// where the account holds a school-portal role.
func (rs *Resource) switchSchool(w http.ResponseWriter, r *http.Request) {
	req := &SwitchSchoolRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())

	accessToken, refreshToken, err := rs.AuthService.SwitchSchool(r.Context(), int64(claims.ID), req.TenantSlug)
	if err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrAccountNotFound):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrAccountNotFound))
			case errors.Is(authErr.Err, authService.ErrAccountInactive):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrAccountInactive))
			case errors.Is(authErr.Err, authService.ErrTenantNotFound):
				common.RenderError(w, r, common.ErrorNotFound(authService.ErrTenantNotFound))
			case errors.Is(authErr.Err, authService.ErrTenantAccessDenied):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrTenantAccessDenied))
			case errors.Is(authErr.Err, authService.ErrAccountNoSchoolPortalRole):
				common.RenderError(w, r, common.ErrorForbiddenWithCode(
					authService.ErrAccountNoSchoolPortalRole, "no_school_portal_role"))
			default:
				common.RenderError(w, r, common.ErrorInternalServer(err))
			}
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	render.JSON(w, r, TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

// requireMFA returns true when the MFA service is wired in and writes a 503
// response otherwise. Use as the first line of every MFA handler.
func (rs *Resource) requireMFA(w http.ResponseWriter, r *http.Request) bool {
	return common.RequireDependency(w, r, rs.MFAService != nil, errMFAServiceUnavailable)
}

// mapMFAError translates known MFA-service errors into HTTP responses —
// same mapping as the tenant portal's MFA surface.
func mapMFAError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, authService.ErrMFAChallengeTokenInvalid),
		errors.Is(err, authService.ErrMFACodeInvalid),
		errors.Is(err, authService.ErrMFAUnsupportedScope):
		common.RenderError(w, r, common.ErrorUnauthorized(err))
	case errors.Is(err, authService.ErrMFALocked),
		errors.Is(err, authService.ErrMFARateLimited):
		common.RenderError(w, r, common.ErrorTooManyRequests(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

// issueTrustedDeviceCookie mirrors the tenant portal's cookie issuance.
// When the tenant has disabled trusted devices, the service returns an
// empty cookie value and no Set-Cookie header is written.
func (rs *Resource) issueTrustedDeviceCookie(w http.ResponseWriter, r *http.Request, accountID, tenantID int64) error {
	cookieValue, expiresAt, err := rs.MFAService.IssueTrustedDevice(
		r.Context(), accountID, tenantID, r.Header.Get(headerUserAgent), parseClientIP(r),
	)
	if err != nil {
		return err
	}
	if cookieValue == "" {
		return nil
	}
	http.SetCookie(w, &http.Cookie{
		Name:     trustedDeviceCookieName,
		Value:    cookieValue,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// getClientIP returns the router-selected client IP for audit/rate-limit flows.
func getClientIP(r *http.Request) string {
	return clientip.GetClientIPString(r)
}

// parseClientIP wraps getClientIP and parses the result as net.IP.
func parseClientIP(r *http.Request) net.IP {
	return clientip.ParseClientIP(r)
}
