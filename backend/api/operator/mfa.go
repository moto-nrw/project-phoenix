package operator

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/spf13/viper"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

// secureCookie returns true only in production so the trusted-device
// cookie is accepted by browsers over HTTP in local dev.
func secureCookie() bool {
	return strings.ToLower(strings.TrimSpace(viper.GetString("app_env"))) == "production"
}

// MFA email-code length is shared with the tenant flow — both call into the
// same crypto helpers in services/auth/mfa_codes.go.
const operatorMFAEmailCodeLength = authService.MFAEmailCodeLength

// errOperatorMFAServiceUnavailable mirrors the tenant-side guard. Operator
// deployments without an MFAService configured should still respond cleanly.
var errOperatorMFAServiceUnavailable = errors.New("operator mfa service is not configured for this deployment")

// MFAResource exposes the operator-side MFA endpoints. Mirror of the tenant
// auth.Resource handlers — split into its own file/struct so the existing
// AuthResource stays focused on credential login + email-change.
type MFAResource struct {
	authService platformSvc.OperatorAuthService
	mfaService  platformSvc.OperatorMFAService
	tokenAuth   *jwt.TokenAuth
}

// NewMFAResource wires the dependencies. mfaService may be nil — handlers
// will return 503 in that case so deployments that haven't enabled MFA yet
// still answer cleanly.
func NewMFAResource(authSvc platformSvc.OperatorAuthService, mfaSvc platformSvc.OperatorMFAService, tokenAuth *jwt.TokenAuth) *MFAResource {
	return &MFAResource{
		authService: authSvc,
		mfaService:  mfaSvc,
		tokenAuth:   tokenAuth,
	}
}

// requireMFA short-circuits with 503 when no MFAService is wired. Use as the
// first line of every handler.
func (rs *MFAResource) requireMFA(w http.ResponseWriter, r *http.Request) bool {
	if rs.mfaService == nil {
		common.RenderError(w, r, &common.ErrResponse{
			Err:            errOperatorMFAServiceUnavailable,
			HTTPStatusCode: http.StatusServiceUnavailable,
			Status:         "error",
			ErrorText:      errOperatorMFAServiceUnavailable.Error(),
		})
		return false
	}
	return true
}

// mapOperatorMFAError translates known MFA-service errors into HTTP responses.
// The error identities are aliased to the tenant ones so the tenant
// errors.Is checks still work — see services/platform/operator_mfa_service.go.
func mapOperatorMFAError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, authService.ErrMFAChallengeTokenInvalid):
		common.RenderError(w, r, ErrUnauthorized())
	case errors.Is(err, authService.ErrMFACodeInvalid):
		common.RenderError(w, r, ErrUnauthorized())
	case errors.Is(err, authService.ErrMFALocked):
		common.RenderError(w, r, ErrTooManyRequests("MFA temporarily locked"))
	case errors.Is(err, authService.ErrMFARateLimited):
		common.RenderError(w, r, ErrTooManyRequests("Too many MFA emails — try again later"))
	case errors.Is(err, authService.ErrMFANotEnrolled):
		common.RenderError(w, r, ErrForbidden("MFA is not enrolled for this operator"))
	case errors.Is(err, authService.ErrMFAAlreadyEnrolled):
		common.RenderError(w, r, ErrConflict("MFA is already enrolled"))
	default:
		common.RenderError(w, r, ErrInternal("MFA operation failed"))
	}
}

// ----- /auth/mfa/verify -----

// MFAVerifyRequest is the body for POST /operator/auth/mfa/verify.
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
		validation.Field(&req.Code, validation.Required, validation.Length(operatorMFAEmailCodeLength, operatorMFAEmailCodeLength)),
	)
}

// Verify exchanges a challenge token + email code for a regular access /
// refresh token pair. When the request carries `remember_device: true` we
// additionally issue a trusted-device cookie.
func (rs *MFAResource) Verify(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	req := &MFAVerifyRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	verified, err := rs.mfaService.VerifyChallenge(r.Context(), req.ChallengeToken, req.Code)
	if err != nil {
		mapOperatorMFAError(w, r, err)
		return
	}
	rs.completeMFAExchange(w, r, verified.OperatorID, req.RememberDevice)
}

// ----- /auth/mfa/recovery/verify -----

// MFARecoveryVerifyRequest is the body for POST /operator/auth/mfa/recovery/verify.
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

// RecoveryVerify is the recovery-code analogue of Verify. The user reaches
// this when the email channel is unavailable. We peek the operator_id out of
// the challenge JWT, then call VerifyRecoveryCode.
func (rs *MFAResource) RecoveryVerify(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	req := &MFARecoveryVerifyRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	operatorID, err := rs.peekOperatorChallengeIdentity(req.ChallengeToken)
	if err != nil {
		common.RenderError(w, r, ErrUnauthorized())
		return
	}

	if err := rs.mfaService.VerifyRecoveryCode(r.Context(), operatorID, req.RecoveryCode); err != nil {
		mapOperatorMFAError(w, r, err)
		return
	}

	rs.completeMFAExchange(w, r, operatorID, req.RememberDevice)
}

// ----- /auth/mfa/resend -----

// MFAResendRequest is the body for POST /operator/auth/mfa/resend.
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

// Resend re-issues an email code against the existing challenge token.
// Rate-limited inside the service.
func (rs *MFAResource) Resend(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	req := &MFAResendRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}
	if err := rs.mfaService.ResendChallenge(r.Context(), req.ChallengeToken, parseOperatorClientIP(r)); err != nil {
		mapOperatorMFAError(w, r, err)
		return
	}
	common.RespondNoContent(w, r)
}

// ----- /auth/mfa/enroll/start -----

// EnrollStart triggers an email with a code that the operator must echo back
// at /auth/mfa/enroll/confirm. No body — the authenticated session identifies
// the operator.
func (rs *MFAResource) EnrollStart(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, ErrUnauthorized())
		return
	}
	if _, err := rs.mfaService.StartChallenge(r.Context(), int64(claims.ID), parseOperatorClientIP(r)); err != nil {
		mapOperatorMFAError(w, r, err)
		return
	}
	common.RespondNoContent(w, r)
}

// ----- /auth/mfa/enroll/confirm -----

// MFAEnrollConfirmRequest is the body for POST /operator/auth/mfa/enroll/confirm.
type MFAEnrollConfirmRequest struct {
	Code string `json:"code"`
}

// Bind validates the confirm request.
func (req *MFAEnrollConfirmRequest) Bind(_ *http.Request) error {
	req.Code = strings.TrimSpace(req.Code)
	return validation.ValidateStruct(req,
		validation.Field(&req.Code, validation.Required, validation.Length(operatorMFAEmailCodeLength, operatorMFAEmailCodeLength)),
	)
}

// MFAEnrollConfirmResponse returns the freshly-generated recovery codes.
// Per the design plan the client must show these once and require the
// operator to confirm storage before continuing.
type MFAEnrollConfirmResponse struct {
	RecoveryCodes []string `json:"recovery_codes"`
}

// EnrollConfirm verifies the just-emailed code, marks the operator as
// enrolled, and seeds the recovery-code pool. Plain codes are returned once
// and only once.
func (rs *MFAResource) EnrollConfirm(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, ErrUnauthorized())
		return
	}
	req := &MFAEnrollConfirmRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	operatorID := int64(claims.ID)

	if err := rs.mfaService.VerifyCodeForOperator(r.Context(), operatorID, req.Code); err != nil {
		mapOperatorMFAError(w, r, err)
		return
	}
	if err := rs.mfaService.Enroll(r.Context(), operatorID); err != nil {
		// Already-enrolled is not fatal — fall through to recovery-code
		// regeneration so re-running enrol gives the operator fresh codes.
		if !errors.Is(err, authService.ErrMFAAlreadyEnrolled) {
			mapOperatorMFAError(w, r, err)
			return
		}
	}
	codes, err := rs.mfaService.GenerateRecoveryCodes(r.Context(), operatorID)
	if err != nil {
		mapOperatorMFAError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, &MFAEnrollConfirmResponse{RecoveryCodes: codes}, "MFA enrolled")
}

// ----- shared helpers -----

// MFATokenResponse is the shape returned to clients after a successful MFA
// challenge → token exchange. Mirrors the tenant TokenResponse.
type MFATokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// completeMFAExchange runs once a challenge has been verified: mints the
// access/refresh token pair, optionally issues a trusted-device cookie, and
// writes the response. Both the email-code and recovery-code paths funnel
// through here so cookie-handling stays in one place.
func (rs *MFAResource) completeMFAExchange(w http.ResponseWriter, r *http.Request, operatorID int64, rememberDevice bool) {
	clientIP := getClientIP(r)
	ipAddress := ""
	if clientIP != nil {
		ipAddress = clientIP.String()
	}
	userAgent := r.Header.Get("User-Agent")

	accessToken, refreshToken, err := rs.authService.IssueTokensForAuthenticatedOperator(
		r.Context(), operatorID, ipAddress, userAgent,
	)
	if err != nil {
		var inactive *platformSvc.OperatorInactiveError
		var notFound *platformSvc.OperatorNotFoundError
		switch {
		case errors.As(err, &inactive):
			common.RenderError(w, r, ErrForbidden("Operator account is inactive"))
		case errors.As(err, &notFound):
			common.RenderError(w, r, ErrUnauthorized())
		default:
			common.RenderError(w, r, ErrInternal("Failed to issue tokens"))
		}
		return
	}

	if rememberDevice {
		if err := rs.issueTrustedDeviceCookie(w, r, operatorID); err != nil {
			// Don't fail the whole login — log and proceed.
			slog.Default().Warn("failed to issue operator trusted-device cookie",
				slog.Int64("operator_id", operatorID),
				slog.String("error", err.Error()),
			)
		}
	}

	common.Respond(w, r, http.StatusOK, &MFATokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, "MFA verification successful")
}

// issueTrustedDeviceCookie hooks into MFAService.IssueTrustedDevice and
// writes the resulting Set-Cookie header.
func (rs *MFAResource) issueTrustedDeviceCookie(w http.ResponseWriter, r *http.Request, operatorID int64) error {
	cookieValue, expiresAt, err := rs.mfaService.IssueTrustedDevice(
		r.Context(), operatorID, r.Header.Get("User-Agent"), parseOperatorClientIP(r),
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

// parseOperatorClientIP wraps getClientIP and returns it as a net.IP, or
// nil when nothing parseable is on the request.
func parseOperatorClientIP(r *http.Request) net.IP {
	return getClientIP(r)
}

// peekOperatorChallengeIdentity decodes the JWT just enough to recover the
// operator_id (carried as `account_id` in MFAChallengeClaims). Used by
// RecoveryVerify, where we need the identity but the OperatorMFAService
// surface only exposes "verify the email code", not "tell me who owns this
// challenge". Full claim validation still runs once VerifyChallenge is called
// via the email path, so this helper is intentionally minimal.
func (rs *MFAResource) peekOperatorChallengeIdentity(tokenString string) (int64, error) {
	if rs.tokenAuth == nil {
		return 0, errors.New("token auth not configured")
	}
	tok, err := rs.tokenAuth.JwtAuth.Decode(tokenString)
	if err != nil {
		return 0, err
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
		return 0, err
	}
	if claims.Scope != jwt.MFAChallengeScopePlatform {
		return 0, errors.New("challenge is not platform-scoped")
	}
	if claims.ExpiresAt > 0 && claims.ExpiresAt < time.Now().Unix() {
		return 0, errors.New("challenge token expired")
	}
	return claims.AccountID, nil
}
