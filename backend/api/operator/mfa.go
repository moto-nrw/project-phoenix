package operator

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
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

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
	return common.RequireDependency(w, r, rs.mfaService != nil, errOperatorMFAServiceUnavailable)
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
	case errors.Is(err, authService.ErrMFAPermissionDenied):
		common.RenderError(w, r, ErrForbidden("Permission denied"))
	case errors.Is(err, authService.ErrMFAStatusUnavailable):
		common.RenderError(w, r, ErrServiceUnavailable("MFA ist gerade nicht verfügbar. Bitte versuchen Sie es erneut."))
	default:
		common.RenderError(w, r, ErrInternal("MFA operation failed"))
	}
}

// ----- /auth/mfa/verify -----

// MFAVerifyRequest is the body for POST /operator/auth/mfa/verify. Shared
// with the tenant portal via api/common.
type MFAVerifyRequest = common.MFAVerifyRequest

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

// ----- /auth/mfa/resend -----

// MFAResendRequest is the body for POST /operator/auth/mfa/resend. Shared
// with the tenant portal via api/common.
type MFAResendRequest = common.MFAResendRequest

// MFAResendResponse carries the renewed challenge token — see api/common.
type MFAResendResponse = common.MFAResendResponse

// Resend re-issues an email code against the existing challenge token.
// Rate-limited inside the service. Returns the renewed challenge JWT.
func (rs *MFAResource) Resend(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	req := &MFAResendRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}
	renewed, err := rs.mfaService.ResendChallenge(r.Context(), req.ChallengeToken, parseOperatorClientIP(r))
	if err != nil {
		mapOperatorMFAError(w, r, err)
		return
	}
	render.JSON(w, r, MFAResendResponse{ChallengeToken: renewed})
}

// ----- /auth/mfa/enroll/start -----

// EnrollStart triggers an email with a code that the operator must echo back
// at /auth/mfa/enroll/confirm. No body — the enrollment-scoped JWT
// identifies the operator. Authenticated by MFAEnrollmentAuthenticator,
// which guarantees mfa_enrollment_pending=true and scope=platform.
func (rs *MFAResource) EnrollStart(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	claims, ok := jwt.EnrollmentClaimsFromCtx(r.Context())
	if !ok || claims.AccountID == 0 || claims.Scope != jwt.MFAEnrollmentScopePlatform {
		common.RenderError(w, r, ErrUnauthorized())
		return
	}
	if _, err := rs.mfaService.StartChallenge(r.Context(), claims.AccountID, parseOperatorClientIP(r)); err != nil {
		mapOperatorMFAError(w, r, err)
		return
	}
	common.RespondNoContent(w, r)
}

// ----- /auth/mfa/enroll/confirm -----

// MFAEnrollConfirmRequest is the body for POST
// /operator/auth/mfa/enroll/confirm. Shared with the tenant portal via
// api/common.
type MFAEnrollConfirmRequest = common.MFAEnrollConfirmRequest

// EnrollConfirm verifies the just-emailed code, marks the operator as
// enrolled, and mints a full access/refresh token pair so the frontend can
// seed a real operator session. Replaces the original 204 response that
// re-used the pre-MFA access token.
func (rs *MFAResource) EnrollConfirm(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	claims, ok := jwt.EnrollmentClaimsFromCtx(r.Context())
	if !ok || claims.AccountID == 0 || claims.Scope != jwt.MFAEnrollmentScopePlatform {
		common.RenderError(w, r, ErrUnauthorized())
		return
	}
	req := &MFAEnrollConfirmRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	operatorID := claims.AccountID

	if err := rs.mfaService.VerifyCodeForOperator(r.Context(), operatorID, req.Code); err != nil {
		mapOperatorMFAError(w, r, err)
		return
	}
	if err := rs.mfaService.Enroll(r.Context(), operatorID); err != nil {
		// Already-enrolled is not fatal — a retried request must still mint
		// a real session (no longer the pre-enrollment token).
		if !errors.Is(err, authService.ErrMFAAlreadyEnrolled) {
			mapOperatorMFAError(w, r, err)
			return
		}
	}

	rs.completeMFAExchange(w, r, operatorID, req.RememberDevice)
}

// ----- /auth/mfa/trusted-devices -----

// ListTrustedDevices returns the operator's active trusted devices so
// they can be displayed + revoked from the operator settings page.
// Uses the shared TrustedDeviceDTO + mapper from api/common — same
// wire shape as the tenant-side /auth/mfa/trusted-devices endpoint.
func (rs *MFAResource) ListTrustedDevices(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, ErrUnauthorized())
		return
	}
	devices, err := rs.mfaService.ListTrustedDevices(r.Context(), int64(claims.ID))
	if err != nil {
		mapOperatorMFAError(w, r, err)
		return
	}
	dtos := common.MapTrustedDevices(devices, func(d *platformModels.OperatorMFATrustedDevice) common.TrustedDeviceRow {
		return common.TrustedDeviceRow{
			ID:         d.ID,
			UserAgent:  d.UserAgent,
			IPAddress:  d.IPAddress,
			CreatedAt:  d.CreatedAt,
			ExpiresAt:  d.ExpiresAt,
			LastUsedAt: d.LastUsedAt,
		}
	})
	common.Respond(w, r, http.StatusOK, dtos, "trusted devices")
}

// RevokeTrustedDevice removes a single device the operator no longer
// wants to trust. The service double-checks ownership so an attacker
// with a stolen access token can't revoke another operator's devices.
func (rs *MFAResource) RevokeTrustedDevice(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, ErrUnauthorized())
		return
	}
	idStr := chi.URLParam(r, "deviceId")
	deviceID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || deviceID <= 0 {
		common.RenderError(w, r, ErrInvalidRequest(errors.New("invalid device id")))
		return
	}
	if err := rs.mfaService.RevokeTrustedDevice(r.Context(), int64(claims.ID), deviceID); err != nil {
		mapOperatorMFAError(w, r, err)
		return
	}
	common.RespondNoContent(w, r)
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

// parseOperatorClientIP wraps getClientIP and returns it as a net.IP, or
// nil when nothing parseable is on the request.
func parseOperatorClientIP(r *http.Request) net.IP {
	return getClientIP(r)
}
