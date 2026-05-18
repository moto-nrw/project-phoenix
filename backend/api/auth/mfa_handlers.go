package auth

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/spf13/viper"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// secureCookie returns true only outside non-production environments so
// the trusted-device cookie is accepted by browsers over HTTP in local
// dev. Production deployments serve HTTPS exclusively, so the cookie
// stays Secure where it matters.
func secureCookie() bool {
	return strings.ToLower(strings.TrimSpace(viper.GetString("app_env"))) == "production"
}

// trustedDeviceCookieName is the browser cookie that carries the HMAC-signed
// trusted-device token. The path is "/" so the cookie is sent on every
// auth-relevant request; SameSite=Lax keeps it usable across the redirect
// chain that NextAuth uses on login while still blocking cross-site POSTs.
const trustedDeviceCookieName = "mfa_trust_device"

var errMFAServiceUnavailable = errors.New("mfa service is not configured for this deployment")

// requireMFA returns true when the MFA service is wired in and writes a 503
// response otherwise. Use as the first line of every MFA handler.
func (rs *Resource) requireMFA(w http.ResponseWriter, r *http.Request) bool {
	if rs.MFAService == nil {
		common.RenderError(w, r, &common.ErrResponse{
			Err:            errMFAServiceUnavailable,
			HTTPStatusCode: http.StatusServiceUnavailable,
			Status:         "error",
			ErrorText:      errMFAServiceUnavailable.Error(),
		})
		return false
	}
	return true
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

// MFAVerifyRequest is the body for POST /auth/mfa/verify.
type MFAVerifyRequest struct {
	ChallengeToken string `json:"challenge_token"`
	Code           string `json:"code"`
	RememberDevice bool   `json:"remember_device,omitempty"`
}

// Bind validates the verify request fields.
func (req *MFAVerifyRequest) Bind(_ *http.Request) error {
	req.ChallengeToken = strings.TrimSpace(req.ChallengeToken)
	req.Code = strings.TrimSpace(req.Code)
	return validation.ValidateStruct(req,
		validation.Field(&req.ChallengeToken, validation.Required),
		validation.Field(&req.Code, validation.Required, validation.Length(authService.MFAEmailCodeLength, authService.MFAEmailCodeLength)),
	)
}

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

// MFAResendRequest is the body for POST /auth/mfa/resend.
type MFAResendRequest struct {
	ChallengeToken string `json:"challenge_token"`
}

// Bind validates the resend request.
func (req *MFAResendRequest) Bind(_ *http.Request) error {
	req.ChallengeToken = strings.TrimSpace(req.ChallengeToken)
	return validation.ValidateStruct(req,
		validation.Field(&req.ChallengeToken, validation.Required),
	)
}

// mfaResend re-issues an email code against the existing challenge token.
// Rate-limited inside the service.
func (rs *Resource) mfaResend(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	req := &MFAResendRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err := rs.MFAService.ResendChallenge(r.Context(), req.ChallengeToken, parseClientIP(r)); err != nil {
		mapMFAError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ----- /mfa/enroll/start -----

// mfaEnrollStart triggers an email with a code that the user must echo back
// at /mfa/enroll/confirm. No body — the authenticated session identifies
// the account.
func (rs *Resource) mfaEnrollStart(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	_, err := rs.MFAService.StartChallenge(r.Context(), int64(claims.ID), claims.TenantID, jwt.MFAChallengeScopeTenant, parseClientIP(r))
	if err != nil {
		mapMFAError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ----- /mfa/enroll/confirm -----

// MFAEnrollConfirmRequest is the body for POST /auth/mfa/enroll/confirm.
type MFAEnrollConfirmRequest struct {
	Code string `json:"code"`
}

// Bind validates the confirm request.
func (req *MFAEnrollConfirmRequest) Bind(_ *http.Request) error {
	req.Code = strings.TrimSpace(req.Code)
	return validation.ValidateStruct(req,
		validation.Field(&req.Code, validation.Required, validation.Length(authService.MFAEmailCodeLength, authService.MFAEmailCodeLength)),
	)
}

// mfaEnrollConfirm verifies the just-emailed code and marks the account as
// enrolled. No body — the frontend only cares about the 204 to advance to
// the "2FA aktiviert" success screen.
func (rs *Resource) mfaEnrollConfirm(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	req := &MFAEnrollConfirmRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	accountID := int64(claims.ID)

	if err := rs.MFAService.VerifyCodeForAccount(r.Context(), accountID, req.Code); err != nil {
		mapMFAError(w, r, err)
		return
	}
	if err := rs.MFAService.Enroll(r.Context(), accountID); err != nil {
		// Already enrolled is fine here — treat the second enroll call as a
		// no-op so a retried request still ends with a 204.
		if !errors.Is(err, authService.ErrMFAAlreadyEnrolled) {
			mapMFAError(w, r, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// ----- /mfa/trusted-devices -----

// TrustedDeviceDTO is what we serialize back to the client. The token hash
// stays server-side, and we surface only the columns the user can act on:
// id (for revoke), user-agent + IP for "is this still you", and the four
// timestamps that drive the list UI ("created on ..., last used ...").
type TrustedDeviceDTO struct {
	ID         int64   `json:"id"`
	UserAgent  *string `json:"user_agent,omitempty"`
	IPAddress  string  `json:"ip_address,omitempty"`
	CreatedAt  string  `json:"created_at"`
	ExpiresAt  string  `json:"expires_at"`
	LastUsedAt *string `json:"last_used_at,omitempty"`
}

// mfaListTrustedDevices returns the calling user's active trusted-device
// records so they can see and revoke them from the admin Sicherheit tab.
// Filtered to the authenticated account — ownership is enforced in the
// service layer too, but scoping the query here keeps the data path tight.
func (rs *Resource) mfaListTrustedDevices(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	devices, err := rs.MFAService.ListTrustedDevices(r.Context(), int64(claims.ID))
	if err != nil {
		mapMFAError(w, r, err)
		return
	}
	out := make([]TrustedDeviceDTO, 0, len(devices))
	for _, d := range devices {
		dto := TrustedDeviceDTO{
			ID:        d.ID,
			UserAgent: d.UserAgent,
			CreatedAt: d.CreatedAt.UTC().Format(time.RFC3339),
			ExpiresAt: d.ExpiresAt.UTC().Format(time.RFC3339),
		}
		if d.IPAddress != nil {
			dto.IPAddress = d.IPAddress.String()
		}
		if d.LastUsedAt != nil {
			s := d.LastUsedAt.UTC().Format(time.RFC3339)
			dto.LastUsedAt = &s
		}
		out = append(out, dto)
	}
	render.JSON(w, r, out)
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
	if err := rs.MFAService.RevokeTrustedDevice(r.Context(), int64(claims.ID), deviceID); err != nil {
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
		// secureCookie() returns true in production. Sonar's static analyzer
		// cannot see through the helper. Mirrors the project-wide convention
		// used by the NextAuth session/CSRF cookies in
		// frontend/src/server/auth/{tenant,operator}-config.ts. Local HTTP
		// dev needs the cookie to be non-Secure or browsers drop it.
		Secure:   secureCookie(), //NOSONAR(go:S2092)
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// parseClientIP wraps getClientIP and parses the result as net.IP. Returns
// nil when the header value is empty or unparseable.
func parseClientIP(r *http.Request) net.IP {
	raw := getClientIP(r)
	if raw == "" {
		return nil
	}
	return net.ParseIP(raw)
}
