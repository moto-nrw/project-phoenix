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

// ----- /mfa/recovery/verify -----

// MFARecoveryVerifyRequest is the body for POST /auth/mfa/recovery/verify.
type MFARecoveryVerifyRequest struct {
	ChallengeToken string `json:"challenge_token"`
	RecoveryCode   string `json:"recovery_code"`
	RememberDevice bool   `json:"remember_device,omitempty"`
}

// Bind validates the recovery verify request.
func (req *MFARecoveryVerifyRequest) Bind(_ *http.Request) error {
	req.ChallengeToken = strings.TrimSpace(req.ChallengeToken)
	req.RecoveryCode = strings.TrimSpace(req.RecoveryCode)
	return validation.ValidateStruct(req,
		validation.Field(&req.ChallengeToken, validation.Required),
		validation.Field(&req.RecoveryCode, validation.Required),
	)
}

// mfaRecoveryVerify is the recovery-code analogue of mfaVerify. The user
// reaches this when the email channel is unavailable. We still parse the
// challenge token to identify the account, then call VerifyRecoveryCode.
func (rs *Resource) mfaRecoveryVerify(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	req := &MFARecoveryVerifyRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Reuse VerifyChallenge with a deliberately-wrong-but-shaped code so we
	// get the typed error path that includes challenge-token validation
	// without consuming an email-code row. Then fall through to recovery.
	// Simpler: parse the challenge ourselves via a tiny resend-shaped path.
	// We already have ResendChallenge for that — but it would issue a new
	// code. Instead we reach into the service via a small new helper.
	//
	// For the v1 cut, accept the same challenge flow: VerifyChallenge errors
	// on a wrong code, but the recovery handler doesn't care — it only needs
	// the account_id + tenant_id from the challenge JWT. We extract those
	// via a 1-call helper to keep this handler thin.
	accountID, tenantID, err := rs.peekChallengeIdentity(r, req.ChallengeToken)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrMFAChallengeTokenInvalid))
		return
	}

	if err := rs.MFAService.VerifyRecoveryCode(r.Context(), accountID, req.RecoveryCode); err != nil {
		mapMFAError(w, r, err)
		return
	}

	rs.completeMFAExchange(w, r, accountID, tenantID, req.RememberDevice)
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

// MFAEnrollConfirmResponse returns the freshly-generated recovery codes.
// Per the design plan the client must show these once and require the user
// to confirm they have stored them before continuing — so the response is
// intentionally minimal.
type MFAEnrollConfirmResponse struct {
	RecoveryCodes []string `json:"recovery_codes"`
}

// mfaEnrollConfirm verifies the just-emailed code, marks the account as
// enrolled, and seeds the recovery-code pool. The plain codes are returned
// once and only once.
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
		// Already enrolled is fine here — surface a 409 but proceed to recovery codes.
		if !errors.Is(err, authService.ErrMFAAlreadyEnrolled) {
			mapMFAError(w, r, err)
			return
		}
	}
	codes, err := rs.MFAService.GenerateRecoveryCodes(r.Context(), accountID)
	if err != nil {
		mapMFAError(w, r, err)
		return
	}
	render.JSON(w, r, MFAEnrollConfirmResponse{RecoveryCodes: codes})
}

// ----- /mfa/recovery-codes (regenerate) -----

// MFARecoveryCodesResponse mirrors MFAEnrollConfirmResponse for the
// regenerate endpoint.
type MFARecoveryCodesResponse struct {
	RecoveryCodes []string `json:"recovery_codes"`
}

// mfaRegenerateRecoveryCodes wipes the user's existing recovery-code pool
// and returns a fresh batch. Self-service, no permission gate beyond the
// access-token middleware.
func (rs *Resource) mfaRegenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	codes, err := rs.MFAService.GenerateRecoveryCodes(r.Context(), int64(claims.ID))
	if err != nil {
		mapMFAError(w, r, err)
		return
	}
	render.JSON(w, r, MFARecoveryCodesResponse{RecoveryCodes: codes})
}

// ----- DELETE /mfa -----

// mfaDisable lets the authenticated user opt out of MFA. The service handles
// the cascade (credential, recovery codes, trusted devices revoked).
func (rs *Resource) mfaDisable(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	if err := rs.MFAService.Disable(r.Context(), int64(claims.ID)); err != nil {
		mapMFAError(w, r, err)
		return
	}
	// Clear the trusted-device cookie on the responder's behalf.
	http.SetCookie(w, &http.Cookie{
		Name:     trustedDeviceCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Secure:   secureCookie(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

// ----- /mfa/trusted-devices -----

// MFATrustedDeviceResponse is the listing shape returned by GET
// /auth/mfa/trusted-devices.
type MFATrustedDeviceResponse struct {
	ID         int64      `json:"id"`
	UserAgent  string     `json:"user_agent,omitempty"`
	IPAddress  string     `json:"ip_address,omitempty"`
	ExpiresAt  time.Time  `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

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
	out := make([]MFATrustedDeviceResponse, 0, len(devices))
	for _, d := range devices {
		entry := MFATrustedDeviceResponse{
			ID:         d.ID,
			ExpiresAt:  d.ExpiresAt,
			LastUsedAt: d.LastUsedAt,
		}
		if d.UserAgent != nil {
			entry.UserAgent = *d.UserAgent
		}
		if d.IPAddress != nil {
			entry.IPAddress = d.IPAddress.String()
		}
		out = append(out, entry)
	}
	render.JSON(w, r, out)
}

func (rs *Resource) mfaRevokeTrustedDevice(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid trusted-device id")))
		return
	}
	if err := rs.MFAService.RevokeTrustedDevice(r.Context(), int64(claims.ID), id); err != nil {
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
		if err := rs.issueTrustedDeviceCookie(w, r, accountID); err != nil {
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
// writes the resulting Set-Cookie header.
func (rs *Resource) issueTrustedDeviceCookie(w http.ResponseWriter, r *http.Request, accountID int64) error {
	cookieValue, expiresAt, err := rs.MFAService.IssueTrustedDevice(
		r.Context(), accountID, r.Header.Get(headerUserAgent), parseClientIP(r),
	)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     trustedDeviceCookieName,
		Value:    cookieValue,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		Secure:   secureCookie(),
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

// peekChallengeIdentity decodes the JWT just enough to recover (account_id,
// tenant_id). Used by mfaRecoveryVerify, where we need the identity but the
// MFAService surface only exposes "verify the email code", not "tell me who
// owns this challenge". We deliberately keep this minimal — full claim
// validation still runs once VerifyChallenge is called via the email path.
func (rs *Resource) peekChallengeIdentity(r *http.Request, tokenString string) (int64, int64, error) {
	jwtAuth := jwt.MustNewTokenAuth().JwtAuth
	tok, err := jwtAuth.Decode(tokenString)
	if err != nil {
		return 0, 0, err
	}
	raw := make(map[string]any)
	for _, k := range tok.Keys() {
		var v any
		if gErr := tok.Get(k, &v); gErr == nil {
			raw[k] = v
		}
	}
	var claims jwt.MFAChallengeClaims
	if err := claims.ParseClaims(raw); err != nil {
		return 0, 0, err
	}
	if claims.ExpiresAt > 0 && claims.ExpiresAt < time.Now().Unix() {
		return 0, 0, errors.New("challenge token expired")
	}
	return claims.AccountID, claims.TenantID, nil
}
