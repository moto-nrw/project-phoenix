package auth

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/clientip"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// trustedDeviceCookieName is the browser cookie that carries the HMAC-signed
// trusted-device token. The path is "/" so the cookie is sent on every
// auth-relevant request; SameSite=Lax keeps it usable across the redirect
// chain that NextAuth uses on login while still blocking cross-site POSTs.
const trustedDeviceCookieName = "mfa_trust_device"

var errMFAServiceUnavailable = errors.New("mfa service is not configured for this deployment")

// requireMFA returns true when the MFA service is wired in and writes a 503
// response otherwise. Use as the first line of every MFA handler.
func (rs *Resource) requireMFA(w http.ResponseWriter, r *http.Request) bool {
	return common.RequireDependency(w, r, rs.MFAService != nil, errMFAServiceUnavailable)
}

// mapMFAError translates known MFA-service errors into HTTP responses.
// Anything unrecognised falls through to a 500.
func mapMFAError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, authService.ErrMFAChallengeTokenInvalid):
		common.RenderError(w, r, common.ErrorUnauthorized(err))
	case errors.Is(err, authService.ErrMFACodeInvalid):
		common.RenderError(w, r, common.ErrorUnauthorized(err))
	case errors.Is(err, authService.ErrMFALocked):
		common.RenderError(w, r, common.ErrorTooManyRequests(err))
	case errors.Is(err, authService.ErrMFARateLimited):
		common.RenderError(w, r, common.ErrorTooManyRequests(err))
	case errors.Is(err, authService.ErrMFANotEnrolled):
		common.RenderError(w, r, common.ErrorForbidden(err))
	case errors.Is(err, authService.ErrMFAAlreadyEnrolled):
		common.RenderError(w, r, common.ErrorConflict(err))
	case errors.Is(err, authService.ErrMFAPermissionDenied):
		common.RenderError(w, r, common.ErrorForbidden(err))
	case errors.Is(err, authService.ErrMFAInvalidOverride):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

// ----- /mfa/verify -----

// MFAVerifyRequest is the body for POST /auth/mfa/verify. Shared with the
// operator portal via api/common.
type MFAVerifyRequest = common.MFAVerifyRequest

// mfaVerify exchanges a challenge token + email code for a regular access /
// refresh token pair. When the request carries `remember_device: true` the
// service additionally issues a trusted-device cookie.
func (rs *Resource) mfaVerify(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	req := &MFAVerifyRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	verified, err := rs.MFAService.VerifyChallenge(r.Context(), req.ChallengeToken, req.Code)
	if err != nil {
		mapMFAError(w, r, err)
		return
	}

	rs.completeMFAExchange(w, r, verified.AccountID, verified.TenantID, req.RememberDevice)
}

// ----- /mfa/resend -----

// MFAResendRequest is the body for POST /auth/mfa/resend. Shared with the
// operator portal via api/common.
type MFAResendRequest = common.MFAResendRequest

// MFAResendResponse carries the renewed challenge token — see api/common.
type MFAResendResponse = common.MFAResendResponse

// mfaResend re-issues an email code against the existing challenge token.
// Rate-limited inside the service. Returns the renewed challenge JWT —
// see MFAResendResponse for why the previous 204-shape was unsafe.
func (rs *Resource) mfaResend(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	req := &MFAResendRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	renewed, err := rs.MFAService.ResendChallenge(r.Context(), req.ChallengeToken, parseClientIP(r))
	if err != nil {
		mapMFAError(w, r, err)
		return
	}
	render.JSON(w, r, MFAResendResponse{ChallengeToken: renewed})
}

// ----- /mfa/enroll/start -----

// mfaEnrollStart triggers an email with a code that the user must echo back
// at /mfa/enroll/confirm. No body — the enrollment-scoped JWT in the
// Authorization header identifies the account. Authenticated by
// MFAEnrollmentAuthenticator, which guarantees the token has
// mfa_enrollment_pending=true.
//
// The authenticator accepts both tenant- and platform-scoped enrollment
// tokens (operator routes share the same middleware). This handler must
// additionally enforce that the presented token is *tenant*-scoped — a
// platform-scoped token whose account_id happens to collide with a tenant
// account_id would otherwise be accepted and mint a full tenant session.
// (#1430 review item — tenant-enrollment scope mismatch.)
func (rs *Resource) mfaEnrollStart(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	claims, ok := jwt.EnrollmentClaimsFromCtx(r.Context())
	if !ok || claims.AccountID == 0 || claims.Scope != jwt.MFAEnrollmentScopeTenant || claims.TenantID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	_, err := rs.MFAService.StartChallenge(r.Context(), claims.AccountID, claims.TenantID, jwt.MFAChallengeScopeTenant, parseClientIP(r))
	if err != nil {
		mapMFAError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ----- /mfa/enroll/confirm -----

// MFAEnrollConfirmRequest is the body for POST /auth/mfa/enroll/confirm.
// Shared with the operator portal via api/common.
type MFAEnrollConfirmRequest = common.MFAEnrollConfirmRequest

// mfaEnrollConfirm verifies the just-emailed code, marks the account as
// enrolled, and mints a full access/refresh token pair so the frontend can
// seed a real session. Replaces the original 204 response that re-used the
// pre-MFA access token — that path allowed bypassing MFA by skipping
// enrollment entirely.
func (rs *Resource) mfaEnrollConfirm(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	claims, ok := jwt.EnrollmentClaimsFromCtx(r.Context())
	// Same defense-in-depth as mfaEnrollStart — tenant endpoint rejects
	// platform-scope enrollment tokens even when account_id matches.
	if !ok || claims.AccountID == 0 || claims.Scope != jwt.MFAEnrollmentScopeTenant || claims.TenantID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	req := &MFAEnrollConfirmRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	accountID := claims.AccountID

	// The enroll routes run under MFAEnrollmentAuthenticator, NOT
	// TenantTxMiddleware, so tenant.FromContext is 0 here. Inject the
	// tenant from the enrollment claims so the MFA audit events
	// (mfa_verified / mfa_failed) that VerifyCodeForAccount emits land
	// in audit.auth_events instead of being dropped as "no tenant
	// context". (#1430 review round 3, finding ①)
	ctx := tenant.WithTenantID(r.Context(), claims.TenantID)

	if err := rs.MFAService.VerifyCodeForAccount(ctx, accountID, req.Code); err != nil {
		mapMFAError(w, r, err)
		return
	}
	if err := rs.MFAService.Enroll(ctx, accountID); err != nil {
		// Already enrolled is fine — a retried request must still produce a
		// valid session. The pre-enrollment check at login means we should
		// rarely hit this branch in practice.
		if !errors.Is(err, authService.ErrMFAAlreadyEnrolled) {
			mapMFAError(w, r, err)
			return
		}
	}

	// Mint the real session now that the second factor is set up. Mirrors
	// the verify-flow's completeMFAExchange so the frontend handles the
	// response shape identically.
	rs.completeMFAExchange(w, r, accountID, claims.TenantID, req.RememberDevice)
}

// ----- /mfa/trusted-devices -----

// mfaListTrustedDevices returns the calling user's active trusted-device
// records so they can see and revoke them from the admin Sicherheit tab.
// Filtered to the authenticated account — ownership is enforced in the
// service layer too, but scoping the query here keeps the data path tight.
//
// The DTO + mapping helper live in api/common so the operator-side
// /operator/auth/mfa/trusted-devices endpoint can reuse them.
func (rs *Resource) mfaListTrustedDevices(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	// Trust is per-(account, tenant) — pass the tenant from claims so the
	// settings page never leaks devices trusted in another tenant. The
	// route runs inside TenantTxMiddleware, but threading the value
	// explicitly avoids relying on context state at the handler boundary.
	devices, err := rs.MFAService.ListTrustedDevices(r.Context(), int64(claims.ID), claims.TenantID)
	if err != nil {
		mapMFAError(w, r, err)
		return
	}
	render.JSON(w, r, common.MapTrustedDevices(devices, func(d *authModels.MFATrustedDevice) common.TrustedDeviceRow {
		return common.TrustedDeviceRow{
			ID:         d.ID,
			UserAgent:  d.UserAgent,
			IPAddress:  d.IPAddress,
			CreatedAt:  d.CreatedAt,
			ExpiresAt:  d.ExpiresAt,
			LastUsedAt: d.LastUsedAt,
		}
	}))
}

// mfaRevokeTrustedDevice deletes one trusted device the user has
// previously remembered. The service double-checks ownership so an
// attacker with an access token for account A can't revoke device rows
// belonging to account B via id-guessing.
func (rs *Resource) mfaRevokeTrustedDevice(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	idStr := chi.URLParam(r, "deviceId")
	deviceID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || deviceID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid device id")))
		return
	}
	// Tenant scope is enforced at the service layer too: a user with an
	// access token for (account A, tenant T1) can't revoke a device that
	// was trusted in tenant T2.
	if err := rs.MFAService.RevokeTrustedDevice(r.Context(), int64(claims.ID), claims.TenantID, deviceID); err != nil {
		mapMFAError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ----- shared helpers -----

// completeMFAExchange runs once a challenge has been verified: mints the
// access/refresh token pair, optionally issues a trusted-device cookie, and
// writes the response. Both the email-code and recovery-code paths funnel
// through here so cookie-handling stays in one place.
func (rs *Resource) completeMFAExchange(w http.ResponseWriter, r *http.Request, accountID, tenantID int64, rememberDevice bool) {
	ipAddress := getClientIP(r)
	userAgent := r.Header.Get(headerUserAgent)

	accessToken, refreshToken, err := rs.AuthService.IssueTokensForAuthenticatedAccount(
		r.Context(), accountID, tenantID, ipAddress, userAgent,
	)
	if err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrAccountNotFound):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrInvalidCredentials))
			case errors.Is(authErr.Err, authService.ErrAccountInactive):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrAccountInactive))
			default:
				common.RenderError(w, r, common.ErrorInternalServer(err))
			}
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	if rememberDevice {
		if err := rs.issueTrustedDeviceCookie(w, r, accountID, tenantID); err != nil {
			// Don't fail the whole login — log and proceed.
			slog.Default().Warn("failed to issue trusted-device cookie",
				slog.Int64("account_id", accountID),
				slog.String("error", err.Error()),
			)
		}
	}

	render.JSON(w, r, TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

// issueTrustedDeviceCookie hooks into MFAService.IssueTrustedDevice and
// writes the resulting Set-Cookie header. When the tenant has disabled
// trusted devices, the service returns an empty cookie value — we skip
// the Set-Cookie write entirely so the browser doesn't store a useless
// cookie.
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
		Name:    trustedDeviceCookieName,
		Value:   cookieValue,
		Path:    "/",
		Expires: expiresAt,
		MaxAge:  int(time.Until(expiresAt).Seconds()),
		// Always Secure — Chrome ≥89, Firefox and Safari accept Secure
		// cookies on `localhost` (and `*.localhost`) over HTTP as a
		// special-cased "secure context". Production serves HTTPS, so
		// the literal `true` is correct in every supported environment.
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// parseClientIP wraps getClientIP and parses the result as net.IP. Returns
// nil when the header value is empty or unparseable.
func parseClientIP(r *http.Request) net.IP {
	return clientip.ParseClientIP(r)
}
